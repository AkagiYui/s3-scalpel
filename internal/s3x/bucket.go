package s3x

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"s3scalpel/internal/model"
)

// isNoSuchConfig reports whether an error is one of S3's "this optional
// configuration was never set" responses. Those are treated as an empty
// (unconfigured) state rather than a hard error.
func isNoSuchConfig(err error) bool {
	if err == nil {
		return false
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchBucketPolicy",
			"NoSuchCORSConfiguration",
			"NoSuchLifecycleConfiguration",
			"NoSuchConfiguration",
			"ServerSideEncryptionConfigurationNotFoundError",
			"NoSuchPublicAccessBlockConfiguration",
			"NoSuchTagSet",
			"NoSuchTagSetError":
			return true
		}
	}
	// Some gateways report absent config as 404.
	if IsNotFound(err) {
		return true
	}
	return false
}

// GetBucketLocation returns a bucket's region (empty means us-east-1).
func GetBucketLocation(ctx context.Context, cl *s3.Client, bucket string) (string, error) {
	out, err := cl.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: aws.String(bucket)})
	if err != nil {
		return "", err
	}
	loc := string(out.LocationConstraint)
	if loc == "" {
		loc = "us-east-1"
	}
	return loc, nil
}

// Versioning ------------------------------------------------------------------

// GetVersioning returns a bucket's versioning configuration.
func GetVersioning(ctx context.Context, cl *s3.Client, bucket string) (model.BucketVersioning, error) {
	out, err := cl.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	if err != nil {
		return model.BucketVersioning{}, err
	}
	return model.BucketVersioning{
		Status:    string(out.Status),
		MFADelete: out.MFADelete == types.MFADeleteStatusEnabled,
	}, nil
}

// SetVersioning enables or suspends versioning on a bucket.
func SetVersioning(ctx context.Context, cl *s3.Client, bucket string, enabled bool) error {
	status := types.BucketVersioningStatusSuspended
	if enabled {
		status = types.BucketVersioningStatusEnabled
	}
	_, err := cl.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket:                  aws.String(bucket),
		VersioningConfiguration: &types.VersioningConfiguration{Status: status},
	})
	return err
}

// Policy ----------------------------------------------------------------------

// GetPolicy returns the raw JSON bucket policy (empty string if none set).
func GetPolicy(ctx context.Context, cl *s3.Client, bucket string) (string, error) {
	out, err := cl.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	if err != nil {
		if isNoSuchConfig(err) {
			return "", nil
		}
		return "", err
	}
	return aws.ToString(out.Policy), nil
}

// PutPolicy sets (non-empty) or deletes (empty) the bucket policy.
func PutPolicy(ctx context.Context, cl *s3.Client, bucket, policy string) error {
	if strings.TrimSpace(policy) == "" {
		_, err := cl.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{Bucket: aws.String(bucket)})
		if isNoSuchConfig(err) {
			return nil
		}
		return err
	}
	_, err := cl.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucket),
		Policy: aws.String(policy),
	})
	return err
}

// CORS ------------------------------------------------------------------------

// GetCORS returns the bucket's CORS rules (nil if none set).
func GetCORS(ctx context.Context, cl *s3.Client, bucket string) ([]model.CORSRule, error) {
	out, err := cl.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: aws.String(bucket)})
	if err != nil {
		if isNoSuchConfig(err) {
			return nil, nil
		}
		return nil, err
	}
	rules := make([]model.CORSRule, 0, len(out.CORSRules))
	for _, r := range out.CORSRules {
		rules = append(rules, model.CORSRule{
			ID:             aws.ToString(r.ID),
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders,
			ExposeHeaders:  r.ExposeHeaders,
			MaxAgeSeconds:  aws.ToInt32(r.MaxAgeSeconds),
		})
	}
	return rules, nil
}

// PutCORS replaces the bucket's CORS rules (empty list deletes the config).
func PutCORS(ctx context.Context, cl *s3.Client, bucket string, rules []model.CORSRule) error {
	if len(rules) == 0 {
		_, err := cl.DeleteBucketCors(ctx, &s3.DeleteBucketCorsInput{Bucket: aws.String(bucket)})
		if isNoSuchConfig(err) {
			return nil
		}
		return err
	}
	set := make([]types.CORSRule, 0, len(rules))
	for _, r := range rules {
		cr := types.CORSRule{
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders,
			ExposeHeaders:  r.ExposeHeaders,
		}
		if r.ID != "" {
			cr.ID = aws.String(r.ID)
		}
		if r.MaxAgeSeconds > 0 {
			cr.MaxAgeSeconds = aws.Int32(r.MaxAgeSeconds)
		}
		set = append(set, cr)
	}
	_, err := cl.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket:            aws.String(bucket),
		CORSConfiguration: &types.CORSConfiguration{CORSRules: set},
	})
	return err
}

