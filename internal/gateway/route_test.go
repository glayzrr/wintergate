package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	internalauth "wintergate/internal/auth"
	authconfig "wintergate/internal/auth/config"
	internalroute "wintergate/internal/route"
	routeconfig "wintergate/internal/route/config"
)

func TestRouteTaskRunReturnsUnauthorizedWhenAuthorizationHeaderMissing(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		Entries: []routeconfig.Entry{
			{
				Path:       "/orders",
				Service:    "order-service",
				HttpMethod: "GET",
				Roles:      []string{"ADMIN"},
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	task := NewRouteTask(internalroute.NewRouter(registry), nil)
	state := &State{
		Request: Request{
			Service: "order-service",
			Method:  "GET",
			Path:    "/orders",
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, internalauth.ErrInvalidAuthorizationHeader) {
		t.Fatalf("error = %v, want ErrInvalidAuthorizationHeader", err)
	}
}

func TestRouteTaskRunReturnsRoutingError(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		Entries: []routeconfig.Entry{
			{
				Path:       "/orders",
				Service:    "order-service",
				HttpMethod: "GET",
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	task := NewRouteTask(internalroute.NewRouter(registry), nil)
	state := &State{
		Request: Request{
			Service: "missing-service",
			Method:  "GET",
			Path:    "/missing",
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, internalroute.ErrServiceNotFound) {
		t.Fatalf("error = %v, want ErrServiceNotFound", err)
	}
}

func TestRouteTaskRunReturnsNilWhenRouteDoesNotMatchRequest(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		Entries: []routeconfig.Entry{
			{
				Path:       "/orders",
				Service:    "order-service",
				HttpMethod: "GET",
				Roles:      []string{"ADMIN"},
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	task := NewRouteTask(internalroute.NewRouter(registry), nil)
	state := &State{
		Request: Request{
			Service: "order-service",
			Method:  "POST",
			Path:    "/orders",
		},
	}

	if err := task.Run(context.Background(), state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRouteTaskRunReturnsWrappedBearerError(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		Entries: []routeconfig.Entry{
			{
				Path:       "/orders",
				Service:    "order-service",
				HttpMethod: "GET",
				Roles:      []string{"ADMIN"},
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	task := NewRouteTask(internalroute.NewRouter(registry), nil)
	state := &State{
		Request: Request{
			Service:             "order-service",
			Method:              "GET",
			Path:                "/orders",
			AuthorizationHeader: "Basic token-value",
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, internalauth.ErrInvalidAuthorizationHeader) {
		t.Fatalf("error = %v, want ErrInvalidAuthorizationHeader", err)
	}
}

func TestRouteTaskRunReturnsDecodeErrorWhenTokenInvalid(t *testing.T) {
	decoder := newHS256Decoder(t, []byte("shared-secret"))
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		Entries: []routeconfig.Entry{
			{
				Path:       "/orders",
				Service:    "order-service",
				HttpMethod: "GET",
				Roles:      []string{"ADMIN"},
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	task := NewRouteTask(internalroute.NewRouter(registry), decoder)
	state := &State{
		Request: Request{
			Service:             "order-service",
			Method:              "GET",
			Path:                "/orders",
			AuthorizationHeader: "Bearer invalid.token",
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, internalauth.ErrInvalidToken) {
		t.Fatalf("error = %v, want ErrInvalidToken", err)
	}
}

func TestRouteTaskRunReturnsInvalidRequestWhenRoleDoesNotMatch(t *testing.T) {
	decoder := newHS256Decoder(t, []byte("shared-secret"))
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		Entries: []routeconfig.Entry{
			{
				Path:       "/orders",
				Service:    "order-service",
				HttpMethod: "GET",
				Roles:      []string{"ADMIN"},
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	task := NewRouteTask(internalroute.NewRouter(registry), decoder)
	state := &State{
		Request: Request{
			Service:             "order-service",
			Method:              "GET",
			Path:                "/orders",
			AuthorizationHeader: "Bearer " + mustHS256Token(t, []byte("shared-secret"), []string{"OPS"}),
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestRouteTaskRunStoresClaimsWhenRoleMatches(t *testing.T) {
	decoder := newHS256Decoder(t, []byte("shared-secret"))
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		Entries: []routeconfig.Entry{
			{
				Path:       "/orders",
				Service:    "order-service",
				HttpMethod: "GET",
				Roles:      []string{"ADMIN"},
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	task := NewRouteTask(internalroute.NewRouter(registry), decoder)
	state := &State{
		Request: Request{
			Service:             "order-service",
			Method:              "GET",
			Path:                "/orders",
			AuthorizationHeader: "Bearer " + mustHS256Token(t, []byte("shared-secret"), []string{"ADMIN", "OPS"}),
		},
	}

	if err := task.Run(context.Background(), state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Claims == nil {
		t.Fatal("state.Claims is nil")
	}

	if len(state.Claims.Roles) != 2 {
		t.Fatalf("len(state.Claims.Roles) = %d, want %d", len(state.Claims.Roles), 2)
	}
}

func TestCheckAuthRoute(t *testing.T) {
	routeInfo := routeconfig.RouteInfo{
		Path:       "/orders",
		HttpMethod: "GET",
	}

	if !checkAuthRoute(routeInfo, "GET", "/orders") {
		t.Fatal("checkAuthRoute returned false, want true")
	}

	if checkAuthRoute(routeInfo, "POST", "/orders") {
		t.Fatal("checkAuthRoute returned true for mismatched method")
	}

	if checkAuthRoute(routeInfo, "GET", "/payments") {
		t.Fatal("checkAuthRoute returned true for mismatched path")
	}
}

func TestCheckRole(t *testing.T) {
	if !checkRole(routeconfig.RouteInfo{}, nil) {
		t.Fatal("checkRole returned false for open route")
	}

	if !checkRole(routeconfig.RouteInfo{Roles: []string{"ADMIN"}}, []string{"OPS", "ADMIN"}) {
		t.Fatal("checkRole returned false for matching role")
	}

	if checkRole(routeconfig.RouteInfo{Roles: []string{"ADMIN"}}, []string{"OPS"}) {
		t.Fatal("checkRole returned true for mismatched role")
	}
}

func newHS256Decoder(t *testing.T, secret []byte) *internalauth.Decoder {
	t.Helper()

	registry := authconfig.NewRegistry()
	if err := registry.Register(authconfig.RuntimeConfig{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: time.Minute,
		JWTIssuer:    "auth-service",
		JWTSecret:    secret,
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	return internalauth.NewDecoder(registry)
}

func mustHS256Token(t *testing.T, secret []byte, roles []string) string {
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
		"aud":   "wintergate",
		"exp":   issuedAt.Add(time.Minute).Unix(),
		"iat":   issuedAt.Unix(),
		"iss":   "auth-service",
		"roles": roles,
		"sub":   "user-1",
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
