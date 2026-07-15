package main

import (
	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// BucketService exposes bucket-level configuration: region, versioning, policy,
// CORS, lifecycle, default encryption, public access block and bucket tagging.
type BucketService struct{ core *Core }

// Location returns a bucket's region.
func (s *BucketService) Location(connID, bucket string) (string, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return "", err
	}
	return s3x.GetBucketLocation(ctx, cl, bucket)
}

// GetVersioning returns a bucket's versioning configuration.
func (s *BucketService) GetVersioning(connID, bucket string) (model.BucketVersioning, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return model.BucketVersioning{}, err
	}
	return s3x.GetVersioning(ctx, cl, bucket)
}

// SetVersioning enables or suspends versioning on a bucket.
func (s *BucketService) SetVersioning(connID, bucket string, enabled bool) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.SetVersioning(ctx, cl, bucket, enabled)
}

// GetPolicy returns the raw JSON bucket policy ("" if none).
func (s *BucketService) GetPolicy(connID, bucket string) (string, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return "", err
	}
	return s3x.GetPolicy(ctx, cl, bucket)
}

// PutPolicy sets or (with an empty string) deletes the bucket policy.
func (s *BucketService) PutPolicy(connID, bucket, policy string) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.PutPolicy(ctx, cl, bucket, policy)
}

// GetCORS returns the bucket's CORS rules.
func (s *BucketService) GetCORS(connID, bucket string) ([]model.CORSRule, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return nil, err
	}
	return s3x.GetCORS(ctx, cl, bucket)
}

// PutCORS replaces the bucket's CORS rules (empty list clears the config).
func (s *BucketService) PutCORS(connID, bucket string, rules []model.CORSRule) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.PutCORS(ctx, cl, bucket, rules)
}

// GetLifecycle returns the bucket's lifecycle rules.
func (s *BucketService) GetLifecycle(connID, bucket string) ([]model.LifecycleRule, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return nil, err
	}
	return s3x.GetLifecycle(ctx, cl, bucket)
}

// PutLifecycle replaces the bucket's lifecycle rules (empty list clears them).
func (s *BucketService) PutLifecycle(connID, bucket string, rules []model.LifecycleRule) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.PutLifecycle(ctx, cl, bucket, rules)
}

// GetEncryption returns the bucket's default server-side encryption config.
func (s *BucketService) GetEncryption(connID, bucket string) (model.BucketEncryption, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return model.BucketEncryption{}, err
	}
	return s3x.GetEncryption(ctx, cl, bucket)
}

// PutEncryption sets or removes default server-side encryption.
func (s *BucketService) PutEncryption(connID, bucket string, enc model.BucketEncryption) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.PutEncryption(ctx, cl, bucket, enc)
}

// GetPublicAccessBlock returns the bucket's public access block configuration.
func (s *BucketService) GetPublicAccessBlock(connID, bucket string) (model.PublicAccessBlock, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return model.PublicAccessBlock{}, err
	}
	return s3x.GetPublicAccessBlock(ctx, cl, bucket)
}

// PutPublicAccessBlock writes the bucket's public access block configuration.
func (s *BucketService) PutPublicAccessBlock(connID, bucket string, cfg model.PublicAccessBlock) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.PutPublicAccessBlock(ctx, cl, bucket, cfg)
}

// GetTags returns the bucket's tag set.
func (s *BucketService) GetTags(connID, bucket string) ([]model.Tag, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return nil, err
	}
	return s3x.GetBucketTags(ctx, cl, bucket)
}

// PutTags replaces the bucket's tag set (empty list clears it).
func (s *BucketService) PutTags(connID, bucket string, tags []model.Tag) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.PutBucketTags(ctx, cl, bucket, tags)
}
