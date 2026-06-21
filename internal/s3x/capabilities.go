package s3x

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"s3scalpel/internal/model"
)

// isAccessDenied reports whether an error is an authorization denial (403).
func isAccessDenied(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "AccessDenied", "Forbidden", "AllAccessDisabled", "UnauthorizedAccess":
			return true
		}
	}
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode() == 403
	}
	return false
}

// probe classifies a probe result. A "not found" outcome means the permission is
// granted but the probed resource simply doesn't exist, so it counts as allowed.
func probe(op string, err error) model.Capability {
	c := model.Capability{Op: op, Tested: true}
	switch {
	case err == nil:
		c.Allowed = true
	case IsNotFound(err):
		c.Allowed = true
		c.Detail = "permission ok (resource absent)"
	case isAccessDenied(err):
		c.Allowed = false
		c.Detail = "access denied"
	default:
		c.Allowed = false
		c.Detail = shortErr(err)
	}
	return c
}

func shortErr(err error) string {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode()
	}
	s := err.Error()
	if len(s) > 80 {
		return s[:80]
	}
	return s
}

// CheckCapabilities probes the credentials against a set of operations and
// reports which are permitted. Account-level operations (ListBuckets) always
// run; bucket/object operations run only when a bucket is supplied. The write
// probes create a tiny temporary object and delete it.
func CheckCapabilities(ctx context.Context, cl *s3.Client, bucket string) []model.Capability {
	caps := make([]model.Capability, 0, 8)

	_, err := cl.ListBuckets(ctx, &s3.ListBucketsInput{})
	caps = append(caps, probe("listBuckets", err))

	if bucket == "" {
		for _, op := range []string{"listObjects", "bucketVersioning", "putObject", "getObject", "objectTagging", "deleteObject"} {
			caps = append(caps, model.Capability{Op: op, Tested: false})
		}
		return caps
	}

	_, err = cl.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), MaxKeys: aws.Int32(1)})
	caps = append(caps, probe("listObjects", err))

	_, err = cl.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	caps = append(caps, probe("bucketVersioning", err))

	probeKey := fmt.Sprintf(".s3scalpel-capability-probe-%d.txt", time.Now().UnixNano())

	_, err = cl.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(probeKey),
		Body:          bytes.NewReader([]byte("s3scalpel probe")),
		ContentLength: aws.Int64(15),
	})
	caps = append(caps, probe("putObject", err))

	_, err = cl.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(probeKey)})
	caps = append(caps, probe("getObject", err))

	_, err = cl.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(probeKey),
		Tagging: &types.Tagging{TagSet: []types.Tag{{Key: aws.String("s3scalpel"), Value: aws.String("probe")}}},
	})
	caps = append(caps, probe("objectTagging", err))

	// Always attempt the delete: it both probes the permission and cleans up.
	_, err = cl.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(probeKey)})
	caps = append(caps, probe("deleteObject", err))

	return caps
}
