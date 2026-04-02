package config

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
)

func TestDocumentPublicKeysReturnsErrorWhenKeysEmpty(t *testing.T) {
	_, err := (document{}).publicKeys()
	if err == nil {
		t.Fatal("publicKeys returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}

func TestNewDocumentFromBytesReturnsErrorWhenPayloadInvalid(t *testing.T) {
	_, err := newDocumentFromBytes([]byte(`{`))
	if err == nil {
		t.Fatal("newDocumentFromBytes returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}

func TestDocumentPublicKeysReturnsErrorWhenDuplicateKeyID(t *testing.T) {
	privateKey := generateRSAKey(t)
	keyValue := newRSAKey("key-1", &privateKey.PublicKey)

	_, err := (document{Keys: []key{keyValue, keyValue}}).publicKeys()
	if err == nil {
		t.Fatal("publicKeys returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}

func TestDocumentPublicKeysIgnoresUnsupportedKeys(t *testing.T) {
	privateKey := generateRSAKey(t)

	publicKeys, err := (document{
		Keys: []key{
			{
				KeyID:   "ec-key",
				KeyType: "EC",
			},
			{
				Algorithm: "RS512",
				KeyID:     "wrong-algorithm",
				KeyType:   "RSA",
				Use:       "sig",
				Modulus:   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
				Exponent:  base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
			},
			newRSAKey("key-1", &privateKey.PublicKey),
		},
	}).publicKeys()
	if err != nil {
		t.Fatalf("publicKeys returned error: %v", err)
	}

	if len(publicKeys) != 1 {
		t.Fatalf("len(publicKeys) = %d, want %d", len(publicKeys), 1)
	}

	if !equalPublicKeys(publicKeys["key-1"], &privateKey.PublicKey) {
		t.Fatal("publicKeys[key-1] does not match the valid RSA key")
	}
}

func TestDocumentPublicKeysReturnsErrorWhenNoUsableSigningKeys(t *testing.T) {
	_, err := (document{
		Keys: []key{
			{
				KeyID:   "ec-key",
				KeyType: "EC",
			},
		},
	}).publicKeys()
	if err == nil {
		t.Fatal("publicKeys returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}

func TestKeyPublicKeyReturnsErrorWhenKeyIDMissing(t *testing.T) {
	privateKey := generateRSAKey(t)

	_, err := (key{
		Algorithm: supportedJWTAlgorithm,
		KeyType:   "RSA",
		Use:       "sig",
		Modulus:   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
		Exponent:  base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
	}).publicKey()
	if err == nil {
		t.Fatal("publicKey returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}

func TestKeyPublicKeyReturnsErrorWhenExponentInvalid(t *testing.T) {
	privateKey := generateRSAKey(t)

	_, err := (key{
		Algorithm: supportedJWTAlgorithm,
		KeyID:     "key-1",
		KeyType:   "RSA",
		Use:       "sig",
		Modulus:   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
		Exponent:  "%%%invalid%%%",
	}).publicKey()
	if err == nil {
		t.Fatal("publicKey returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}

func TestDecodeExponent(t *testing.T) {
	exponent, err := decodeExponent([]byte{0x01, 0x00, 0x01})
	if err != nil {
		t.Fatalf("decodeExponent returned error: %v", err)
	}

	if exponent != 65537 {
		t.Fatalf("exponent = %d, want %d", exponent, 65537)
	}
}

func TestDecodeExponentReturnsErrorWhenZero(t *testing.T) {
	_, err := decodeExponent(nil)
	if err == nil {
		t.Fatal("decodeExponent returned nil error")
	}
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	return privateKey
}

func mustMarshalDocument(t *testing.T, keys ...key) string {
	t.Helper()

	payload, err := json.Marshal(document{Keys: keys})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	return string(payload)
}

func newRSAKey(keyID string, publicKey *rsa.PublicKey) key {
	return key{
		Algorithm: supportedJWTAlgorithm,
		Exponent:  base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
		KeyID:     keyID,
		KeyType:   "RSA",
		Modulus:   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		Use:       "sig",
	}
}

func equalPublicKeys(left *rsa.PublicKey, right *rsa.PublicKey) bool {
	if left == nil || right == nil {
		return false
	}

	return left.E == right.E && left.N.Cmp(right.N) == 0
}
