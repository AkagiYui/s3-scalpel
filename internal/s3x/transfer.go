package s3x

import (
	"bytes"
	"context"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ProgressFunc reports cumulative bytes transferred.
//
// Parallel transfers (multipart upload, ranged download) call it concurrently
// from several goroutines, so implementations must be safe for concurrent use.
// The reported value is monotonically non-decreasing within a single pass, but a
// body that the SDK rewinds to hash or retry restarts the count.
type ProgressFunc func(transferred int64)

// MinPartSize is the S3 multipart minimum (5 MiB) for all but the final part.
const MinPartSize int64 = 5 * 1024 * 1024

// contentTypeForName guesses a content type from a file extension.
func contentTypeForName(name string) string {
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}

// copyWithContext is an io.Copy that aborts promptly on context cancellation and
// reports progress as it goes.
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader, onProgress ProgressFunc) (int64, error) {
	buf := make([]byte, 256*1024)
	var total int64
	for {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
			if onProgress != nil {
				onProgress(total)
			}
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

// Upload transfers a local file to S3, using manual multipart when enabled and
// the file exceeds partSize. Progress is reported cumulatively.
func Upload(ctx context.Context, cl *s3.Client, bucket, key, localPath string, multipart bool, partSize int64, opts UploadOptions, onProgress ProgressFunc) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()
	ct := contentTypeForName(localPath)

	if partSize < MinPartSize {
		partSize = MinPartSize
	}
	if !multipart || size <= partSize {
		in := &s3.PutObjectInput{
			Bucket:        aws.String(bucket),
			Key:           aws.String(key),
			Body:          newProgressBody(f, onProgress),
			ContentLength: aws.Int64(size),
			ContentType:   aws.String(ct),
		}
		opts.applyPut(in)
		_, err := cl.PutObject(ctx, in)
		return err
	}
	return multipartUploadParallel(ctx, cl, bucket, key, f, size, ct, partSize, opts, onProgress)
}

// newProgressBody wraps a request body so it reports cumulative bytes read.
//
// Seekability matters: over an https endpoint the SDK signs S3 payloads as
// UNSIGNED-PAYLOAD and streams the body once, but over a plain http endpoint —
// which self-hosted MinIO installs commonly are — it must hash the body first,
// which means rewinding it. It also rewinds to retry a failed request. Wrapping
// a seekable source in a plain io.Reader therefore breaks uploads outright on
// http endpoints, so the wrapper forwards Seek whenever the source supports it.
func newProgressBody(r io.Reader, onProgress ProgressFunc) io.Reader {
	if rs, ok := r.(io.ReadSeeker); ok {
		return &progressReadSeeker{progressReader{r: rs, onProgress: onProgress}}
	}
	return &progressReader{r: r, onProgress: onProgress}
}

// progressReader wraps a reader to report cumulative bytes read.
type progressReader struct {
	r          io.Reader
	total      int64
	onProgress ProgressFunc
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.total += int64(n)
		if p.onProgress != nil {
			p.onProgress(p.total)
		}
	}
	return n, err
}

// progressReadSeeker exposes the source's Seek so the SDK can hash and retry the
// body. Rewinding resets the counter, so the progress reported during the pass
// that actually transmits is accurate.
type progressReadSeeker struct{ progressReader }

func (p *progressReadSeeker) Seek(offset int64, whence int) (int64, error) {
	pos, err := p.r.(io.Seeker).Seek(offset, whence)
	if err == nil {
		p.total = pos
	}
	return pos, err
}

// DownloadOptions tunes how an object is fetched. When Multipart is set and the
// object is large (>= MinParallelDownloadSize) the object is fetched with
// concurrent ranged GETs; otherwise a single streaming GET is used.
type DownloadOptions struct {
	PartSize        int64
	PartConcurrency int
	Multipart       bool
}

// Download fetches an object to a local path, creating parent directories. It
// writes to a ".part" temp file and renames on success.
func Download(ctx context.Context, cl *s3.Client, bucket, key, localPath string, opts DownloadOptions, onProgress ProgressFunc) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	tmp := localPath + ".part"

	if opts.Multipart {
		if size, err := ObjectSize(ctx, cl, bucket, key); err == nil && size >= MinParallelDownloadSize {
			if derr := downloadParallel(ctx, cl, bucket, key, tmp, size, opts.PartSize, opts.PartConcurrency, onProgress); derr != nil {
				return derr
			}
			return os.Rename(tmp, localPath)
		}
	}

	out, err := cl.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer func() { _ = out.Body.Close() }()

	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = copyWithContext(ctx, f, out.Body, onProgress)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, localPath)
}

// StreamCopy copies an object across two (possibly different) connections or
// endpoints by pumping the source body into a destination upload. It is used for
// cross-account / cross-endpoint copies, where a server-side CopyObject cannot
// reach the destination.
//
// The source body is an HTTP stream and therefore not seekable, which the SDK
// requires to sign and retry a request. Bytes are staged through a buffer of at
// most partSize instead: a small object becomes one PutObject, a large one a
// sequential multipart upload. Memory stays bounded by partSize either way and
// no temporary file is needed.
func StreamCopy(ctx context.Context, src, dst *s3.Client, srcBucket, srcKey, dstBucket, dstKey string, partSize int64, opts UploadOptions, onProgress ProgressFunc) error {
	if partSize < MinPartSize {
		partSize = MinPartSize
	}
	head, err := src.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(srcBucket), Key: aws.String(srcKey)})
	if err != nil {
		return err
	}
	size := aws.ToInt64(head.ContentLength)
	ct := aws.ToString(head.ContentType)

	out, err := src.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(srcBucket), Key: aws.String(srcKey)})
	if err != nil {
		return err
	}
	defer func() { _ = out.Body.Close() }()

	if size <= partSize {
		body, err := io.ReadAll(io.LimitReader(out.Body, partSize+1))
		if err != nil {
			return err
		}
		in := &s3.PutObjectInput{
			Bucket:        aws.String(dstBucket),
			Key:           aws.String(dstKey),
			Body:          bytes.NewReader(body),
			ContentLength: aws.Int64(int64(len(body))),
		}
		if ct != "" {
			in.ContentType = aws.String(ct)
		}
		opts.applyPut(in)
		if _, err := dst.PutObject(ctx, in); err != nil {
			return err
		}
		if onProgress != nil {
			onProgress(int64(len(body)))
		}
		return nil
	}
	return multipartUploadStream(ctx, dst, dstBucket, dstKey, out.Body, ct, partSize, opts, onProgress)
}

// ObjectSize returns the content length of an object via HeadObject.
func ObjectSize(ctx context.Context, cl *s3.Client, bucket, key string) (int64, error) {
	out, err := cl.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return 0, err
	}
	return aws.ToInt64(out.ContentLength), nil
}

// JoinKey joins a prefix and a name into an object key.
func JoinKey(prefix, name string) string {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}
