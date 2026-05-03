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

	err := registerer.Register(validSettings())
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

	service, err := registerer.routeRegistry.ServiceFor("localhost", "8080")
	if err != nil {
		t.Fatalf("ServiceFor returned error: %v", err)
	}
	if service != "order-service" {
		t.Fatalf("service = %q, want %q", service, "order-service")
	}
}

func TestRegisterReturnsErrorWhenGlobalMissing(t *testing.T) {
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

func TestRegisterReturnsErrorWhenRoutesMissing(t *testing.T) {
	registerer := NewRegisterer()

	err := registerer.Register(Settings{
		Global: &GlobalSettings{
			Auth: validAuthSettings(),
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}
	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
}

func TestRouteRuntimeConfigReturnsEntries(t *testing.T) {
	registerer := NewRegisterer()

	cfg := registerer.registerRouteConfig(validRouteSettings())
	if len(cfg.Services) != 1 {
		t.Fatalf("len(cfg.Services) = %d, want %d", len(cfg.Services), 1)
	}
	if len(cfg.Entries) != 1 {
		t.Fatalf("len(cfg.Entries) = %d, want %d", len(cfg.Entries), 1)
	}
	if cfg.Entries[0].Service != "order-service" {
		t.Fatalf("cfg.Entries[0].Service = %q, want %q", cfg.Entries[0].Service, "order-service")
	}

	cfg.Entries[0].Roles[0] = "GUEST"
	cfg = registerer.registerRouteConfig(validRouteSettings())
	if cfg.Entries[0].Roles[0] != "ADMIN" {
		t.Fatalf("cfg.Entries[0].Roles[0] = %q, want %q", cfg.Entries[0].Roles[0], "ADMIN")
	}
}

func validSettings() Settings {
	return Settings{
		Global: &GlobalSettings{
			Auth: validAuthSettings(),
		},
		Routes: validRouteSettings(),
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

func validRouteSettings() []RouteSettings {
	return []RouteSettings{
		{
			Name: "order-service",
			Host: "localhost",
			Port: 8080,
			Threshold: &ThresholdSettings{
				Hot:   ThresholdPoint{RPS: 100, InFlight: 14},
				Super: ThresholdPoint{RPS: 150, InFlight: 50},
			},
			Endpoints: []EndpointSettings{
				{
					Path:   "/api/order",
					Method: "POST",
					Roles:  []string{"ADMIN", "OPS"},
				},
			},
		},
	}
}
