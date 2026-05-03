package gatewayapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	configapi "wintergate/api/config"
	responseapi "wintergate/api/response"
	internalauth "wintergate/internal/auth"
	authconfig "wintergate/internal/auth/config"
	internalgateway "wintergate/internal/gateway"

	"github.com/gin-gonic/gin"
)

func TestNewHandlerUsesInjectedOrchestrator(t *testing.T) {
	orchestrator := internalgateway.NewOrchestrator()
	handler := NewHandler(orchestrator)
	if handler.orchestrator == nil {
		t.Fatal("handler.orchestrator is nil")
	}

	if handler.orchestrator != orchestrator {
		t.Fatal("handler.orchestrator did not use injected orchestrator")
	}
}

func TestHandlerReceiveReturnsGatewayIngressResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(internalgateway.NewOrchestrator())

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"hello":"world"}`))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, true)
	}

	if response.Message != responseReceiveSuccess {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseReceiveSuccess)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("response.Data type = %T, want map[string]any", response.Data)
	}

	received, ok := data["received"].(bool)
	if !ok || !received {
		t.Fatalf("data[received] = %#v, want true", data["received"])
	}

	method, ok := data["method"].(string)
	if !ok || method != http.MethodPost {
		t.Fatalf("data[method] = %#v, want %q", data["method"], http.MethodPost)
	}

	path, ok := data["path"].(string)
	if !ok || path != "/orders" {
		t.Fatalf("data[path] = %#v, want %q", data["path"], "/orders")
	}
}

func TestHandlerReceiveLeavesConfigRouteUnclaimed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(internalgateway.NewOrchestrator())

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodGet, configapi.ConfigRoute, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}

	if response.Message != responseNotFound {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseNotFound)
	}
}

func TestHandlerReceiveReturnsBadRequestWhenOrchestratorRejectsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orchestrator := internalgateway.NewOrchestrator(
		internalgatewayTaskFunc(func(_ context.Context, _ *internalgateway.State) error {
			return internalgateway.ErrInvalidRequest
		}),
	)
	handler := NewHandler(orchestrator)

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}

	if response.Message != responseReceiveFailed {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseReceiveFailed)
	}
}

func TestHandlerReceivePassesServiceHeaderToOrchestrator(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orchestrator := internalgateway.NewOrchestrator(
		internalgatewayTaskFunc(func(_ context.Context, state *internalgateway.State) error {
			if state.Request.Service != "order-service" {
				t.Fatalf("state.Request.Service = %q, want %q", state.Request.Service, "order-service")
			}

			return nil
		}),
	)
	handler := NewHandler(orchestrator)

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	request.Header.Set(requestHeaderService, "order-service")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestHandlerReceiveReturnsInternalServerErrorWhenTaskFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orchestrator := internalgateway.NewOrchestrator(
		internalgatewayTaskFunc(func(_ context.Context, _ *internalgateway.State) error {
			return errors.New("boom")
		}),
	)
	handler := NewHandler(orchestrator)

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}

	if response.Message != responseReceiveFailed {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseReceiveFailed)
	}
}

func TestHandlerReceiveReturnsUnauthorizedWhenAuthorizationHeaderInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := authconfig.NewRegistry()
	err := registry.Register(authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	authenticateTask := internalgateway.NewAuthenticateTask(internalauth.NewDecoder(registry))
	handler := NewHandler(internalgateway.NewOrchestrator(authenticateTask))

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	request.Header.Set("Authorization", "Bearer invalid.token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}

	if response.Message != responseUnauthorized {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseUnauthorized)
	}
}

func TestHandlerReceiveAcceptsValidBearerTokenWhenAuthTaskRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := authconfig.NewRegistry()
	err := registry.Register(authconfig.Config{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    []byte("shared-secret"),
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	authenticateTask := internalgateway.NewAuthenticateTask(internalauth.NewDecoder(registry))
	handler := NewHandler(internalgateway.NewOrchestrator(authenticateTask))

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	request.Header.Set("Authorization", "Bearer "+mustHS256Token(t, []byte("shared-secret")))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, true)
	}
}

func TestReceiveFailure(t *testing.T) {
	testCases := []struct {
		name       string
		err        error
		statusCode int
		message    string
	}{
		{name: "invalid request", err: internalgateway.ErrInvalidRequest, statusCode: http.StatusBadRequest, message: responseReceiveFailed},
		{name: "invalid authorization header", err: internalauth.ErrInvalidAuthorizationHeader, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "invalid audience", err: internalauth.ErrInvalidAudience, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "invalid issuer", err: internalauth.ErrInvalidIssuer, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "invalid signature", err: internalauth.ErrInvalidSignature, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "invalid token", err: internalauth.ErrInvalidToken, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "token expired", err: internalauth.ErrTokenExpired, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "token not yet valid", err: internalauth.ErrTokenNotYetValid, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "unsupported algorithm", err: internalauth.ErrUnsupportedAlgorithm, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "key not found", err: authconfig.ErrKeyNotFound, statusCode: http.StatusUnauthorized, message: responseUnauthorized},
		{name: "unknown error", err: errors.New("boom"), statusCode: http.StatusInternalServerError, message: responseReceiveFailed},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			statusCode, message := receiveFailure(testCase.err)
			if statusCode != testCase.statusCode {
				t.Fatalf("statusCode = %d, want %d", statusCode, testCase.statusCode)
			}

			if message != testCase.message {
				t.Fatalf("message = %q, want %q", message, testCase.message)
			}
		})
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

type internalgatewayTaskFunc func(ctx context.Context, state *internalgateway.State) error

func (fn internalgatewayTaskFunc) Run(ctx context.Context, state *internalgateway.State) error {
	return fn(ctx, state)
}

func mustHS256Token(t *testing.T, secret []byte) string {
	t.Helper()

	issuedAt := time.Now().UTC()

	headerPayload, err := json.Marshal(map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("Marshal returned error for header: %v", err)
	}

	claimsPayload, err := json.Marshal(map[string]any{
		"aud": "wintergate",
		"exp": issuedAt.Add(time.Minute).Unix(),
		"iat": issuedAt.Unix(),
		"iss": "auth-service",
		"sub": "user-1",
	})
	if err != nil {
		t.Fatalf("Marshal returned error for claims: %v", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerPayload) + "." + base64.RawURLEncoding.EncodeToString(claimsPayload)
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(signingInput)); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
