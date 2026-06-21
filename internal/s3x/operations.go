package s3x

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"s3scalpel/internal/model"
)

func ms(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixMilli()
}

// ListBuckets returns all buckets visible to the connection.
func ListBuckets(ctx context.Context, cl *s3.Client) ([]model.BucketInfo, error) {
	out, err := cl.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	buckets := make([]model.BucketInfo, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		buckets = append(buckets, model.BucketInfo{
			Name:      aws.ToString(b.Name),
			CreatedAt: ms(b.CreationDate),
		})
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Name < buckets[j].Name })
	return buckets, nil
}

// CreateBucket creates a bucket, sending a LocationConstraint only when needed.
func CreateBucket(ctx context.Context, cl *s3.Client, name, region string) error {
	in := &s3.CreateBucketInput{Bucket: aws.String(name)}
	if region != "" && region != "us-east-1" && region != "auto" {
		in.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}
	_, err := cl.CreateBucket(ctx, in)
	return err
}

// DeleteBucket removes an (empty) bucket.
func DeleteBucket(ctx context.Context, cl *s3.Client, name string) error {
	_, err := cl.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(name)})
	return err
}

// ListObjects returns one folder-style page (CommonPrefixes become folders).
func ListObjects(ctx context.Context, cl *s3.Client, bucket, prefix, token string, maxKeys int32) (model.ListResult, error) {
	in := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}
	if token != "" {
		in.ContinuationToken = aws.String(token)
	}
	if maxKeys > 0 {
		in.MaxKeys = aws.Int32(maxKeys)
	}
	out, err := cl.ListObjectsV2(ctx, in)
	if err != nil {
		return model.ListResult{}, err
	}
	res := model.ListResult{
		Prefix:      prefix,
		IsTruncated: aws.ToBool(out.IsTruncated),
		NextToken:   aws.ToString(out.NextContinuationToken),
	}
	for _, cp := range out.CommonPrefixes {
		key := aws.ToString(cp.Prefix)
		res.Entries = append(res.Entries, model.ObjectEntry{
			Key:      key,
			Name:     folderName(key, prefix),
			IsFolder: true,
		})
	}
	for _, obj := range out.Contents {
		key := aws.ToString(obj.Key)
		if key == prefix { // the folder placeholder object itself
			continue
		}
		res.Entries = append(res.Entries, model.ObjectEntry{
			Key:          key,
			Name:         strings.TrimPrefix(key, prefix),
			Size:         aws.ToInt64(obj.Size),
			LastModified: ms(obj.LastModified),
			ETag:         strings.Trim(aws.ToString(obj.ETag), "\""),
			StorageClass: string(obj.StorageClass),
		})
	}
	return res, nil
}

func folderName(key, prefix string) string {
	name := strings.TrimPrefix(key, prefix)
	return strings.TrimSuffix(name, "/")
}

// ListAllObjects recursively lists every object under a prefix (no delimiter),
// used for recursive folder download and prefix deletion.
func ListAllObjects(ctx context.Context, cl *s3.Client, bucket, prefix string) ([]model.ObjectEntry, error) {
	p := s3.NewListObjectsV2Paginator(cl, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	var entries []model.ObjectEntry
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			entries = append(entries, model.ObjectEntry{
				Key:          key,
				Name:         key,
				Size:         aws.ToInt64(obj.Size),
				LastModified: ms(obj.LastModified),
				ETag:         strings.Trim(aws.ToString(obj.ETag), "\""),
				StorageClass: string(obj.StorageClass),
				IsFolder:     strings.HasSuffix(key, "/"),
			})
		}
	}
	return entries, nil
}

// HeadObject fetches full object metadata.
func HeadObject(ctx context.Context, cl *s3.Client, bucket, key, versionID string) (model.ObjectProperties, error) {
	in := &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	out, err := cl.HeadObject(ctx, in)
	if err != nil {
		return model.ObjectProperties{}, err
	}
	return model.ObjectProperties{
		Key:                key,
		Size:               aws.ToInt64(out.ContentLength),
		ContentType:        aws.ToString(out.ContentType),
		ETag:               strings.Trim(aws.ToString(out.ETag), "\""),
		LastModified:       ms(out.LastModified),
		StorageClass:       string(out.StorageClass),
		VersionID:          aws.ToString(out.VersionId),
		CacheControl:       aws.ToString(out.CacheControl),
		ContentEncoding:    aws.ToString(out.ContentEncoding),
		ContentDisposition: aws.ToString(out.ContentDisposition),
		Metadata:           out.Metadata,
	}, nil
}

