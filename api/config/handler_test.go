package configapi

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authconfig "sidecargo/internal/auth/config"
	routeconfig "sidecargo/internal/route/config"

	"github.com/gin-gonic/gin"
)

func TestNewHandlerReturnsErrorWhenRegistrarNil(t *testing.T) {
	_, err := NewHandler(nil)
	if err == nil {
		t.Fatal("NewHandler returned nil error")
	}

	if !errors.Is(err, ErrNilRegisterer) {
		t.Fatalf("error = %v, want ErrNilRegisterer", err)
	}
}

func TestHandlerPutSnapshotRegistersJWKSAndRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey := generateRSAKey(t)
	jwksPayload := mustJWKSJSON("key-1", &privateKey.PublicKey)

	authRegistry := authconfig.NewRegistry()
	routingRegistry := routeconfig.NewRegistry()
	registerer, err := NewRegisterer(authRegistry, routingRegistry)
	if err != nil {
		t.Fatalf("NewRegisterer returned error: %v", err)
	}

	handler, err := NewHandler(registerer)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		http.MethodPut,
		DefaultRoute,
		strings.NewReader(`{"auth":{"jwt_algorithm":"RS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwks":`+jwksPayload+`},"routing":{"route_service_header":"X-Wintergate-Service","route_upstream_request_timeout":"2s","routes":[{"path":"/orders","service":"order-service"}]}}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}

	publicKey, err := authRegistry.PublicKey("key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error: %v", err)
	}

	if !equalPublicKeys(publicKey, &privateKey.PublicKey) {
		t.Fatal("publicKey does not match the registered key")
	}

	authRuntimeConfig, authConfigFound := authRegistry.Snapshot()
	if !authConfigFound {
		t.Fatal("Snapshot did not return auth config")
	}

	if authRuntimeConfig.JWTIssuer != "auth-service" {
		t.Fatalf("JWTIssuer = %q, want %q", authRuntimeConfig.JWTIssuer, "auth-service")
	}

	service, found := routingRegistry.Service("/orders")
	if !found {
		t.Fatal("Service did not find /orders")
	}

	if service != "order-service" {
		t.Fatalf("service = %q, want %q", service, "order-service")
	}

	routingRuntimeConfig, routingConfigFound := routingRegistry.Snapshot()
	if !routingConfigFound {
		t.Fatal("Snapshot did not return routing config")
	}

	if routingRuntimeConfig.RouteServiceHeader != "X-Wintergate-Service" {
		t.Fatalf("RouteServiceHeader = %q, want %q", routingRuntimeConfig.RouteServiceHeader, "X-Wintergate-Service")
	}

	if routingRuntimeConfig.RouteUpstreamRequestTimeout.String() != "2s" {
		t.Fatalf("RouteUpstreamRequestTimeout = %s, want %s", routingRuntimeConfig.RouteUpstreamRequestTimeout, "2s")
	}
}

func TestHandlerPutSnapshotReturnsBadRequestWhenPayloadInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registerer, err := NewRegisterer(authconfig.NewRegistry(), routeconfig.NewRegistry())
	if err != nil {
		t.Fatalf("NewRegisterer returned error: %v", err)
	}

	handler, err := NewHandler(registerer)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodPut, DefaultRoute, strings.NewReader(`{`))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
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

func equalPublicKeys(left *rsa.PublicKey, right *rsa.PublicKey) bool {
	if left == nil || right == nil {
		return false
	}

	return left.E == right.E && left.N.Cmp(right.N) == 0
}
