package s3x

import (
	"context"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3scalpel/internal/model"
)

// ListMultipartUploads returns every multipart upload a bucket has started but
// never completed. Each one holds storage that most providers bill for until it
// is aborted, and nothing removes them automatically unless a lifecycle rule
// says so — an interrupted client leaves them behind indefinitely.
func ListMultipartUploads(ctx context.Context, cl *s3.Client, bucket, prefix string) ([]model.MultipartUpload, error) {
	in := &s3.ListMultipartUploadsInput{Bucket: aws.String(bucket)}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}

	var out []model.MultipartUpload
	for {
		page, err := cl.ListMultipartUploads(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, u := range page.Uploads {
			item := model.MultipartUpload{
				Key:          aws.ToString(u.Key),
				UploadID:     aws.ToString(u.UploadId),
				Initiated:    ms(u.Initiated),
				StorageClass: string(u.StorageClass),
			}
			// Part sizes are what actually cost money, so total them up.
			if parts, err := listParts(ctx, cl, bucket, item.Key, item.UploadID); err == nil {
				item.PartCount = len(parts)
				for _, p := range parts {
					item.Size += aws.ToInt64(p.Size)
				}
			}
			out = append(out, item)
		}
		if !aws.ToBool(page.IsTruncated) {
			break
		}
		in.KeyMarker = page.NextKeyMarker
		in.UploadIdMarker = page.NextUploadIdMarker
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Initiated < out[j].Initiated })
	return out, nil
}

// AbortUpload discards an in-flight multipart upload and the parts it holds.
func AbortUpload(ctx context.Context, cl *s3.Client, bucket, key, uploadID string) error {
	_, err := cl.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil && IsNotFound(err) {
		return nil // already gone
	}
	return err
}

// AbortUploads discards several uploads, reporting how many succeeded and the
// first failure. A single denied key should not stop the rest of the sweep.
func AbortUploads(ctx context.Context, cl *s3.Client, bucket string, uploads []model.MultipartUpload) (int, error) {
	var firstErr error
	done := 0
	for _, u := range uploads {
		if err := AbortUpload(ctx, cl, bucket, u.Key, u.UploadID); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		done++
	}
	return done, firstErr
}

// ObjectExists reports whether a key is present. A HeadObject 404 is the answer,
// not an error; anything else is surfaced so callers do not mistake a denied
// probe for a free slot.
func ObjectExists(ctx context.Context, cl *s3.Client, bucket, key string) (bool, error) {
	_, err := cl.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err == nil {
		return true, nil
	}
	if IsNotFound(err) {
		return false, nil
	}
	return false, err
}

// FreeKey returns key itself when it is unused, otherwise the first free
// "name (n).ext" variant. It gives up after `limit` attempts so a pathological
// directory cannot spin forever.
func FreeKey(ctx context.Context, cl *s3.Client, bucket, key string, limit int) (string, error) {
	exists, err := ObjectExists(ctx, cl, bucket, key)
	if err != nil {
		return "", err
	}
	if !exists {
		return key, nil
	}
	if limit <= 0 {
		limit = 100
	}
	for n := 1; n <= limit; n++ {
		candidate := numberedName(key, n)
		exists, err := ObjectExists(ctx, cl, bucket, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", &appError{"no free name found for " + key}
}

// numberedName inserts " (n)" before the extension: "a/b.txt" -> "a/b (2).txt".
func numberedName(key string, n int) string {
	dir, base := "", key
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		dir, base = key[:idx+1], key[idx+1:]
	}
	stem, ext := base, ""
	// A leading dot is part of the name (".gitignore"), not an extension.
	if idx := strings.LastIndex(base, "."); idx > 0 {
		stem, ext = base[:idx], base[idx:]
	}
	return dir + stem + " (" + itoa(n) + ")" + ext
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

type appError struct{ msg string }

func (e *appError) Error() string { return e.msg }
