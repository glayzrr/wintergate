package config

import (
	"bytes"
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

func TestRegistrySnapshotReturnsFalseWhenConfigMissing(t *testing.T) {
	registry := NewRegistry()

	_, found := registry.Snapshot()
	if found {
		t.Fatal("Snapshot returned config unexpectedly")
	}
}

func TestRegistryPublicKeyReturnsErrorWhenKeyIDMissing(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.PublicKey(" ")
	if err == nil {
		t.Fatal("PublicKey returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeyID) {
		t.Fatalf("error = %v, want ErrInvalidKeyID", err)
	}
}

func TestRegistryRegisterStoresKeysAndConfig(t *testing.T) {
	privateKey := generateRSAKey(t)
	registry := NewRegistry()
	jwksDocument := mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey))

	err := registry.Register(RuntimeConfig{
		JWTAlgorithm: supportedJWTAlgorithm,
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWKS:         []byte(jwksDocument),
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

	snapshot, found := registry.Snapshot()
	if !found {
		t.Fatal("Snapshot did not return config")
	}

	if snapshot.JWTIssuer != "auth-service" {
		t.Fatalf("JWTIssuer = %q, want %q", snapshot.JWTIssuer, "auth-service")
	}

	if snapshot.JWTAlgorithm != supportedJWTAlgorithm {
		t.Fatalf("JWTAlgorithm = %q, want %q", snapshot.JWTAlgorithm, supportedJWTAlgorithm)
	}

	if snapshot.JWTAudience != "wintergate" {
		t.Fatalf("JWTAudience = %q, want %q", snapshot.JWTAudience, "wintergate")
	}

	if snapshot.JWTClockSkew != time.Minute {
		t.Fatalf("JWTClockSkew = %s, want %s", snapshot.JWTClockSkew, time.Minute)
	}

	if len(snapshot.JWTSecret) != 0 {
		t.Fatalf("len(JWTSecret) = %d, want %d", len(snapshot.JWTSecret), 0)
	}

	if !bytes.Equal(snapshot.JWKS, []byte(jwksDocument)) {
		t.Fatal("snapshot.JWKS does not match the registered payload")
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

func TestRegistryRegisterStoresSecretForHS256(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		JWTAlgorithm: supportedHMACJWTAlgorithm,
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	snapshot, found := registry.Snapshot()
	if !found {
		t.Fatal("Snapshot did not return config")
	}

	if string(snapshot.JWTSecret) != "shared-secret" {
		t.Fatalf("JWTSecret = %q, want %q", string(snapshot.JWTSecret), "shared-secret")
	}

	if len(snapshot.JWKS) != 0 {
		t.Fatalf("len(JWKS) = %d, want %d", len(snapshot.JWKS), 0)
	}
}

func TestRegistryRegisterReturnsErrorWhenAlgorithmUnsupported(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		JWTAlgorithm: "ES256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWKS:         []byte(`{"keys":[]}`),
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}
