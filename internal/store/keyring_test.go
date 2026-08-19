package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

// TestMain keeps every test in this package off the real OS credential store.
func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

// resetKeyring clears the mock store between tests.
func resetKeyring(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	t.Cleanup(keyring.MockInit)
}

func TestKeyIsStoredInTheCredentialStore(t *testing.T) {
	resetKeyring(t)
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	k, err := s.key()
	if err != nil {
		t.Fatal(err)
	}
	if len(k) != 32 {
		t.Fatalf("key is %d bytes, want 32", len(k))
	}
	if s.KeySource() != KeySourceKeyring {
		t.Errorf("key source = %q, want %q", s.KeySource(), KeySourceKeyring)
	}

	// A second Store over the same data must resolve the same key, or every
	// saved credential would become undecryptable.
	again, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := again.key()
	if err != nil {
		t.Fatal(err)
	}
	if string(k) != string(k2) {
		t.Error("a second Store resolved a different key")
	}
}

func TestLegacyFileKeyIsAdoptedAndPromoted(t *testing.T) {
	resetKeyring(t)
	dir := t.TempDir()

	// A pre-keyring installation left its key on disk.
	legacy := make([]byte, 32)
	for i := range legacy {
		legacy[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(dir, keyFile), legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.key()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(legacy) {
		t.Fatal("the existing key must be adopted, never replaced")
	}
	if s.KeySource() != KeySourceKeyring {
		t.Errorf("key source = %q, want the key promoted to %q", s.KeySource(), KeySourceKeyring)
	}
	if _, err := os.Stat(filepath.Join(dir, keyFile)); !os.IsNotExist(err) {
		t.Error("the on-disk key should be removed once it is in the credential store")
	}
}

func TestFallsBackToFileWhenNoCredentialStore(t *testing.T) {
	resetKeyring(t)
	keyring.MockInitWithError(errors.New("no credential store on this host"))
	dir := t.TempDir()

	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	k, err := s.key()
	if err != nil {
		t.Fatalf("a host without a credential store must still work: %v", err)
	}
	if len(k) != 32 {
		t.Fatalf("key is %d bytes, want 32", len(k))
	}
	if s.KeySource() != KeySourceFile {
		t.Errorf("key source = %q, want %q", s.KeySource(), KeySourceFile)
	}
	info, err := os.Stat(filepath.Join(dir, keyFile))
	if err != nil {
		t.Fatalf("fallback key file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("fallback key file perm = %v, want 0600", info.Mode().Perm())
	}
}

// The dangerous case: the credential store is present but refuses to answer
// (the user dismissed the unlock prompt). Minting a fresh key there would leave
// every saved credential undecryptable, so the store must refuse instead.
func TestUnreachableCredentialStoreDoesNotMintANewKey(t *testing.T) {
	resetKeyring(t)
	dir := t.TempDir()

	// Establish a key first, so real ciphertext would exist.
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.key(); err != nil {
		t.Fatal(err)
	}

	keyring.MockInitWithError(errors.New("user denied access"))
	locked, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locked.key(); err == nil {
		t.Fatal("a locked credential store must surface an error, not silently re-key")
	}
	if _, err := os.Stat(filepath.Join(dir, keyFile)); !os.IsNotExist(err) {
		t.Error("no fallback key file should be written when a keyring key already exists")
	}
}

func TestKeySourceDoesNotCreateAKey(t *testing.T) {
	resetKeyring(t)
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := s.KeySource(); got != "" {
		t.Errorf("KeySource on a fresh install = %q, want empty", got)
	}
	if _, err := keyring.Get(keyringService, keyringUser); !errors.Is(err, keyring.ErrNotFound) {
		t.Error("merely asking where the key lives must not create one")
	}

	// Once something needs the key, the source becomes reportable.
	if _, err := s.key(); err != nil {
		t.Fatal(err)
	}
	if got := s.KeySource(); got != KeySourceKeyring {
		t.Errorf("KeySource after use = %q, want %q", got, KeySourceKeyring)
	}
}
