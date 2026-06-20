package main

import (
	"context"
	"fmt"
	"time"

	"s3scalpel/internal/model"
	"s3scalpel/internal/s3x"
)

// ConfigService manages S3-compatible connection configurations. The list is
// shared across all windows (single backend process) and persisted to disk.
type ConfigService struct{ core *Core }

// List returns all saved connections.
func (s *ConfigService) List() []model.Connection {
	s.core.connMu.RLock()
	defer s.core.connMu.RUnlock()
	out := make([]model.Connection, len(s.core.conns))
	copy(out, s.core.conns)
	return out
}

// Save creates or updates a connection and broadcasts a change event.
func (s *ConfigService) Save(conn model.Connection) (model.Connection, error) {
	if conn.Name == "" {
		return conn, fmt.Errorf("display name is required")
	}
	if conn.Endpoint == "" {
		return conn, fmt.Errorf("endpoint is required")
	}
	s.core.connMu.Lock()
	if conn.ID == "" {
		conn.ID = randID()
		conn.CreatedAt = time.Now().UnixMilli()
		s.core.conns = append(s.core.conns, conn)
	} else {
		found := false
		for i, c := range s.core.conns {
			if c.ID == conn.ID {
				if conn.CreatedAt == 0 {
					conn.CreatedAt = c.CreatedAt
				}
				s.core.conns[i] = conn
				found = true
				break
			}
		}
		if !found {
			s.core.conns = append(s.core.conns, conn)
		}
	}
	s.core.connMu.Unlock()

	if err := s.core.saveConnections(); err != nil {
		return conn, err
	}
	s.core.clients.Invalidate(conn.ID)
	s.core.emit("configs:changed", nil)
	return conn, nil
}

// Delete removes a connection by id.
func (s *ConfigService) Delete(id string) error {
	s.core.connMu.Lock()
	out := s.core.conns[:0]
	for _, c := range s.core.conns {
		if c.ID != id {
			out = append(out, c)
		}
	}
	s.core.conns = out
	s.core.connMu.Unlock()

	if err := s.core.saveConnections(); err != nil {
		return err
	}
	s.core.clients.Invalidate(id)
	s.core.emit("configs:changed", nil)
	return nil
}

// Test verifies a (possibly unsaved) connection by listing buckets.
func (s *ConfigService) Test(conn model.Connection) model.TestResult {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cl, err := s.core.clients.Client(ctx, conn)
	if err != nil {
		return model.TestResult{OK: false, Message: err.Error()}
	}
	buckets, err := s3x.ListBuckets(ctx, cl)
	if err != nil {
		return model.TestResult{OK: false, Message: friendlyErr(err)}
	}
	return model.TestResult{OK: true, Message: "connected", BucketCount: len(buckets)}
}

func friendlyErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
