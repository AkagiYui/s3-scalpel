// Package store provides small atomic JSON persistence helpers rooted at the
// platform application-data directory.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Store owns a base directory and serialises writes per file path.
type Store struct {
	baseDir string
	mu      sync.Mutex
}

// New creates a Store rooted at baseDir, creating the directory if needed.
func New(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}
	return &Store{baseDir: baseDir}, nil
}

// BaseDir returns the root directory.
func (s *Store) BaseDir() string { return s.baseDir }

// Path resolves a file name relative to the base directory.
func (s *Store) Path(name string) string { return filepath.Join(s.baseDir, name) }

// ReadJSON unmarshals the named file into v. Missing files are not an error;
// v is left untouched so callers can supply defaults beforehand.
func (s *Store) ReadJSON(name string, v any) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.Path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, err
	}
	return true, nil
}

// WriteJSON atomically writes v as indented JSON to the named file.
func (s *Store) WriteJSON(name string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.Path(name), data)
}

// writeFileAtomic writes via a temp file + rename so a crash never leaves a
// half-written file.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
