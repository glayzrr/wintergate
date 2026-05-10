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
	internalconfig "wintergate/internal/config"
)

func TestNewDecoderInitializesProvider(t *testing.T) {
	decoder := NewDecoder(authconfig.NewStore())
	if decoder.provider == nil {
		t.Fatal("decoder.provider is nil")
	}
}

func TestDecoderReplaceProviderReturnsErrorWhenProviderNil(t *testing.T) {
	decoder := NewDecoder(authconfig.NewStore())

	err := decoder.ReplaceProvider(nil)
	if err == nil {
		t.Fatal("ReplaceProvider returned nil error")
	}

	if !errors.Is(err, ErrNilProvider) {
		t.Fatalf("error = %v, want ErrNilProvider", err)
	}
}

func TestDecoderReplaceProviderStoresProvider(t *testing.T) {
	decoder := NewDecoder(authconfig.NewStore())
	replacement := authconfig.NewStore()

	err := decoder.ReplaceProvider(replacement)
	if err != nil {
		t.Fatalf("ReplaceProvider returned error: %v", err)
	}

	if decoder.provider != replacement {
		t.Fatal("decoder.provider did not use the replacement provider")
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

func TestBearerTokenReturnsErrorWhenHeaderInvalid(t *testing.T) {
	_, err := BearerToken("Basic abc.def.ghi")
	if err == nil {
		t.Fatal("BearerToken returned nil error")
	}

	if !errors.Is(err, ErrInvalidAuthorizationHeader) {
		t.Fatalf("error = %v, want ErrInvalidAuthorizationHeader", err)
	}
}

func TestBearerTokenReturnsErrorWhenTokenMissing(t *testing.T) {
	_, err := BearerToken("Bearer")
	if err == nil {
		t.Fatal("BearerToken returned nil error")
	}

	if !errors.Is(err, ErrInvalidAuthorizationHeader) {
		t.Fatalf("error = %v, want ErrInvalidAuthorizationHeader", err)
	}
}

func TestDecodeReturnsErrorWhenConfigUnavailable(t *testing.T) {
	decoder := NewDecoder(authconfig.NewStore())

	_, err := decoder.DecodeFor("order-service", mustHS256Token(t, []byte("shared-secret"), map[string]any{
		"aud": "wintergate",
		"exp": time.Now().Add(time.Minute).Unix(),
		"iss": "auth-service",
	}))
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrConfigUnavailable) {
		t.Fatalf("error = %v, want ErrConfigUnavailable", err)
	}
}

func TestDecodeReturnsClaimsForHS256Token(t *testing.T) {
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})

	decoder := NewDecoder(authconfig.NewStore())
	if err := decoder.ReplaceProvider(registry); err != nil {
		t.Fatalf("ReplaceProvider returned error: %v", err)
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
		"roles": []string{
			"ADMIN",
			"OPS",
		},
		"sub": "user-1",
	})

	claims, err := decoder.DecodeFor("order-service", token)
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

	if len(claims.Roles) != 2 || claims.Roles[0] != "ADMIN" || claims.Roles[1] != "OPS" {
		t.Fatalf("claims.Roles = %#v, want [ADMIN OPS]", claims.Roles)
	}
}

func TestDecodeReturnsCustomClaimsFromGeneratedHS256Token(t *testing.T) {
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})

	decoder := NewDecoder(authconfig.NewStore())
	if err := decoder.ReplaceProvider(registry); err != nil {
		t.Fatalf("ReplaceProvider returned error: %v", err)
	}

	currentTime := time.Unix(1_700_000_050, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustHS256Token(t, []byte("shared-secret"), map[string]any{
		"aud":   "wintergate",
		"exp":   currentTime.Add(time.Minute).Unix(),
		"iat":   currentTime.Unix(),
		"iss":   "auth-service",
		"scope": "orders:read",
		"sub":   "user-1",
	})

	claims, err := decoder.DecodeFor("order-service", token)
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

	scope, ok := claims.Raw["scope"].(string)
	if !ok || scope != "orders:read" {
		t.Fatalf("claims.Raw[scope] = %#v, want %q", claims.Raw["scope"], "orders:read")
	}
}

