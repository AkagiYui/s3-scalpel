package store

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/zalando/go-keyring"
)

// Where the data-encryption key lives in the OS credential store.
const (
	keyringService = "S3 Scalpel"
	keyringUser    = "credential-encryption-key"

	// keyringMarker records that the key was successfully placed in the OS
	// credential store. It carries no secret — it exists so that a later failure
	// to reach that store can be told apart from a host that never had one.
	keyringMarker = ".s3scalpel.keyring"
)

// keySource says where the active encryption key came from, for the About panel.
type keySource string

const (
	// KeySourceKeyring means the key lives in the OS credential store (macOS
	// Keychain, Windows Credential Manager, Linux Secret Service).
	KeySourceKeyring keySource = "keyring"
	// KeySourceFile means the key sits in a 0600 file beside the data, which is
	// the fallback when no credential store is reachable (headless Linux, CI).
	KeySourceFile keySource = "file"
)

// KeySource reports where the encryption key is stored. It never resolves the
// key itself — asking would create one, and a purely informational call has no
// business minting a credential-store entry — so it reports "" until something
// has actually needed the key.
func (s *Store) KeySource() keySource {
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	if s.keySrc != "" {
		return s.keySrc
	}
	if _, err := os.Stat(s.Path(keyringMarker)); err == nil {
		return KeySourceKeyring
	}
	if _, err := os.Stat(s.Path(keyFile)); err == nil {
		return KeySourceFile
	}
	return ""
}

// key returns the per-installation encryption key.
//
// Resolution order matters more than it looks. The key is read from the OS
// credential store first; a key found only in the legacy on-disk file is
// promoted into the credential store and the file removed. A credential store
// that is present but *fails* (the user dismissed the unlock prompt, the daemon
// is down) is a hard error rather than a cue to mint a fresh key — generating
// one there would silently render every saved credential undecryptable.
func (s *Store) key() ([]byte, error) {
	s.keyMu.Lock()
	defer s.keyMu.Unlock()
	if s.cachedKey != nil {
		return s.cachedKey, nil
	}

	stored, err := keyring.Get(keyringService, keyringUser)
	switch {
	case err == nil:
		k, decErr := base64.StdEncoding.DecodeString(stored)
		if decErr != nil || len(k) != 32 {
			return nil, errors.New("the stored encryption key is malformed")
		}
		s.cachedKey, s.keySrc = k, KeySourceKeyring
		return k, nil
	case errors.Is(err, keyring.ErrNotFound):
		// Nothing stored yet: fall through to migration or generation.
	default:
		// The credential store would not answer. If a key was previously stored
		// there, refuse: minting a fresh one would orphan every saved credential
		// behind a key that cannot decrypt them. A host that never had a
		// credential store has nothing to lose, so it falls through.
		if _, statErr := os.Stat(s.Path(keyFile)); statErr == nil {
			return s.useFileKey()
		}
		if _, statErr := os.Stat(s.Path(keyringMarker)); statErr == nil {
			return nil, fmt.Errorf("the OS credential store holding the encryption key is unavailable: %w", err)
		}
	}

	// Legacy on-disk key: adopt it, then move it into the credential store.
	if data, readErr := os.ReadFile(s.Path(keyFile)); readErr == nil && len(data) == 32 {
		if s.promote(data) {
			_ = os.Remove(s.Path(keyFile))
			s.cachedKey, s.keySrc = data, KeySourceKeyring
		} else {
			s.cachedKey, s.keySrc = data, KeySourceFile
		}
		return s.cachedKey, nil
	}

	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil, err
	}
	if s.promote(k) {
		s.cachedKey, s.keySrc = k, KeySourceKeyring
		return k, nil
	}
	if err := os.WriteFile(s.Path(keyFile), k, 0o600); err != nil {
		return nil, err
	}
	s.cachedKey, s.keySrc = k, KeySourceFile
	return k, nil
}

// promote writes the key into the OS credential store and verifies it reads
// back, reporting whether the store can be relied on. Verification matters: some
// Linux setups accept a write into a session keyring that evaporates on logout.
func (s *Store) promote(k []byte) bool {
	encoded := base64.StdEncoding.EncodeToString(k)
	if err := keyring.Set(keyringService, keyringUser, encoded); err != nil {
		return false
	}
	got, err := keyring.Get(keyringService, keyringUser)
	if err != nil || got != encoded {
		return false
	}
	_ = os.WriteFile(s.Path(keyringMarker), nil, 0o600)
	return true
}

// useFileKey falls back to the on-disk key. Callers hold s.keyMu.
func (s *Store) useFileKey() ([]byte, error) {
	data, err := os.ReadFile(s.Path(keyFile))
	if err != nil {
		return nil, err
	}
	if len(data) != 32 {
		return nil, errors.New("the on-disk encryption key is malformed")
	}
	s.cachedKey, s.keySrc = data, KeySourceFile
	return data, nil
}
