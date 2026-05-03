package configapi

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	responseapi "wintergate/api/response"
	internalconfig "wintergate/internal/config"

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

func TestHandlerEnrollConfigRegistersJWKSWhenPayloadValid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	privateKey := generateRSAKey(t)
	jwksPayload := mustJWKSJSON("key-1", &privateKey.PublicKey)

	registerer := internalconfig.NewRegisterer()

	handler, err := NewHandler(registerer)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		http.MethodPost,
		ConfigRoute,
		strings.NewReader(`{"auth":{"jwt_algorithm":"RS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwks":`+jwksPayload+`},"routes":{"protected":[{"path":"/api/order","method":"POST","service":"order-service","roles":["ADMIN","OPS"],"time_window":{"start":"09:00","end":"18:00","timezone":"Asia/Seoul"}}]},"rate_limit":[{"path":"/api/order","method":"POST","service":"order-service","roles":["anyone"],"duration":"1m","limit":10}]}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, true)
	}

	if response.Message != responseRegisterSuccess {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseRegisterSuccess)
	}

	publicKey, err := registerer.AuthRegistry().PublicKey("key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error: %v", err)
	}

	if !equalPublicKeys(publicKey, &privateKey.PublicKey) {
		t.Fatal("publicKey does not match the registered key")
	}

	authRuntimeConfig, authConfigFound := registerer.AuthRegistry().Snapshot()
	if !authConfigFound {
		t.Fatal("Snapshot did not return auth config")
	}

	if authRuntimeConfig.JWTIssuer != "auth-service" {
		t.Fatalf("JWTIssuer = %q, want %q", authRuntimeConfig.JWTIssuer, "auth-service")
	}
}

func TestHandlerEnrollConfigReturnsBadRequestWhenPayloadInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registerer := internalconfig.NewRegisterer()
	handler, err := NewHandler(registerer)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodPost, ConfigRoute, strings.NewReader(`{`))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}

	if response.Message != responseBindFailed {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseBindFailed)
	}
}

func TestHandlerEnrollConfigReturnsBadRequestWhenRegisterFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registerer := internalconfig.NewRegisterer()
	handler, err := NewHandler(registerer)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(
		http.MethodPost,
		ConfigRoute,
		strings.NewReader(`{"auth":{"jwt_algorithm":"HS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwks":{"keys":[{"kid":"key-1","kty":"RSA","alg":"RS256","use":"sig","n":"AQAB","e":"AQAB"}]}},"routes":{"protected":[{"path":"/api/order","method":"POST","service":"order-service","roles":["ADMIN","OPS"],"time_window":{"start":"09:00","end":"18:00","timezone":"Asia/Seoul"}}]},"rate_limit":[{"path":"/api/order","method":"POST","service":"order-service","roles":["anyone"],"duration":"1m","limit":10}]}`),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}

	if response.Message != responseRegisterFailed {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseRegisterFailed)
	}
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) responseapi.APIResponse {
	t.Helper()

	var response responseapi.APIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	return response
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
