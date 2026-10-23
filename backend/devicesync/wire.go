package devicesync

import (
	"bytes"
	"crypto/ecdh"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"slices"

	"github.com/gofiber/fiber/v3"
)

// Sizes the protocol fixes (protocol.md); the server takes nothing else.
const (
	idSize      = 16
	keySize     = 32
	spkiSize    = 91
	sealing     = 12 + 16
	pairPadded  = 512
	pairMessage = pairPadded + sealing
	wrapSize    = spkiSize + 12 + keySize + 16
)

var recordBuckets = []int{1 << 10, 4 << 10, 16 << 10, 64 << 10}

var deviceKinds = []string{"web", "android", "wear"}

var b64 = base64.RawURLEncoding.Strict()

func validRecordBody(body []byte) bool {
	return slices.Contains(recordBuckets, len(body)-sealing)
}

// fromB64 decodes a value that must be exactly size bytes long.
func fromB64(text string, size int) ([]byte, bool) {
	if len(text) != b64.EncodedLen(size) {
		return nil, false
	}
	raw, err := b64.DecodeString(text)
	return raw, err == nil && len(raw) == size
}

func recordFromB64(text string) ([]byte, bool) {
	if len(text) > b64.EncodedLen(recordBuckets[len(recordBuckets)-1]+sealing) {
		return nil, false
	}
	raw, err := b64.DecodeString(text)
	return raw, err == nil && validRecordBody(raw)
}

func validID(text string) bool {
	_, ok := fromB64(text, idSize)
	return ok
}

// p256 accepts only a P-256 public key in SubjectPublicKeyInfo form.
func p256(text string) ([]byte, bool) {
	raw, ok := fromB64(text, spkiSize)
	if !ok {
		return nil, false
	}
	key, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, false
	}
	pub, err := ecdhKey(key)
	return raw, err == nil && pub.Curve() == ecdh.P256()
}

type ecdhConverter interface {
	ECDH() (*ecdh.PublicKey, error)
}

func ecdhKey(key any) (*ecdh.PublicKey, error) {
	converter, ok := key.(ecdhConverter)
	if !ok {
		return nil, x509.ErrUnsupportedAlgorithm
	}
	return converter.ECDH()
}

// decode reads a JSON object with exactly the fields of into, at most limit bytes.
func decode(c fiber.Ctx, limit int, into any) bool {
	body := c.Body()
	if len(body) > limit {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if dec.Decode(into) != nil {
		return false
	}
	_, err := dec.Token()
	return err == io.EOF
}
