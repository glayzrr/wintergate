package config

import (
	"errors"
	"testing"
	"time"
)

func TestRegistryPublicKeyReturnsErrorWhenKeySetUnavailable(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.PublicKey("key-1")
	if err == nil {
		t.Fatal("PublicKey returned nil error")
	}

	if !errors.Is(err, ErrKeySetUnavailable) {
		t.Fatalf("error = %v, want ErrKeySetUnavailable", err)
	}
}

func TestRegistryRegisterStoresKeysAndConfig(t *testing.T) {
	privateKey := generateRSAKey(t)
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		JWTAlgorithm: supportedJWTAlgorithm,
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWKS:         []byte(mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey))),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	publicKey, err := registry.PublicKey("key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error: %v", err)
	}

	if !equalPublicKeys(publicKey, &privateKey.PublicKey) {
		t.Fatal("publicKey does not match the registered key")
	}

	if string(registry.config.JWKS) != mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey)) {
		t.Fatal("JWKS does not match the registered document")
	}

	snapshot, found := registry.Snapshot()
	if !found {
		t.Fatal("Snapshot did not return config")
	}

	if snapshot.JWTIssuer != "auth-service" {
		t.Fatalf("JWTIssuer = %q, want %q", snapshot.JWTIssuer, "auth-service")
	}
}

func TestRegistryRegisterReplacesExistingKeys(t *testing.T) {
	firstPrivateKey := generateRSAKey(t)
	secondPrivateKey := generateRSAKey(t)
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		JWTAlgorithm: supportedJWTAlgorithm,
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWKS:         []byte(mustMarshalDocument(t, newRSAKey("key-1", &firstPrivateKey.PublicKey))),
	})
	if err != nil {
		t.Fatalf("first Register returned error: %v", err)
	}

	err = registry.Register(RuntimeConfig{
		JWTAlgorithm: supportedJWTAlgorithm,
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWKS:         []byte(mustMarshalDocument(t, newRSAKey("key-2", &secondPrivateKey.PublicKey))),
	})
	if err != nil {
		t.Fatalf("second Register returned error: %v", err)
	}

	_, err = registry.PublicKey("key-1")
	if err == nil {
		t.Fatal("PublicKey returned nil error for replaced key")
	}

	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("error = %v, want ErrKeyNotFound", err)
	}

	publicKey, err := registry.PublicKey("key-2")
	if err != nil {
		t.Fatalf("PublicKey returned error for key-2: %v", err)
	}

	if !equalPublicKeys(publicKey, &secondPrivateKey.PublicKey) {
		t.Fatal("publicKey does not match the replacement key")
	}
}

func TestRegistryRegisterReturnsErrorWhenPayloadInvalid(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		JWTAlgorithm: supportedJWTAlgorithm,
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWKS:         []byte(`{`),
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}
