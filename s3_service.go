package main

import (
	"context"
	"time"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// S3Service exposes immediate (non-queued) S3 operations: bucket management,
// listing, metadata, tags, versions, presigning and folder creation.
type S3Service struct{ core *Core }

func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}

// ListBuckets lists the buckets visible to a connection.
func (s *S3Service) ListBuckets(connID string) ([]model.BucketInfo, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return nil, err
	}
	return s3x.ListBuckets(ctx, cl)
}

// CreateBucket creates a new bucket.
func (s *S3Service) CreateBucket(connID, name string) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, conn, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.CreateBucket(ctx, cl, name, conn.Region)
}

// DeleteBucket removes an empty bucket.
func (s *S3Service) DeleteBucket(connID, name string) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.DeleteBucket(ctx, cl, name)
}

// ListObjects returns one folder-style page of a bucket/prefix.
func (s *S3Service) ListObjects(connID, bucket, prefix, token string) (model.ListResult, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return model.ListResult{}, err
	}
	return s3x.ListObjects(ctx, cl, bucket, prefix, token, 1000)
}

// ListAll recursively lists every object under a prefix (for folder operations).
func (s *S3Service) ListAll(connID, bucket, prefix string) ([]model.ObjectEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return nil, err
	}
	return s3x.ListAllObjects(ctx, cl, bucket, prefix)
}

// Properties returns full object metadata (HeadObject).
func (s *S3Service) Properties(connID, bucket, key, versionID string) (model.ObjectProperties, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return model.ObjectProperties{}, err
	}
	return s3x.HeadObject(ctx, cl, bucket, key, versionID)
}

// PresignGet returns a presigned GET URL valid for expirySeconds.
func (s *S3Service) PresignGet(connID, bucket, key, versionID string, expirySeconds int) (string, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return "", err
	}
	if expirySeconds <= 0 {
		expirySeconds = 3600
	}
	return s3x.PresignGet(ctx, cl, bucket, key, versionID, time.Duration(expirySeconds)*time.Second)
}

// GetTags returns an object's tag set.
func (s *S3Service) GetTags(connID, bucket, key string) ([]model.Tag, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return nil, err
	}
	return s3x.GetTags(ctx, cl, bucket, key)
}

// PutTags replaces an object's tag set.
func (s *S3Service) PutTags(connID, bucket, key string, tags []model.Tag) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	return s3x.PutTags(ctx, cl, bucket, key, tags)
}

// VersioningEnabled reports whether a bucket has versioning enabled.
func (s *S3Service) VersioningEnabled(connID, bucket string) (bool, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return false, err
	}
	return s3x.VersioningEnabled(ctx, cl, bucket)
}

// ListVersions returns all versions and delete markers for a key.
func (s *S3Service) ListVersions(connID, bucket, key string) ([]model.ObjectVersion, error) {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return nil, err
	}
	return s3x.ListVersions(ctx, cl, bucket, key)
}

// CreateFolder creates a zero-byte folder placeholder under a prefix.
func (s *S3Service) CreateFolder(connID, bucket, prefix, name string) error {
	ctx, cancel := opCtx()
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return err
	}
	key := s3x.JoinKey(prefix, name) + "/"
	return s3x.CreateFolder(ctx, cl, bucket, key)
}

// CheckCapabilities probes which operations the connection's credentials are
// permitted to perform. Pass an empty bucket for account-level probes only.
func (s *S3Service) CheckCapabilities(connID, bucket string) ([]model.Capability, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cl, _, err := s.core.clientFor(ctx, connID)
	if err != nil {
		return nil, err
	}
	return s3x.CheckCapabilities(ctx, cl, bucket), nil
}
