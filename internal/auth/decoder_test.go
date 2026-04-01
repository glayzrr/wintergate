package auth

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"testing"
	"time"

	authconfig "wintergate/internal/auth/config"
)

func TestNewDecoderInitializesRegistry(t *testing.T) {
	decoder := NewDecoder()
	if decoder.registry == nil {
		t.Fatal("decoder.registry is nil")
	}
}

func TestDecoderUseRegistryReturnsErrorWhenRegistryNil(t *testing.T) {
	decoder := NewDecoder()

	err := decoder.UseRegistry(nil)
	if err == nil {
		t.Fatal("UseRegistry returned nil error")
	}

	if !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("error = %v, want ErrNilRegistry", err)
	}
}

func TestBearerTokenReturnsToken(t *testing.T) {
	token, err := BearerToken("Bearer abc.def.ghi")
	if err != nil {
		t.Fatalf("BearerToken returned error: %v", err)
	}

	if token != "abc.def.ghi" {
		t.Fatalf("token = %q, want %q", token, "abc.def.ghi")
	}
}

func TestDecodeReturnsClaimsForHS256Token(t *testing.T) {
	registry := authconfig.NewRegistry()
	err := registry.Register(authconfig.RuntimeConfig{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decoder := NewDecoder()
	if err := decoder.UseRegistry(registry); err != nil {
		t.Fatalf("UseRegistry returned error: %v", err)
	}

	currentTime := time.Unix(1_700_000_000, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustHS256Token(t, []byte("shared-secret"), map[string]any{
		"aud": "wintergate",
		"exp": currentTime.Add(time.Minute).Unix(),
		"iat": currentTime.Add(-time.Second).Unix(),
		"iss": "auth-service",
		"sub": "user-1",
	})

	claims, err := decoder.Decode(token)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	if claims.Subject != "user-1" {
		t.Fatalf("claims.Subject = %q, want %q", claims.Subject, "user-1")
	}

	if claims.Issuer != "auth-service" {
		t.Fatalf("claims.Issuer = %q, want %q", claims.Issuer, "auth-service")
	}

	if len(claims.Audience) != 1 || claims.Audience[0] != "wintergate" {
		t.Fatalf("claims.Audience = %#v, want [wintergate]", claims.Audience)
	}
}

func TestDecodeReturnsClaimsForRS256Token(t *testing.T) {
	privateKey := generatePrivateKey(t)
	registry := authconfig.NewRegistry()
	err := registry.Register(authconfig.RuntimeConfig{
		JWTAlgorithm: "RS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWKS:         []byte(mustJWKSJSON("key-1", &privateKey.PublicKey)),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decoder := NewDecoder()
	if err := decoder.UseRegistry(registry); err != nil {
		t.Fatalf("UseRegistry returned error: %v", err)
	}

	currentTime := time.Unix(1_700_000_100, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustRS256Token(t, privateKey, "key-1", map[string]any{
		"aud": []string{"wintergate"},
		"exp": currentTime.Add(time.Minute).Unix(),
		"iss": "auth-service",
		"sub": "service-a",
	})

	claims, err := decoder.Decode(token)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	if claims.Subject != "service-a" {
		t.Fatalf("claims.Subject = %q, want %q", claims.Subject, "service-a")
	}
}

func TestDecodeReturnsErrorWhenSignatureInvalid(t *testing.T) {
	registry := authconfig.NewRegistry()
	err := registry.Register(authconfig.RuntimeConfig{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decoder := NewDecoder()
	if err := decoder.UseRegistry(registry); err != nil {
		t.Fatalf("UseRegistry returned error: %v", err)
	}

	currentTime := time.Unix(1_700_000_200, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustHS256Token(t, []byte("wrong-secret"), map[string]any{
		"aud": "wintergate",
		"exp": currentTime.Add(time.Minute).Unix(),
		"iss": "auth-service",
	})

	_, err = decoder.Decode(token)
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("error = %v, want ErrInvalidSignature", err)
	}
}

func TestDecodeReturnsErrorWhenTokenExpired(t *testing.T) {
	registry := authconfig.NewRegistry()
	err := registry.Register(authconfig.RuntimeConfig{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Second,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decoder := NewDecoder()
	if err := decoder.UseRegistry(registry); err != nil {
		t.Fatalf("UseRegistry returned error: %v", err)
	}

	currentTime := time.Unix(1_700_000_300, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustHS256Token(t, []byte("shared-secret"), map[string]any{
		"aud": "wintergate",
		"exp": currentTime.Add(-time.Minute).Unix(),
		"iss": "auth-service",
	})

	_, err = decoder.Decode(token)
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("error = %v, want ErrTokenExpired", err)
	}
}

func mustHS256Token(t *testing.T, secret []byte, claims map[string]any) string {
	t.Helper()

	header := map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}

	return mustSignedToken(t, header, claims, func(signingInput string) []byte {
		mac := hmac.New(sha256.New, secret)
		if _, err := mac.Write([]byte(signingInput)); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}

		return mac.Sum(nil)
	})
}

func mustRS256Token(t *testing.T, privateKey *rsa.PrivateKey, keyID string, claims map[string]any) string {
	t.Helper()

	header := map[string]any{
		"alg": "RS256",
		"kid": keyID,
		"typ": "JWT",
	}

	return mustSignedToken(t, header, claims, func(signingInput string) []byte {
		hashed := sha256.Sum256([]byte(signingInput))
		signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
		if err != nil {
			t.Fatalf("SignPKCS1v15 returned error: %v", err)
		}

		return signature
	})
}

func mustSignedToken(t *testing.T, header map[string]any, claims map[string]any, sign func(signingInput string) []byte) string {
	t.Helper()

	headerPayload, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("Marshal returned error for header: %v", err)
	}

	claimsPayload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal returned error for claims: %v", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerPayload) + "." + base64.RawURLEncoding.EncodeToString(claimsPayload)
	signature := sign(signingInput)

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func generatePrivateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	return privateKey
}

func mustJWKSJSON(keyID string, publicKey *rsa.PublicKey) string {
	return `{"keys":[{"kid":"` + keyID + `","kty":"RSA","alg":"RS256","use":"sig","n":"` +
		base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()) + `","e":"` +
		base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()) + `"}]}`
}