// PresignGet returns a presigned GET URL valid for the given duration.
func PresignGet(ctx context.Context, cl *s3.Client, bucket, key, versionID string, expiry time.Duration) (string, error) {
	ps := s3.NewPresignClient(cl)
	in := &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	req, err := ps.PresignGetObject(ctx, in, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// GetTags returns the tag set on an object.
func GetTags(ctx context.Context, cl *s3.Client, bucket, key string) ([]model.Tag, error) {
	out, err := cl.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: aws.String(bucket), Key: aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	tags := make([]model.Tag, 0, len(out.TagSet))
	for _, t := range out.TagSet {
		tags = append(tags, model.Tag{Key: aws.ToString(t.Key), Value: aws.ToString(t.Value)})
	}
	return tags, nil
}

// PutTags replaces the entire tag set on an object.
func PutTags(ctx context.Context, cl *s3.Client, bucket, key string, tags []model.Tag) error {
	set := make([]types.Tag, 0, len(tags))
	for _, t := range tags {
		set = append(set, types.Tag{Key: aws.String(t.Key), Value: aws.String(t.Value)})
	}
	if len(set) == 0 {
		_, err := cl.DeleteObjectTagging(ctx, &s3.DeleteObjectTaggingInput{
			Bucket: aws.String(bucket), Key: aws.String(key),
		})
		return err
	}
	_, err := cl.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		Tagging: &types.Tagging{TagSet: set},
	})
	return err
}

// VersioningEnabled reports whether a bucket has versioning turned on.
func VersioningEnabled(ctx context.Context, cl *s3.Client, bucket string) (bool, error) {
	out, err := cl.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	if err != nil {
		return false, err
	}
	return out.Status == types.BucketVersioningStatusEnabled, nil
}

// ListVersions returns all versions and delete markers for a specific key.
func ListVersions(ctx context.Context, cl *s3.Client, bucket, key string) ([]model.ObjectVersion, error) {
	out, err := cl.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	var versions []model.ObjectVersion
	for _, v := range out.Versions {
		if aws.ToString(v.Key) != key {
			continue
		}
		versions = append(versions, model.ObjectVersion{
			Key:          aws.ToString(v.Key),
			VersionID:    aws.ToString(v.VersionId),
			IsLatest:     aws.ToBool(v.IsLatest),
			Size:         aws.ToInt64(v.Size),
			LastModified: ms(v.LastModified),
			ETag:         strings.Trim(aws.ToString(v.ETag), "\""),
		})
	}
	for _, d := range out.DeleteMarkers {
		if aws.ToString(d.Key) != key {
			continue
		}
		versions = append(versions, model.ObjectVersion{
			Key:            aws.ToString(d.Key),
			VersionID:      aws.ToString(d.VersionId),
			IsLatest:       aws.ToBool(d.IsLatest),
			LastModified:   ms(d.LastModified),
			IsDeleteMarker: true,
		})
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].LastModified > versions[j].LastModified })
	return versions, nil
}

// CreateFolder writes a zero-byte placeholder object whose key ends in "/".
func CreateFolder(ctx context.Context, cl *s3.Client, bucket, key string) error {
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	_, err := cl.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		ContentLength: aws.Int64(0),
	})
	return err
}

// DeleteObject removes a single object (optionally a specific version).
func DeleteObject(ctx context.Context, cl *s3.Client, bucket, key, versionID string) error {
	in := &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if versionID != "" {
		in.VersionId = aws.String(versionID)
	}
	_, err := cl.DeleteObject(ctx, in)
	return err
}

// DeletePrefix removes every object under a prefix (used to delete a folder).
//
// It deletes objects individually with bounded concurrency rather than via the
// batch DeleteObjects API: that API requires a Content-MD5 header which several
// S3-compatible gateways enforce but the SDK omits once default checksums are
// disabled (needed for broad upload compatibility). Single DeleteObject calls
// are portable everywhere.
func DeletePrefix(ctx context.Context, cl *s3.Client, bucket, prefix string) error {
	entries, err := ListAllObjects(ctx, cl, bucket, prefix)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		// Empty folder: remove just the placeholder object.
		return DeleteObject(ctx, cl, bucket, prefix, "")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, e := range entries {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(key string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := DeleteObject(ctx, cl, bucket, key, ""); err != nil && ctx.Err() == nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				mu.Unlock()
			}
		}(e.Key)
	}
	wg.Wait()
	return firstErr
}

// CopyObject copies within or across buckets. CopySource must be URL-escaped.
func CopyObject(ctx context.Context, cl *s3.Client, srcBucket, srcKey, dstBucket, dstKey string) error {
	source := srcBucket + "/" + url.PathEscape(srcKey)
	_, err := cl.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(source),
	})
	return err
}

// IsNotFound reports whether an error is an S3 404.
func IsNotFound(err error) bool {
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode() == 404
	}
	return false
}
