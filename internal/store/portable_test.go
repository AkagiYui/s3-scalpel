package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

type exportPayload struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

func TestSealedExportRoundTrip(t *testing.T) {
	in := exportPayload{Name: "prod", Secret: "AKIA-super-secret"}

	sealed, err := SealExport(in, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !IsSealedExport(sealed) {
		t.Fatal("the envelope should be recognised as sealed")
	}
	// The whole point: the secret must not be readable in the written file.
	if bytes.Contains(sealed, []byte("AKIA-super-secret")) {
		t.Fatal("the plaintext secret leaked into the exported file")
	}

	var out exportPayload
	if err := OpenExport(sealed, "correct horse battery staple", &out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("round-trip produced %+v, want %+v", out, in)
	}
}

func TestSealedExportRejectsWrongPassphrase(t *testing.T) {
	sealed, err := SealExport(exportPayload{Secret: "s"}, "right")
	if err != nil {
		t.Fatal(err)
	}
	var out exportPayload
	if err := OpenExport(sealed, "wrong", &out); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("OpenExport with a wrong passphrase = %v, want ErrWrongPassphrase", err)
	}
}

func TestSealedExportDetectsTampering(t *testing.T) {
	sealed, err := SealExport(exportPayload{Secret: "s"}, "pw")
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(sealed, &env); err != nil {
		t.Fatal(err)
	}
	env.Ciphertext[0] ^= 0xff
	tampered, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	var out exportPayload
	if err := OpenExport(tampered, "pw", &out); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("a modified ciphertext must not decrypt, got %v", err)
	}
}

func TestSealedExportRequiresAPassphrase(t *testing.T) {
	if _, err := SealExport(exportPayload{}, ""); err == nil {
		t.Error("sealing without a passphrase should be refused")
	}
}

func TestSaltAndNonceAreFreshEachTime(t *testing.T) {
	a, err := SealExport(exportPayload{Secret: "s"}, "pw")
	if err != nil {
		t.Fatal(err)
	}
	b, err := SealExport(exportPayload{Secret: "s"}, "pw")
	if err != nil {
		t.Fatal(err)
	}
	var ea, eb Envelope
	if err := json.Unmarshal(a, &ea); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &eb); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ea.Salt, eb.Salt) {
		t.Error("the salt must differ between exports")
	}
	if bytes.Equal(ea.Nonce, eb.Nonce) {
		t.Error("the nonce must differ between exports")
	}
	if bytes.Equal(ea.Ciphertext, eb.Ciphertext) {
		t.Error("identical plaintext must not produce identical ciphertext")
	}
}

func TestIsSealedExportRejectsPlainJSON(t *testing.T) {
	if IsSealedExport([]byte(`{"settings":{},"connections":[]}`)) {
		t.Error("a plain settings file must not be mistaken for an envelope")
	}
	if IsSealedExport([]byte("not json at all")) {
		t.Error("garbage must not be mistaken for an envelope")
	}
}

func TestOpenExportHonoursTheFilesOwnKDFParameters(t *testing.T) {
	sealed, err := SealExport(exportPayload{Secret: "s"}, "pw")
	if err != nil {
		t.Fatal(err)
	}
	var env Envelope
	if err := json.Unmarshal(sealed, &env); err != nil {
		t.Fatal(err)
	}
	if env.Time != argonTime || env.Memory != argonMemory || env.Threads != argonThreads {
		t.Fatal("the envelope should record the parameters it was sealed with")
	}
	// Rewriting the recorded parameters must break decryption, proving the file's
	// own values are used rather than today's constants.
	env.Time = argonTime + 1
	altered, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var out exportPayload
	if err := OpenExport(altered, "pw", &out); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("altered KDF parameters should fail to decrypt, got %v", err)
	}
}
