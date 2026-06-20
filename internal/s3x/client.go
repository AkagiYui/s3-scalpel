// Package s3x wraps the AWS SDK for Go v2 to talk to arbitrary S3-compatible
// endpoints (MinIO, Cloudflare R2, generic providers).
package s3x

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3scalpel/internal/model"
)

// Manager builds and caches *s3.Client instances per connection fingerprint so
// repeated operations reuse connections.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*s3.Client
}

// NewManager creates an empty client manager.
func NewManager() *Manager {
	return &Manager{clients: map[string]*s3.Client{}}
}

// fingerprint identifies a client by the connection fields that affect it, so an
// edited connection transparently gets a fresh client.
func fingerprint(c model.Connection) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%t|%s|%s",
		c.ID, c.Endpoint, c.Region, c.PathStyle, c.AccessKey, c.SecretKey)))
	return hex.EncodeToString(h[:8])
}

// Client returns a cached or freshly built client for the connection.
func (m *Manager) Client(ctx context.Context, c model.Connection) (*s3.Client, error) {
	key := fingerprint(c)
	m.mu.Lock()
	if cl, ok := m.clients[key]; ok {
		m.mu.Unlock()
		return cl, nil
	}
	m.mu.Unlock()

	cl, err := build(ctx, c)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.clients[key] = cl
	m.mu.Unlock()
	return cl, nil
}

// Invalidate drops any cached clients for a connection ID.
func (m *Manager) Invalidate(connID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.clients {
		// fingerprints are not reversible to the ID, so clear all on any change.
		// Cheap and safe: clients rebuild lazily.
		_ = k
	}
	m.clients = map[string]*s3.Client{}
}

// build constructs an S3 client for an S3-compatible endpoint. It disables the
// default request/response CRC32 checksums which break many non-AWS endpoints.
func build(ctx context.Context, c model.Connection) (*s3.Client, error) {
	region := c.Region
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""),
		),
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, err
	}
	endpoint := c.Endpoint
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = c.PathStyle
	})
	return client, nil
}