func TestDecodeReturnsClaimsForRS256Token(t *testing.T) {
	privateKey := generatePrivateKey(t)
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "RS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWKS:         []byte(mustJWKSJSON("key-1", &privateKey.PublicKey)),
	})

	decoder := NewDecoder(authconfig.NewStore())
	if err := decoder.ReplaceProvider(registry); err != nil {
		t.Fatalf("ReplaceProvider returned error: %v", err)
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

	claims, err := decoder.DecodeFor("order-service", token)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}

	if claims.Subject != "service-a" {
		t.Fatalf("claims.Subject = %q, want %q", claims.Subject, "service-a")
	}
}

func TestDecodeReturnsErrorWhenSignatureInvalid(t *testing.T) {
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})

	decoder := NewDecoder(authconfig.NewStore())
	if err := decoder.ReplaceProvider(registry); err != nil {
		t.Fatalf("ReplaceProvider returned error: %v", err)
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

	_, err := decoder.DecodeFor("order-service", token)
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("error = %v, want ErrInvalidSignature", err)
	}
}

func TestDecodeReturnsErrorWhenTokenExpired(t *testing.T) {
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Second,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})

	decoder := NewDecoder(authconfig.NewStore())
	if err := decoder.ReplaceProvider(registry); err != nil {
		t.Fatalf("ReplaceProvider returned error: %v", err)
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

	_, err := decoder.DecodeFor("order-service", token)
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("error = %v, want ErrTokenExpired", err)
	}
}

func TestDecodeReturnsErrorWhenIssuerInvalid(t *testing.T) {
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})

	decoder := NewDecoder(registry)
	currentTime := time.Unix(1_700_000_350, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustHS256Token(t, []byte("shared-secret"), map[string]any{
		"aud": "wintergate",
		"exp": currentTime.Add(time.Minute).Unix(),
		"iss": "other-service",
	})

	_, err := decoder.DecodeFor("order-service", token)
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrInvalidIssuer) {
		t.Fatalf("error = %v, want ErrInvalidIssuer", err)
	}
}

func TestDecodeReturnsErrorWhenAudienceInvalid(t *testing.T) {
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})

	decoder := NewDecoder(registry)
	currentTime := time.Unix(1_700_000_360, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustHS256Token(t, []byte("shared-secret"), map[string]any{
		"aud": "other-service",
		"exp": currentTime.Add(time.Minute).Unix(),
		"iss": "auth-service",
	})

	_, err := decoder.DecodeFor("order-service", token)
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrInvalidAudience) {
		t.Fatalf("error = %v, want ErrInvalidAudience", err)
	}
}

func TestDecodeReturnsErrorWhenTokenNotYetValid(t *testing.T) {
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Second,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})

	decoder := NewDecoder(registry)
	currentTime := time.Unix(1_700_000_370, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustHS256Token(t, []byte("shared-secret"), map[string]any{
		"aud": "wintergate",
		"exp": currentTime.Add(time.Minute).Unix(),
		"iss": "auth-service",
		"nbf": currentTime.Add(time.Minute).Unix(),
	})

	_, err := decoder.DecodeFor("order-service", token)
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrTokenNotYetValid) {
		t.Fatalf("error = %v, want ErrTokenNotYetValid", err)
	}
}

func TestDecodeReturnsErrorWhenAlgorithmMismatch(t *testing.T) {
	registry := mustAuthStore(t, authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})

	decoder := NewDecoder(registry)
	currentTime := time.Unix(1_700_000_380, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	token := mustSignedToken(t, map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	}, map[string]any{
		"aud": "wintergate",
		"exp": currentTime.Add(time.Minute).Unix(),
		"iss": "auth-service",
	}, func(signingInput string) []byte {
		mac := hmac.New(sha256.New, []byte("shared-secret"))
		if _, err := mac.Write([]byte(signingInput)); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}

		return mac.Sum(nil)
	})

	_, err := decoder.DecodeFor("order-service", token)
	if err == nil {
		t.Fatal("Decode returned nil error")
	}

	if !errors.Is(err, ErrUnsupportedAlgorithm) {
		t.Fatalf("error = %v, want ErrUnsupportedAlgorithm", err)
	}
}

