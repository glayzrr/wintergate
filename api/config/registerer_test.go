package configapi

import (
	"bytes"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authconfig "wintergate/internal/auth/config"
	routeconfig "wintergate/internal/route/config"

	"github.com/gin-gonic/gin"
)

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
				{Path: " /orders ", Service: " order-service "},
				{Path: "/payments", Service: "payment-service"},
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

	routingRuntimeConfig, routingConfigFound := registerer.routingRegistry.Snapshot()
	if !routingConfigFound {
		t.Fatal("Snapshot did not return routing config")
	}

	if routingRuntimeConfig.RouteServiceHeader != "X-Wintergate-Service" {
		t.Fatalf("RouteServiceHeader = %q, want %q", routingRuntimeConfig.RouteServiceHeader, "X-Wintergate-Service")
	}

	if len(routingRuntimeConfig.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want %d", len(routingRuntimeConfig.Entries), 2)
	}

	service, found := registerer.routingRegistry.Service("/orders")
	if !found {
		t.Fatal("Service did not find /orders")
	}

	if service != "order-service" {
		t.Fatalf("service = %q, want %q", service, "order-service")
	}
}

func TestRegisterReturnsErrorWhenAuthSectionMissing(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Snapshot{
		Routing: &RoutingSection{
			RouteServiceHeader:          "X-Wintergate-Service",
			RouteUpstreamRequestTimeout: "2s",
			Routes: []Route{
				{Path: "/orders", Service: "order-service"},
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

func TestRegisterReturnsErrorWhenAuthRegistryRejectsSnapshot(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Snapshot{
		Auth: &AuthSection{
			JWTAlgorithm: "HS256",
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
				{Path: "/orders", Service: "order-service"},
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
				{Path: "/orders", Service: "order-service"},
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
		strings.NewReader(`{"auth":{"jwt_algorithm":"HS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwks":{"keys":[{"kid":"key-1","kty":"RSA","alg":"RS256","use":"sig","n":"AQAB","e":"AQAB"}]}},"routing":{"route_service_header":"X-Wintergate-Service","route_upstream_request_timeout":"2s","routes":[{"path":"/orders","service":"order-service"}]}}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
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
			{Path: "/orders", Service: "order-service"},
		},
	}
}
