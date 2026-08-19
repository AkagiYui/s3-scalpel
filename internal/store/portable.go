package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

// EnvelopeFormat tags a passphrase-encrypted export so Import can recognise one
// without guessing.
const EnvelopeFormat = "s3scalpel-encrypted-export"

// Argon2id parameters. These are the values RFC 9106 recommends for the
// memory-constrained second option, which a desktop app can afford on any
// machine that runs the app at all.
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // KiB
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	saltLen             = 16
)

// Envelope is the on-disk shape of a passphrase-encrypted export. Only the
// ciphertext carries data; every other field is public parameter material
// needed to derive the same key again.
type Envelope struct {
	Format     string `json:"format"`
	Version    int    `json:"version"`
	KDF        string `json:"kdf"`
	Time       uint32 `json:"time"`
	Memory     uint32 `json:"memory"`
	Threads    uint8  `json:"threads"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// ErrWrongPassphrase is returned when an envelope will not decrypt, which in
// practice always means the passphrase is wrong (GCM also catches tampering).
var ErrWrongPassphrase = errors.New("wrong passphrase, or the file is damaged")

// SealExport encrypts v under a passphrase, returning the JSON envelope to write
// to disk. Credentials leaving the app's own encrypted storage should never
// travel as plaintext.
func SealExport(v any, passphrase string) ([]byte, error) {
	if passphrase == "" {
		return nil, errors.New("a passphrase is required to export credentials")
	}
	plain, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	gcm, err := envelopeCipher(passphrase, salt)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return json.MarshalIndent(Envelope{
		Format:     EnvelopeFormat,
		Version:    1,
		KDF:        "argon2id",
		Time:       argonTime,
		Memory:     argonMemory,
		Threads:    argonThreads,
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: gcm.Seal(nil, nonce, plain, nil),
	}, "", "  ")
}

// IsSealedExport reports whether data is a passphrase-encrypted envelope, so the
// caller knows to ask for a passphrase before importing.
func IsSealedExport(data []byte) bool {
	var probe struct {
		Format string `json:"format"`
	}
	return json.Unmarshal(data, &probe) == nil && probe.Format == EnvelopeFormat
}

// OpenExport decrypts an envelope into v.
func OpenExport(data []byte, passphrase string, v any) error {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	if env.Format != EnvelopeFormat {
		return errors.New("not an encrypted S3 Scalpel export")
	}
	if env.KDF != "argon2id" {
		return errors.New("unsupported key derivation: " + env.KDF)
	}
	// Honour the parameters recorded in the file rather than today's constants,
	// so an export stays readable after the defaults are tuned.
	key := argon2.IDKey([]byte(passphrase), env.Salt, env.Time, env.Memory, env.Threads, argonKeyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(env.Nonce) != gcm.NonceSize() {
		return ErrWrongPassphrase
	}
	plain, err := gcm.Open(nil, env.Nonce, env.Ciphertext, nil)
	if err != nil {
		return ErrWrongPassphrase
	}
	return json.Unmarshal(plain, v)
}

func envelopeCipher(passphrase string, salt []byte) (cipher.AEAD, error) {
	key := argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
