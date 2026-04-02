package configapi

import (
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	responseapi "wintergate/api/response"
	authconfig "wintergate/internal/auth/config"
	routeconfig "wintergate/internal/route/config"

	"github.com/gin-gonic/gin"
)

const testClientIP = "192.0.2.10"

func TestNewRegistererInitializesRegistries(t *testing.T) {
	registerer := NewRegisterer()

	if registerer.authRegistry == nil {
		t.Fatal("authRegistry is nil")
	}

	if registerer.routingRegistry == nil {
		t.Fatal("routingRegistry is nil")
	}
}

func TestRegisterStoresSnapshotWhenValid(t *testing.T) {
	registerer := NewRegisterer()
	privateKey := generateRSAKey(t)
	authSection := validAuthSection(t, privateKey)
	originalJWKS := append([]byte(nil), authSection.JWKS...)

	err := registerer.Register(Snapshot{
		Auth: authSection,
		Routing: &RoutingSection{
			RouteServiceHeader:          " X-Wintergate-Service ",
			RouteUpstreamRequestTimeout: "2s",
			Routes: []Route{
				{Path: " /orders ", Service: " order-service ", ClientIP: testClientIP, Port: 8080},
				{Path: "/payments", Service: "payment-service", ClientIP: testClientIP, Port: 8081},
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

	routingSnapshot, routingConfigFound := registerer.routingRegistry.Snapshot()
	if !routingConfigFound {
		t.Fatal("Snapshot did not return routing info")
	}

	if len(routingSnapshot) != 2 {
		t.Fatalf("len(routingSnapshot) = %d, want %d", len(routingSnapshot), 2)
	}

	routeInfo, found := registerer.routingRegistry.Route("/orders")
	if !found {
		t.Fatal("Route did not find /orders")
	}

	if routeInfo.Service != "order-service" {
		t.Fatalf("routeInfo.Service = %q, want %q", routeInfo.Service, "order-service")
	}

	if routeInfo.ClientIP != testClientIP {
		t.Fatalf("routeInfo.ClientIP = %q, want %q", routeInfo.ClientIP, testClientIP)
	}

	if routeInfo.Port != 8080 {
		t.Fatalf("routeInfo.Port = %d, want %d", routeInfo.Port, 8080)
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
		Routing: validRoutingSection(),
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
		Routing: &RoutingSection{
			RouteServiceHeader:          "X-Wintergate-Service",
			RouteUpstreamRequestTimeout: "2s",
			Routes: []Route{
				{Path: "/orders", Service: "order-service", ClientIP: testClientIP, Port: 8080},
			},
		},
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
		Routing: validRoutingSection(),
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
		Routing: validRoutingSection(),
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
		Routing: validRoutingSection(),
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
		Routing: validRoutingSection(),
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, authconfig.ErrInvalidConfig) {
		t.Fatalf("error = %v, want authconfig.ErrInvalidConfig", err)
	}
}

func TestRegisterReturnsErrorWhenRoutingSectionMissing(t *testing.T) {
	registerer := NewRegisterer()
	privateKey := generateRSAKey(t)

	err := registerer.Register(Snapshot{
		Auth: validAuthSection(t, privateKey),
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestRegisterReturnsErrorWhenRoutingTimeoutInvalid(t *testing.T) {
	registerer := NewRegisterer()
	privateKey := generateRSAKey(t)

	err := registerer.Register(Snapshot{
		Auth: validAuthSection(t, privateKey),
		Routing: &RoutingSection{
			RouteServiceHeader:          "X-Wintergate-Service",
			RouteUpstreamRequestTimeout: "bad",
			Routes: []Route{
				{Path: "/orders", Service: "order-service", ClientIP: testClientIP, Port: 8080},
			},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !strings.Contains(err.Error(), "parse route upstream timeout") {
		t.Fatalf("error = %q, want route upstream timeout context", err.Error())
	}
}

func TestRegisterReturnsErrorWhenRoutesMissing(t *testing.T) {
	registerer := NewRegisterer()
	privateKey := generateRSAKey(t)

	err := registerer.Register(Snapshot{
		Auth: validAuthSection(t, privateKey),
		Routing: &RoutingSection{
			RouteServiceHeader:          "X-Wintergate-Service",
			RouteUpstreamRequestTimeout: "2s",
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestRegisterReturnsErrorWhenRoutingRegistryRejectsSnapshot(t *testing.T) {
	registerer := NewRegisterer()
	privateKey := generateRSAKey(t)

	err := registerer.Register(Snapshot{
		Auth: validAuthSection(t, privateKey),
		Routing: &RoutingSection{
			RouteServiceHeader:          "",
			RouteUpstreamRequestTimeout: "2s",
			Routes: []Route{
				{Path: "/orders", Service: "order-service", ClientIP: testClientIP, Port: 8080},
			},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, routeconfig.ErrInvalidConfig) {
		t.Fatalf("error = %v, want routeconfig.ErrInvalidConfig", err)
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
		strings.NewReader(`{"auth":{"jwt_algorithm":"HS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwks":{"keys":[{"kid":"key-1","kty":"RSA","alg":"RS256","use":"sig","n":"AQAB","e":"AQAB"}]}},"routing":{"route_service_header":"X-Wintergate-Service","route_upstream_request_timeout":"2s","routes":[{"path":"/orders","service":"order-service","port":8080}]}}`),
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

func validAuthSection(t *testing.T, privateKey *rsa.PrivateKey) *AuthSection {
	t.Helper()

	return &AuthSection{
		JWTAlgorithm: "RS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: "1m",
		JWTIssuer:    "auth-service",
		JWKS:         []byte(mustJWKSJSON("key-1", &privateKey.PublicKey)),
	}
}

func validRoutingSection() *RoutingSection {
	return &RoutingSection{
		RouteServiceHeader:          "X-Wintergate-Service",
		RouteUpstreamRequestTimeout: "2s",
		Routes: []Route{
			{Path: "/orders", Service: "order-service", ClientIP: testClientIP, Port: 8080},
		},
	}
}
