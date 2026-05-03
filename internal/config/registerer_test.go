package config

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"math/big"
	"net/http"
	"testing"

	authconfig "wintergate/internal/auth/config"
)

func TestNewRegistererInitializesRegistry(t *testing.T) {
	registerer := NewRegisterer()

	if registerer.authRegistry == nil {
		t.Fatal("authRegistry is nil")
	}

	if registerer.routeRegistry == nil {
		t.Fatal("routeRegistry is nil")
	}
}

func TestRegisterStoresAuthConfigWhenSettingsValid(t *testing.T) {
	registerer := NewRegisterer()
	authSettings := validAuthSettings(t)
	originalJWKS := append([]byte(nil), authSettings.JWKS...)

	err := registerer.Register(Settings{
		Auth:   authSettings,
		Routes: validRouteSettings(),
		RateLimit: []RateLimitSettings{
			{
				Route: Route{
					Path:    "/api/order",
					Method:  http.MethodPost,
					Service: "order-service",
				},
				Roles:    []string{"anyone"},
				Duration: "1m",
				Limit:    10,
			},
		},
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	authSettings.JWKS[0] = 'x'

	authRuntimeConfig, authConfigFound := registerer.authRegistry.Snapshot()
	if !authConfigFound {
		t.Fatal("Snapshot did not return auth config")
	}

	if !bytes.Equal(authRuntimeConfig.JWKS, originalJWKS) {
		t.Fatal("JWKS was not copied during registration")
	}

	if authRuntimeConfig.JWTIssuer != "auth-service" {
		t.Fatalf("JWTIssuer = %q, want %q", authRuntimeConfig.JWTIssuer, "auth-service")
	}

	routeInfos, err := registerer.routeRegistry.RouteInfos("order-service")
	if err != nil {
		t.Fatalf("RouteInfos returned error: %v", err)
	}

	if len(routeInfos) != 1 {
		t.Fatalf("len(routeInfos) = %d, want %d", len(routeInfos), 1)
	}

	if routeInfos[0].Path != "/api/order" {
		t.Fatalf("routeInfos[0].Path = %q, want %q", routeInfos[0].Path, "/api/order")
	}
}

func TestRegisterStoresHS256SecretWhenValid(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Auth: &AuthSettings{
			JWTAlgorithm: "HS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
			JWTSecret:    " shared-secret ",
		},
		Routes: validRouteSettings(),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	authRuntimeConfig, authConfigFound := registerer.authRegistry.Snapshot()
	if !authConfigFound {
		t.Fatal("Snapshot did not return auth config")
	}

	if string(authRuntimeConfig.JWTSecret) != "shared-secret" {
		t.Fatalf("JWTSecret = %q, want %q", string(authRuntimeConfig.JWTSecret), "shared-secret")
	}

	if len(authRuntimeConfig.JWKS) != 0 {
		t.Fatalf("len(JWKS) = %d, want %d", len(authRuntimeConfig.JWKS), 0)
	}
}

func TestRegisterReturnsErrorWhenAuthSettingsMissing(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Routes: validRouteSettings(),
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
}

func TestRegisterReturnsErrorWhenAuthClockSkewInvalid(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Auth: &AuthSettings{
			JWTAlgorithm: "RS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "bad",
			JWTIssuer:    "auth-service",
			JWKS:         []byte(`{"keys":[{"kid":"key-1","kty":"RSA","alg":"RS256","use":"sig","n":"AQAB","e":"AQAB"}]}`),
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
}

func TestRegisterReturnsErrorWhenAuthJWKSMissing(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Auth: &AuthSettings{
			JWTAlgorithm: "RS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
}

func TestRegisterReturnsErrorWhenAuthSecretMissingForHS256(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Auth: &AuthSettings{
			JWTAlgorithm: "HS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
}

func TestRegisterReturnsErrorWhenAuthRegistryRejectsSettings(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Auth: &AuthSettings{
			JWTAlgorithm: "ES256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
			JWKS:         []byte(`{"keys":[{"kid":"key-1","kty":"RSA","alg":"RS256","use":"sig","n":"AQAB","e":"AQAB"}]}`),
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, authconfig.ErrInvalidConfig) {
		t.Fatalf("error = %v, want authconfig.ErrInvalidConfig", err)
	}
}

func TestRegisterReturnsErrorWhenRouteRegistryRejectsSettings(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Auth: &AuthSettings{
			JWTAlgorithm: "HS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
			JWTSecret:    "shared-secret",
		},
		Routes: &RouteSettings{
			Protected: []ProtectedRoute{
				{
					Route: Route{
						Method:  http.MethodPost,
						Service: "order-service",
					},
					Roles: []string{"ADMIN"},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}
}

func TestRouteRuntimeConfigReturnsEntries(t *testing.T) {
	registerer := NewRegisterer()

	cfg := registerer.registerRouteConfig(&RouteSettings{
		Protected: []ProtectedRoute{
			{
				Route: Route{
					Path:    "/api/order",
					Method:  http.MethodPost,
					Service: "order-service",
				},
				Roles: []string{"ADMIN", "OPS"},
			},
		},
	})

	if len(cfg.Entries) != 1 {
		t.Fatalf("len(cfg.Entries) = %d, want %d", len(cfg.Entries), 1)
	}

	if cfg.Entries[0].Service != "order-service" {
		t.Fatalf("cfg.Entries[0].Service = %q, want %q", cfg.Entries[0].Service, "order-service")
	}

	cfg.Entries[0].Roles[0] = "GUEST"

	cfg = registerer.registerRouteConfig(&RouteSettings{
		Protected: []ProtectedRoute{
			{
				Route: Route{
					Path:    "/api/order",
					Method:  http.MethodPost,
					Service: "order-service",
				},
				Roles: []string{"ADMIN", "OPS"},
			},
		},
	})

	if cfg.Entries[0].Roles[0] != "ADMIN" {
		t.Fatalf("cfg.Entries[0].Roles[0] = %q, want %q", cfg.Entries[0].Roles[0], "ADMIN")
	}
}

func validAuthSettings(t *testing.T) *AuthSettings {
	t.Helper()

	privateKey := generateRSAKey(t)

	return &AuthSettings{
		JWTAlgorithm: "RS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: "1m",
		JWTIssuer:    "auth-service",
		JWKS:         []byte(mustJWKSJSON("key-1", &privateKey.PublicKey)),
	}
}

func validRouteSettings() *RouteSettings {
	return &RouteSettings{
		Protected: []ProtectedRoute{
			{
				Route: Route{
					Path:    "/api/order",
					Method:  http.MethodPost,
					Service: "order-service",
				},
				Roles: []string{"ADMIN", "OPS"},
				AccessWindow: &AccessWindow{
					Start:    "09:00",
					End:      "18:00",
					Timezone: "Asia/Seoul",
				},
			},
		},
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

func mustJWKSJSON(keyID string, publicKey *rsa.PublicKey) string {
	return `{"keys":[{"kid":"` + keyID + `","kty":"RSA","alg":"RS256","use":"sig","n":"` +
		base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()) + `","e":"` +
		base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()) + `"}]}`
}