func TestDecodeTokenHeaderReturnsErrorWhenAlgorithmMissing(t *testing.T) {
	_, err := decodeTokenHeader([]byte(`{}`))
	if err == nil {
		t.Fatal("decodeTokenHeader returned nil error")
	}

	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestSplitTokenReturnsErrorWhenFormatInvalid(t *testing.T) {
	_, _, _, err := splitToken("only.two")
	if err == nil {
		t.Fatal("splitToken returned nil error")
	}

	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRS256SignatureReturnsErrorWhenInvalid(t *testing.T) {
	privateKey := generatePrivateKey(t)

	err := verifyRS256Signature(&privateKey.PublicKey, "header.payload", []byte("bad-signature"))
	if err == nil {
		t.Fatal("verifyRS256Signature returned nil error")
	}

	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("error = %v, want ErrInvalidSignature", err)
	}
}

func TestValidateClaimsReturnsErrorWhenIssuedAtInFuture(t *testing.T) {
	decoder := NewDecoder(authconfig.NewStore())
	currentTime := time.Unix(1_700_000_390, 0).UTC()
	decoder.now = func() time.Time {
		return currentTime
	}

	err := decoder.validateClaims(authconfig.Config{
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Second,
		JWTIssuer:    "auth-service",
	}, decodedClaims{
		Claims: Claims{
			Audience:  []string{"wintergate"},
			ExpiresAt: currentTime.Add(time.Minute),
			IssuedAt:  currentTime.Add(time.Minute),
			Issuer:    "auth-service",
		},
		hasExpiresAt: true,
		hasIssuedAt:  true,
	})
	if err == nil {
		t.Fatal("validateClaims returned nil error")
	}

	if !errors.Is(err, ErrTokenNotYetValid) {
		t.Fatalf("error = %v, want ErrTokenNotYetValid", err)
	}
}

func TestAudienceClaimUnmarshalJSON(t *testing.T) {
	var claim audienceClaim

	if err := claim.UnmarshalJSON([]byte(`"wintergate"`)); err != nil {
		t.Fatalf("UnmarshalJSON returned error for string: %v", err)
	}

	if len(claim) != 1 || claim[0] != "wintergate" {
		t.Fatalf("claim = %#v, want [wintergate]", claim)
	}

	if err := claim.UnmarshalJSON([]byte(`["wintergate","orders"]`)); err != nil {
		t.Fatalf("UnmarshalJSON returned error for string array: %v", err)
	}

	if len(claim) != 2 || claim[1] != "orders" {
		t.Fatalf("claim = %#v, want [wintergate orders]", claim)
	}

	if err := claim.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Fatalf("UnmarshalJSON returned error for null: %v", err)
	}

	if claim != nil {
		t.Fatalf("claim = %#v, want nil", claim)
	}

	if err := claim.UnmarshalJSON([]byte(`123`)); err == nil {
		t.Fatal("UnmarshalJSON returned nil error for invalid payload")
	}
}

func TestNumericDateUnmarshalJSON(t *testing.T) {
	var numeric numericDate

	if err := numeric.UnmarshalJSON([]byte(`1700000000.5`)); err != nil {
		t.Fatalf("UnmarshalJSON returned error: %v", err)
	}

	if !numeric.set {
		t.Fatal("numeric.set = false, want true")
	}

	if !numeric.time.Equal(time.Unix(1_700_000_000, int64(500*time.Millisecond)).UTC()) {
		t.Fatalf("numeric.time = %s, want %s", numeric.time, time.Unix(1_700_000_000, int64(500*time.Millisecond)).UTC())
	}

	if err := numeric.UnmarshalJSON([]byte(`null`)); err != nil {
		t.Fatalf("UnmarshalJSON returned error for null: %v", err)
	}

	if numeric.set {
		t.Fatal("numeric.set = true, want false")
	}

	if err := numeric.UnmarshalJSON([]byte(`"bad"`)); err == nil {
		t.Fatal("UnmarshalJSON returned nil error for invalid payload")
	}
}

func mustAuthStore(t *testing.T, cfg authconfig.Config) *authconfig.Store {
	t.Helper()

	store := authconfig.NewStore()
	settings := internalconfig.Settings{
		ServiceName: "order-service",
		Global: &internalconfig.GlobalSettings{
			Auth: &internalconfig.AuthSettings{
				JWTAlgorithm: cfg.JWTAlgorithm,
				JWTAudience:  cfg.JWTAudience,
				JWTClockSkew: cfg.JWTClockSkew.String(),
				JWTIssuer:    cfg.JWTIssuer,
				JWTSecret:    string(cfg.JWTSecret),
				JWKS:         append([]byte(nil), cfg.JWKS...),
			},
		},
	}

	if err := store.Apply(settings, "", ""); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}

	return store
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
