package config

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

const (
	supportedJWTAlgorithm     = "RS256"
	supportedHMACJWTAlgorithm = "HS256"
)

func newDocumentFromBytes(payload []byte) (document, error) {
	var keyDocument document
	if err := json.Unmarshal(payload, &keyDocument); err != nil {
		return document{}, fmt.Errorf("%w: decode jwks payload: %w", ErrInvalidKeySet, err)
	}

	return keyDocument, nil
}

func (d document) publicKeys() (map[string]*rsa.PublicKey, error) {
	if len(d.Keys) == 0 {
		return nil, fmt.Errorf("%w: keys is required", ErrInvalidKeySet)
	}

	publicKeys := make(map[string]*rsa.PublicKey)
	for _, keyValue := range d.Keys {
		publicKey, err := keyValue.publicKey()
		if err != nil {
			return nil, err
		}

		if publicKey == nil {
			continue
		}

		if _, exists := publicKeys[keyValue.KeyID]; exists {
			return nil, fmt.Errorf("%w: duplicate kid %q", ErrInvalidKeySet, keyValue.KeyID)
		}

		publicKeys[keyValue.KeyID] = publicKey
	}

	if len(publicKeys) == 0 {
		return nil, fmt.Errorf("%w: no usable rsa signing keys", ErrInvalidKeySet)
	}

	return publicKeys, nil
}

func (k key) publicKey() (*rsa.PublicKey, error) {
	if k.KeyType != "RSA" {
		return nil, nil
	}

	if k.Use != "" && k.Use != "sig" {
		return nil, nil
	}

	if k.Algorithm != "" && k.Algorithm != supportedJWTAlgorithm {
		return nil, nil
	}

	trimmedKeyID := strings.TrimSpace(k.KeyID)
	if trimmedKeyID == "" {
		return nil, fmt.Errorf("%w: kid is required", ErrInvalidKeySet)
	}

	if strings.TrimSpace(k.Modulus) == "" {
		return nil, fmt.Errorf("%w: modulus is required for kid %q", ErrInvalidKeySet, trimmedKeyID)
	}

	if strings.TrimSpace(k.Exponent) == "" {
		return nil, fmt.Errorf("%w: exponent is required for kid %q", ErrInvalidKeySet, trimmedKeyID)
	}

	modulusBytes, err := base64.RawURLEncoding.DecodeString(k.Modulus)
	if err != nil {
		return nil, fmt.Errorf("%w: decode modulus for kid %q: %w", ErrInvalidKeySet, trimmedKeyID, err)
	}

	exponentBytes, err := base64.RawURLEncoding.DecodeString(k.Exponent)
	if err != nil {
		return nil, fmt.Errorf("%w: decode exponent for kid %q: %w", ErrInvalidKeySet, trimmedKeyID, err)
	}

	if len(modulusBytes) == 0 {
		return nil, fmt.Errorf("%w: modulus is empty for kid %q", ErrInvalidKeySet, trimmedKeyID)
	}

	if len(exponentBytes) == 0 {
		return nil, fmt.Errorf("%w: exponent is empty for kid %q", ErrInvalidKeySet, trimmedKeyID)
	}

	exponent, err := decodeExponent(exponentBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: decode exponent for kid %q: %w", ErrInvalidKeySet, trimmedKeyID, err)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(modulusBytes),
		E: exponent,
	}, nil
}

func decodeExponent(rawExponent []byte) (int, error) {
	exponentValue := 0
	for _, part := range rawExponent {
		if exponentValue > int(^uint(0)>>1)>>8 {
			return 0, fmt.Errorf("exponent overflows int")
		}

		exponentValue = (exponentValue << 8) | int(part)
	}

	if exponentValue <= 0 {
		return 0, fmt.Errorf("exponent must be greater than zero")
	}

	return exponentValue, nil
}
