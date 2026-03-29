package client

import (
	"encoding/base64"
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

func TestKeyPublicKeyReturnsErrorWhenKeyIDMissing(t *testing.T) {
	privateKey := generateRSAKey(t)

	_, err := (key{
		Algorithm: supportedAlgorithm,
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
		Algorithm: supportedAlgorithm,
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
