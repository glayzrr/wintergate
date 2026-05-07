package config

import (
	"errors"
	"testing"
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

func TestRegisterStoresSettingsWhenValid(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(validSettings(), "localhost", "8080")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	authRuntimeConfig, authConfigFound := registerer.authRegistry.SnapshotFor("localhost:8080")
	if !authConfigFound {
		t.Fatal("Snapshot did not return auth config")
	}

	if string(authRuntimeConfig.JWTSecret) != "shared-secret" {
		t.Fatalf("JWTSecret = %q, want %q", string(authRuntimeConfig.JWTSecret), "shared-secret")
	}

	routeInfos, err := registerer.routeRegistry.RouteInfos("localhost:8080")
	if err != nil {
		t.Fatalf("RouteInfos returned error: %v", err)
	}
	if len(routeInfos) != 1 {
		t.Fatalf("len(routeInfos) = %d, want %d", len(routeInfos), 1)
	}
	if routeInfos[0].Path != "/api/order" {
		t.Fatalf("routeInfos[0].Path = %q, want %q", routeInfos[0].Path, "/api/order")
	}

	if _, found := registerer.authRegistry.SnapshotFor("localhost:8080"); !found {
		t.Fatal("SnapshotFor did not find localhost:8080")
	}
}

func TestRegisterReturnsErrorWhenGlobalMissing(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Endpoints: validEndpointSettings(),
	}, "localhost", "8080")
	if err == nil {
		t.Fatal("Register returned nil error")
	}
	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
}

func TestRegisterReturnsErrorWhenEndpointsMissing(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Global: &GlobalSettings{
			Auth: validAuthSettings(),
		},
	}, "localhost", "8080")
	if err == nil {
		t.Fatal("Register returned nil error")
	}
	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
}

func TestRouteRuntimeConfigReturnsEntries(t *testing.T) {
	registerer := NewRegisterer()

	cfg := registerer.registerRouteConfig("localhost:8080", validEndpointSettings())
	if cfg.Key != "localhost:8080" {
		t.Fatalf("cfg.Key = %q, want %q", cfg.Key, "localhost:8080")
	}
	if len(cfg.Entries) != 1 {
		t.Fatalf("len(cfg.Entries) = %d, want %d", len(cfg.Entries), 1)
	}

	cfg.Entries[0].Roles[0] = "GUEST"
	cfg = registerer.registerRouteConfig("localhost:8080", validEndpointSettings())
	if cfg.Entries[0].Roles[0] != "ADMIN" {
		t.Fatalf("cfg.Entries[0].Roles[0] = %q, want %q", cfg.Entries[0].Roles[0], "ADMIN")
	}
}

func TestRegisterUpsertsSettingsByHostPort(t *testing.T) {
	registerer := NewRegisterer()

	if err := registerer.Register(validSettings(), "localhost", "8080"); err != nil {
		t.Fatalf("first Register returned error: %v", err)
	}
	if err := registerer.Register(validSettings(), "localhost", "8081"); err != nil {
		t.Fatalf("second Register returned error: %v", err)
	}

	if _, err := registerer.routeRegistry.RouteInfos("localhost:8080"); err != nil {
		t.Fatalf("RouteInfos returned error for first service: %v", err)
	}
	if _, err := registerer.routeRegistry.RouteInfos("localhost:8081"); err != nil {
		t.Fatalf("RouteInfos returned error for second service: %v", err)
	}
}

func validSettings() Settings {
	return Settings{
		Global: &GlobalSettings{
			Auth: validAuthSettings(),
		},
		Threshold: &ThresholdSettings{
			Hot:   ThresholdPoint{RPS: 100, InFlight: 14},
			Super: ThresholdPoint{RPS: 150, InFlight: 50},
		},
		Endpoints: validEndpointSettings(),
	}
}

func validAuthSettings() *AuthSettings {
	return &AuthSettings{
		JWTAlgorithm: "HS256",
		JWTAudience:  "wintergate",
		JWTClockSkew: "1m",
		JWTIssuer:    "auth-service",
		JWTSecret:    " shared-secret ",
	}
}

func validEndpointSettings() []EndpointSettings {
	return []EndpointSettings{
		{
			Path:   "/api/order",
			Method: "POST",
			Roles:  []string{"ADMIN", "OPS"},
		},
	}
}
