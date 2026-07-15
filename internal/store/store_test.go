package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	type secret struct {
		Access string `json:"access"`
		Secret string `json:"secret"`
	}
	in := []secret{{Access: "AKIA", Secret: "topsecret"}}

	if err := s.WriteEncrypted("conns.enc", in); err != nil {
		t.Fatalf("WriteEncrypted: %v", err)
	}

	// The file on disk must not contain the plaintext secret.
	raw, err := os.ReadFile(filepath.Join(dir, "conns.enc"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("encrypted file is empty")
	}
	for _, needle := range []string{"topsecret", "AKIA"} {
		if containsSub(raw, needle) {
			t.Errorf("plaintext %q leaked into encrypted file", needle)
		}
	}

	var out []secret
	ok, err := s.ReadEncrypted("conns.enc", &out)
	if err != nil || !ok {
		t.Fatalf("ReadEncrypted ok=%v err=%v", ok, err)
	}
	if len(out) != 1 || out[0].Secret != "topsecret" || out[0].Access != "AKIA" {
		t.Errorf("round-trip mismatch: %+v", out)
	}

	// Missing file is not an error.
	var none []secret
	ok, err = s.ReadEncrypted("absent.enc", &none)
	if err != nil || ok {
		t.Errorf("missing file: ok=%v err=%v", ok, err)
	}

	// The key file must be present and 0600.
	info, err := os.Stat(filepath.Join(dir, keyFile))
	if err != nil {
		t.Fatalf("key file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file perm = %v, want 0600", info.Mode().Perm())
	}
}

func containsSub(haystack []byte, needle string) bool {
	n := []byte(needle)
	for i := 0; i+len(n) <= len(haystack); i++ {
		if string(haystack[i:i+len(n)]) == needle {
			return true
		}
	}
	return false
}
