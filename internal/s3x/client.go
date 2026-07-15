// Package s3x wraps the AWS SDK for Go v2 to talk to arbitrary S3-compatible
// endpoints (MinIO, Cloudflare R2, generic providers).
package s3x

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
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
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%t|%s|%s|%s|%t|%s|%s",
		c.ID, c.Endpoint, c.Region, c.PathStyle, c.AccessKey, c.SecretKey,
		c.SessionToken, c.SkipTLSVerify, c.ProxyURL, c.CACert)))
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
	httpClient, err := buildHTTPClient(c)
	if err != nil {
		return nil, err
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, c.SessionToken),
		),
		awsconfig.WithHTTPClient(httpClient),
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

// buildHTTPClient constructs the HTTP client used for S3 requests, honouring the
// connection's proxy, custom CA and TLS-verification options. When none are set
// it returns a client with a plain default transport.
func buildHTTPClient(c model.Connection) (*http.Client, error) {
	if !c.SkipTLSVerify && c.ProxyURL == "" && c.CACert == "" {
		return &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}, nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsCfg := &tls.Config{}

	if c.SkipTLSVerify {
		tlsCfg.InsecureSkipVerify = true
	}
	if c.CACert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(c.CACert)) {
			return nil, fmt.Errorf("invalid CA certificate PEM")
		}
		tlsCfg.RootCAs = pool
	}
	transport.TLSClientConfig = tlsCfg

	if c.ProxyURL != "" {
		pu, err := url.Parse(c.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(pu)
	}
	return &http.Client{Transport: transport}, nil
}