// Lifecycle -------------------------------------------------------------------

// GetLifecycle returns the bucket's lifecycle rules (nil if none set).
func GetLifecycle(ctx context.Context, cl *s3.Client, bucket string) ([]model.LifecycleRule, error) {
	out, err := cl.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucket)})
	if err != nil {
		if isNoSuchConfig(err) {
			return nil, nil
		}
		return nil, err
	}
	rules := make([]model.LifecycleRule, 0, len(out.Rules))
	for _, r := range out.Rules {
		lr := model.LifecycleRule{
			ID:      aws.ToString(r.ID),
			Enabled: r.Status == types.ExpirationStatusEnabled,
		}
		if r.Filter != nil && r.Filter.Prefix != nil {
			lr.Prefix = aws.ToString(r.Filter.Prefix)
		}
		if r.Prefix != nil {
			lr.Prefix = aws.ToString(r.Prefix)
		}
		if r.Expiration != nil {
			lr.ExpirationDays = aws.ToInt32(r.Expiration.Days)
		}
		if r.AbortIncompleteMultipartUpload != nil {
			lr.AbortIncompleteMultipartDays = aws.ToInt32(r.AbortIncompleteMultipartUpload.DaysAfterInitiation)
		}
		for _, nv := range []*types.NoncurrentVersionExpiration{r.NoncurrentVersionExpiration} {
			if nv != nil {
				lr.NoncurrentVersionExpirationDays = aws.ToInt32(nv.NoncurrentDays)
			}
		}
		if len(r.Transitions) > 0 {
			lr.TransitionDays = aws.ToInt32(r.Transitions[0].Days)
			lr.TransitionStorageClass = string(r.Transitions[0].StorageClass)
		}
		rules = append(rules, lr)
	}
	return rules, nil
}

// PutLifecycle replaces the bucket's lifecycle rules (empty list deletes it).
func PutLifecycle(ctx context.Context, cl *s3.Client, bucket string, rules []model.LifecycleRule) error {
	if len(rules) == 0 {
		_, err := cl.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{Bucket: aws.String(bucket)})
		if isNoSuchConfig(err) {
			return nil
		}
		return err
	}
	set := make([]types.LifecycleRule, 0, len(rules))
	for _, r := range rules {
		status := types.ExpirationStatusDisabled
		if r.Enabled {
			status = types.ExpirationStatusEnabled
		}
		lr := types.LifecycleRule{
			ID:     aws.String(r.ID),
			Status: status,
			Filter: &types.LifecycleRuleFilter{Prefix: aws.String(r.Prefix)},
		}
		if r.ExpirationDays > 0 {
			lr.Expiration = &types.LifecycleExpiration{Days: aws.Int32(r.ExpirationDays)}
		}
		if r.NoncurrentVersionExpirationDays > 0 {
			lr.NoncurrentVersionExpiration = &types.NoncurrentVersionExpiration{NoncurrentDays: aws.Int32(r.NoncurrentVersionExpirationDays)}
		}
		if r.AbortIncompleteMultipartDays > 0 {
			lr.AbortIncompleteMultipartUpload = &types.AbortIncompleteMultipartUpload{DaysAfterInitiation: aws.Int32(r.AbortIncompleteMultipartDays)}
		}
		if r.TransitionDays > 0 && r.TransitionStorageClass != "" {
			lr.Transitions = []types.Transition{{
				Days:         aws.Int32(r.TransitionDays),
				StorageClass: types.TransitionStorageClass(r.TransitionStorageClass),
			}}
		}
		set = append(set, lr)
	}
	_, err := cl.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{Rules: set},
	})
	return err
}

// Encryption ------------------------------------------------------------------

// GetEncryption returns the bucket's default server-side encryption config.
func GetEncryption(ctx context.Context, cl *s3.Client, bucket string) (model.BucketEncryption, error) {
	out, err := cl.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(bucket)})
	if err != nil {
		if isNoSuchConfig(err) {
			return model.BucketEncryption{}, nil
		}
		return model.BucketEncryption{}, err
	}
	enc := model.BucketEncryption{}
	if out.ServerSideEncryptionConfiguration != nil {
		for _, r := range out.ServerSideEncryptionConfiguration.Rules {
			if r.ApplyServerSideEncryptionByDefault != nil {
				enc.Enabled = true
				enc.SSEAlgorithm = string(r.ApplyServerSideEncryptionByDefault.SSEAlgorithm)
				enc.KMSKeyID = aws.ToString(r.ApplyServerSideEncryptionByDefault.KMSMasterKeyID)
			}
			enc.BucketKeyEnabled = aws.ToBool(r.BucketKeyEnabled)
		}
	}
	return enc, nil
}

