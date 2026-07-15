package s3x

import (
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
	defer f.Close()
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
			Body:          &progressReader{r: f, onProgress: onProgress},
			ContentLength: aws.Int64(size),
			ContentType:   aws.String(ct),
		}
		opts.applyPut(in)
		_, err := cl.PutObject(ctx, in)
		return err
	}
	return multipartUploadParallel(ctx, cl, bucket, key, f, size, ct, partSize, opts, onProgress)
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
	defer out.Body.Close()

	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = copyWithContext(ctx, f, out.Body, onProgress)
	closeErr := f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if closeErr != nil {
		os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, localPath)
}

// StreamCopy copies an object across two (possibly different) connections/clients
// by streaming the source body straight into a destination upload. Used for
// cross-account / cross-endpoint copy where server-side CopyObject cannot reach.
func StreamCopy(ctx context.Context, src, dst *s3.Client, srcBucket, srcKey, dstBucket, dstKey string, opts UploadOptions, onProgress ProgressFunc) error {
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
	defer out.Body.Close()

	in := &s3.PutObjectInput{
		Bucket:        aws.String(dstBucket),
		Key:           aws.String(dstKey),
		Body:          &progressReader{r: out.Body, onProgress: onProgress},
		ContentLength: aws.Int64(size),
	}
	if ct != "" {
		in.ContentType = aws.String(ct)
	}
	opts.applyPut(in)
	_, err = dst.PutObject(ctx, in)
	return err
}

// DownloadBytes reads an object fully into memory (bounded by the caller via a
// prior HeadObject size check). Used for previews.
func DownloadBytes(ctx context.Context, cl *s3.Client, bucket, key string) ([]byte, string, error) {
	out, err := cl.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", err
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, "", err
	}
	return data, aws.ToString(out.ContentType), nil
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
