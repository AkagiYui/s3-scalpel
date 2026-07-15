package s3x

import (
	"context"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"s3scalpel/internal/model"
)

// UploadOptions carries optional per-upload settings (storage class, server-side
// encryption, canned ACL) applied to PutObject / CreateMultipartUpload.
type UploadOptions struct {
	StorageClass string
	SSEAlgorithm string // "", "AES256", "aws:kms"
	KMSKeyID     string
	ACL          string // canned ACL, e.g. "private", "public-read"
}

func (o UploadOptions) applyPut(in *s3.PutObjectInput) {
	if o.StorageClass != "" {
		in.StorageClass = types.StorageClass(o.StorageClass)
	}
	if o.SSEAlgorithm != "" {
		in.ServerSideEncryption = types.ServerSideEncryption(o.SSEAlgorithm)
		if types.ServerSideEncryption(o.SSEAlgorithm) == types.ServerSideEncryptionAwsKms && o.KMSKeyID != "" {
			in.SSEKMSKeyId = aws.String(o.KMSKeyID)
		}
	}
	if o.ACL != "" {
		in.ACL = types.ObjectCannedACL(o.ACL)
	}
}

func (o UploadOptions) applyCreateMultipart(in *s3.CreateMultipartUploadInput) {
	if o.StorageClass != "" {
		in.StorageClass = types.StorageClass(o.StorageClass)
	}
	if o.SSEAlgorithm != "" {
		in.ServerSideEncryption = types.ServerSideEncryption(o.SSEAlgorithm)
		if types.ServerSideEncryption(o.SSEAlgorithm) == types.ServerSideEncryptionAwsKms && o.KMSKeyID != "" {
			in.SSEKMSKeyId = aws.String(o.KMSKeyID)
		}
	}
	if o.ACL != "" {
		in.ACL = types.ObjectCannedACL(o.ACL)
	}
}

// RestoreObject initiates a restore of an archived (Glacier/Deep Archive) object
// for the given number of days at the given retrieval tier.
func RestoreObject(ctx context.Context, cl *s3.Client, bucket, key string, days int32, tier string) error {
	req := &types.RestoreRequest{Days: aws.Int32(days)}
	if tier != "" {
		req.GlacierJobParameters = &types.GlacierJobParameters{Tier: types.Tier(tier)}
	}
	_, err := cl.RestoreObject(ctx, &s3.RestoreObjectInput{
		Bucket:         aws.String(bucket),
		Key:            aws.String(key),
		RestoreRequest: req,
	})
	return err
}

// SetObjectACL applies a canned ACL to an object (e.g. "private", "public-read").
func SetObjectACL(ctx context.Context, cl *s3.Client, bucket, key, cannedACL string) error {
	_, err := cl.PutObjectAcl(ctx, &s3.PutObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ACL:    types.ObjectCannedACL(cannedACL),
	})
	return err
}

const allUsersURI = "http://acs.amazonaws.com/groups/global/AllUsers"

// GetObjectACL returns a simplified view of an object's ACL: the owner, whether
// it is publicly readable, and the individual grants.
func GetObjectACL(ctx context.Context, cl *s3.Client, bucket, key string) (model.ObjectACL, error) {
	out, err := cl.GetObjectAcl(ctx, &s3.GetObjectAclInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return model.ObjectACL{}, err
	}
	acl := model.ObjectACL{}
	if out.Owner != nil {
		acl.Owner = aws.ToString(out.Owner.DisplayName)
		if acl.Owner == "" {
			acl.Owner = aws.ToString(out.Owner.ID)
		}
	}
	for _, g := range out.Grants {
		grant := model.ACLGrant{Permission: string(g.Permission)}
		if g.Grantee != nil {
			if g.Grantee.URI != nil {
				grant.URI = aws.ToString(g.Grantee.URI)
				if grant.URI == allUsersURI {
					grant.Grantee = "AllUsers"
					if g.Permission == types.PermissionRead || g.Permission == types.PermissionFullControl {
						acl.IsPublic = true
					}
				}
			}
			if grant.Grantee == "" {
				grant.Grantee = aws.ToString(g.Grantee.DisplayName)
			}
			if grant.Grantee == "" {
				grant.Grantee = aws.ToString(g.Grantee.ID)
			}
		}
		acl.Grants = append(acl.Grants, grant)
	}
	return acl, nil
}

// UpdateObjectMeta rewrites an object's system/user metadata and storage class
// in place via a self-copy with REPLACE directive.
func UpdateObjectMeta(ctx context.Context, cl *s3.Client, bucket, key string, m model.ObjectMetaUpdate) error {
	in := &s3.CopyObjectInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(key),
		CopySource:        aws.String(bucket + "/" + url.PathEscape(key)),
		MetadataDirective: types.MetadataDirectiveReplace,
		Metadata:          m.Metadata,
	}
	if m.ContentType != "" {
		in.ContentType = aws.String(m.ContentType)
	}
	if m.CacheControl != "" {
		in.CacheControl = aws.String(m.CacheControl)
	}
	if m.ContentDisposition != "" {
		in.ContentDisposition = aws.String(m.ContentDisposition)
	}
	if m.ContentEncoding != "" {
		in.ContentEncoding = aws.String(m.ContentEncoding)
	}
	if m.StorageClass != "" {
		in.StorageClass = types.StorageClass(m.StorageClass)
	}
	_, err := cl.CopyObject(ctx, in)
	return err
}

// PresignPut returns a presigned PUT URL for uploading to a key.
func PresignPut(ctx context.Context, cl *s3.Client, bucket, key, contentType string, expiry time.Duration) (string, error) {
	ps := s3.NewPresignClient(cl)
	in := &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	req, err := ps.PresignPutObject(ctx, in, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}