// PutEncryption sets (Enabled) or removes (disabled) default encryption.
func PutEncryption(ctx context.Context, cl *s3.Client, bucket string, enc model.BucketEncryption) error {
	if !enc.Enabled {
		_, err := cl.DeleteBucketEncryption(ctx, &s3.DeleteBucketEncryptionInput{Bucket: aws.String(bucket)})
		if isNoSuchConfig(err) {
			return nil
		}
		return err
	}
	alg := enc.SSEAlgorithm
	if alg == "" {
		alg = "AES256"
	}
	byDefault := &types.ServerSideEncryptionByDefault{SSEAlgorithm: types.ServerSideEncryption(alg)}
	if types.ServerSideEncryption(alg) == types.ServerSideEncryptionAwsKms && enc.KMSKeyID != "" {
		byDefault.KMSMasterKeyID = aws.String(enc.KMSKeyID)
	}
	_, err := cl.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket: aws.String(bucket),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{{
				ApplyServerSideEncryptionByDefault: byDefault,
				BucketKeyEnabled:                   aws.Bool(enc.BucketKeyEnabled),
			}},
		},
	})
	return err
}

// Public access block ---------------------------------------------------------

// GetPublicAccessBlock returns the bucket's public access block configuration.
func GetPublicAccessBlock(ctx context.Context, cl *s3.Client, bucket string) (model.PublicAccessBlock, error) {
	out, err := cl.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
	if err != nil {
		if isNoSuchConfig(err) {
			return model.PublicAccessBlock{Configured: false}, nil
		}
		return model.PublicAccessBlock{}, err
	}
	c := out.PublicAccessBlockConfiguration
	if c == nil {
		return model.PublicAccessBlock{Configured: false}, nil
	}
	return model.PublicAccessBlock{
		Configured:            true,
		BlockPublicAcls:       aws.ToBool(c.BlockPublicAcls),
		IgnorePublicAcls:      aws.ToBool(c.IgnorePublicAcls),
		BlockPublicPolicy:     aws.ToBool(c.BlockPublicPolicy),
		RestrictPublicBuckets: aws.ToBool(c.RestrictPublicBuckets),
	}, nil
}

// PutPublicAccessBlock writes the bucket's public access block configuration.
func PutPublicAccessBlock(ctx context.Context, cl *s3.Client, bucket string, cfg model.PublicAccessBlock) error {
	_, err := cl.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
		Bucket: aws.String(bucket),
		PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(cfg.BlockPublicAcls),
			IgnorePublicAcls:      aws.Bool(cfg.IgnorePublicAcls),
			BlockPublicPolicy:     aws.Bool(cfg.BlockPublicPolicy),
			RestrictPublicBuckets: aws.Bool(cfg.RestrictPublicBuckets),
		},
	})
	return err
}

// Bucket tagging --------------------------------------------------------------

// GetBucketTags returns the bucket's tag set.
func GetBucketTags(ctx context.Context, cl *s3.Client, bucket string) ([]model.Tag, error) {
	out, err := cl.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: aws.String(bucket)})
	if err != nil {
		if isNoSuchConfig(err) {
			return nil, nil
		}
		return nil, err
	}
	tags := make([]model.Tag, 0, len(out.TagSet))
	for _, t := range out.TagSet {
		tags = append(tags, model.Tag{Key: aws.ToString(t.Key), Value: aws.ToString(t.Value)})
	}
	return tags, nil
}

// PutBucketTags replaces the bucket's tag set (empty list clears it).
func PutBucketTags(ctx context.Context, cl *s3.Client, bucket string, tags []model.Tag) error {
	if len(tags) == 0 {
		_, err := cl.DeleteBucketTagging(ctx, &s3.DeleteBucketTaggingInput{Bucket: aws.String(bucket)})
		if isNoSuchConfig(err) {
			return nil
		}
		return err
	}
	set := make([]types.Tag, 0, len(tags))
	for _, t := range tags {
		set = append(set, types.Tag{Key: aws.String(t.Key), Value: aws.String(t.Value)})
	}
	_, err := cl.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket:  aws.String(bucket),
		Tagging: &types.Tagging{TagSet: set},
	})
	return err
}
