package configapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	responseapi "wintergate/api/response"
	authconfig "wintergate/internal/auth/config"

	"github.com/gin-gonic/gin"
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

func TestRegisterStoresAuthConfigWhenSnapshotValid(t *testing.T) {
	registerer := NewRegisterer()
	authSection := validAuthSection(t)
	originalJWKS := append([]byte(nil), authSection.JWKS...)

	err := registerer.Register(Snapshot{
		Auth:   authSection,
		Routes: validRoutesSection(),
		RateLimit: []RateLimitEndpoint{
			{
				Endpoint: Endpoint{
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

	authSection.JWKS[0] = 'x'

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

	routeInfos, found := registerer.routeRegistry.RouteInfos("order-service")
	if !found {
		t.Fatal("RouteInfos did not return registered route info")
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

	err := registerer.Register(Snapshot{
		Auth: &AuthSection{
			JWTAlgorithm: "HS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
			JWTSecret:    " shared-secret ",
		},
		Routes: validRoutesSection(),
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

func TestRegisterReturnsErrorWhenAuthSectionMissing(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Snapshot{
		Routes: validRoutesSection(),
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestRegisterReturnsErrorWhenAuthClockSkewInvalid(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Snapshot{
		Auth: &AuthSection{
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

	if !strings.Contains(err.Error(), "parse auth clock skew") {
		t.Fatalf("error = %q, want auth clock skew context", err.Error())
	}
}

func TestRegisterReturnsErrorWhenAuthJWKSMissing(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Snapshot{
		Auth: &AuthSection{
			JWTAlgorithm: "RS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestRegisterReturnsErrorWhenAuthSecretMissingForHS256(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Snapshot{
		Auth: &AuthSection{
			JWTAlgorithm: "HS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestRegisterReturnsErrorWhenAuthRegistryRejectsSnapshot(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Snapshot{
		Auth: &AuthSection{
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

func TestRegisterReturnsErrorWhenRouteRegistryRejectsSnapshot(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Snapshot{
		Auth: &AuthSection{
			JWTAlgorithm: "HS256",
			JWTAudience:  "wintergate",
			JWTClockSkew: "1m",
			JWTIssuer:    "auth-service",
			JWTSecret:    "shared-secret",
		},
		Routes: &RoutesSection{
			Protected: []ProtectedEndpoint{
				{
					Endpoint: Endpoint{
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

	cfg := registerer.routeRuntimeConfig(&RoutesSection{
		Protected: []ProtectedEndpoint{
			{
				Endpoint: Endpoint{
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

	cfg = registerer.routeRuntimeConfig(&RoutesSection{
		Protected: []ProtectedEndpoint{
			{
				Endpoint: Endpoint{
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

func TestHandlerPutSnapshotReturnsBadRequestWhenRegisterFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registerer := NewRegisterer()
	handler, err := NewHandler(registerer)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		http.MethodPost,
		DefaultRoute,
		strings.NewReader(`{"auth":{"jwt_algorithm":"HS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwks":{"keys":[{"kid":"key-1","kty":"RSA","alg":"RS256","use":"sig","n":"AQAB","e":"AQAB"}]}},"routes":{"protected":[{"path":"/api/order","method":"POST","service":"order-service","roles":["ADMIN","OPS"],"time_window":{"start":"09:00","end":"18:00","timezone":"Asia/Seoul"}}]},"rate_limit":[{"path":"/api/order","method":"POST","service":"order-service","roles":["anyone"],"duration":"1m","limit":10}]}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var response responseapi.APIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}

	if response.Message != responseRegisterFailed {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseRegisterFailed)
	}
}

func validAuthSection(t *testing.T) *AuthSection {
	t.Helper()

	privateKey := generateRSAKey(t)

	return &AuthSection{
		JWTAlgorithm: "RS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: "1m",
		JWTIssuer:    "auth-service",
		JWKS:         []byte(mustJWKSJSON("key-1", &privateKey.PublicKey)),
	}
}

func validRoutesSection() *RoutesSection {
	return &RoutesSection{
		Protected: []ProtectedEndpoint{
			{
				Endpoint: Endpoint{
					Path:    "/api/order",
					Method:  http.MethodPost,
					Service: "order-service",
				},
				Roles: []string{"ADMIN", "OPS"},
				TimeWindow: &TimeWindow{
					Start:    "09:00",
					End:      "18:00",
					Timezone: "Asia/Seoul",
				},
			},
		},
	}
}
