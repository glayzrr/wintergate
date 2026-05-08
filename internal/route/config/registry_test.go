package config

import (
	"errors"
	"testing"
)

func TestRegistryRegisterStoresRuntimeConfigAndEntries(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(Config{
		Key: "localhost:8080",
		Entries: []Entry{
			{Path: "/orders", HttpMethod: "get", Roles: []string{"ADMIN"}},
			{Path: "/orders/history", HttpMethod: "GET", Roles: []string{"OPS"}},
		},
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	routeInfos, err := registry.RouteInfos("localhost:8080")
	if err != nil {
		t.Fatalf("RouteInfos returned error: %v", err)
	}

	if len(routeInfos) != 2 {
		t.Fatalf("len(routeInfos) = %d, want %d", len(routeInfos), 2)
	}

	if routeInfos[0].Path != "/orders" {
		t.Fatalf("routeInfos[0].Path = %q, want %q", routeInfos[0].Path, "/orders")
	}

	if routeInfos[0].HttpMethod != "GET" {
		t.Fatalf("routeInfos[0].HttpMethod = %q, want %q", routeInfos[0].HttpMethod, "GET")
	}

	if len(routeInfos[0].Roles) != 1 || routeInfos[0].Roles[0] != "ADMIN" {
		t.Fatalf("routeInfos[0].Roles = %#v, want %#v", routeInfos[0].Roles, []string{"ADMIN"})
	}
}

func TestRegistryRegisterUpsertsByConfigKey(t *testing.T) {
	registry := NewRegistry()

	if err := registry.Register(Config{
		Key:     "localhost:8080",
		Entries: []Entry{{Path: "/orders", HttpMethod: "GET"}},
	}); err != nil {
		t.Fatalf("first Register returned error: %v", err)
	}

	if err := registry.Register(Config{
		Key:     "localhost:8080",
		Entries: []Entry{{Path: "/orders/v2", HttpMethod: "GET"}},
	}); err != nil {
		t.Fatalf("second Register returned error: %v", err)
	}

	routeInfos, err := registry.RouteInfos("localhost:8080")
	if err != nil {
		t.Fatalf("RouteInfos returned error: %v", err)
	}
	if len(routeInfos) != 1 || routeInfos[0].Path != "/orders/v2" {
		t.Fatalf("routeInfos = %#v, want replacement /orders/v2", routeInfos)
	}
}

func TestRegistryRegisterReturnsErrorWhenRouteDuplicated(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(Config{
		Key: "localhost:8080",
		Entries: []Entry{
			{Path: "/orders", HttpMethod: "GET"},
			{Path: "/orders", HttpMethod: "GET"},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestRegistryRegisterReturnsErrorWhenHTTPMethodMissing(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(Config{
		Key: "localhost:8080",
		Entries: []Entry{
			{Path: "/orders"},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestRegistryRegisterReturnsErrorWhenEntriesMissing(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(Config{Key: "localhost:8080"})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestRegistryRegisterReturnsErrorWhenPathMissing(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(Config{
		Key: "localhost:8080",
		Entries: []Entry{
			{HttpMethod: "GET"},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestRegistryRegisterReturnsErrorWhenConfigKeyMissing(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(Config{
		Entries: []Entry{
			{Path: "/orders", HttpMethod: "GET"},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestRegistryRegisterReturnsErrorWhenRoleEmpty(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(Config{
		Key: "localhost:8080",
		Entries: []Entry{
			{Path: "/orders", HttpMethod: "GET", Roles: []string{"ADMIN", " "}},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestRegistryRouteInfosReturnsErrorWhenConfigKeyBlankOrMissing(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(Config{
		Key: "localhost:8080",
		Entries: []Entry{
			{Path: "/orders", HttpMethod: "GET"},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if _, err := registry.RouteInfos(""); err == nil {
		t.Fatal("RouteInfos returned nil error for blank config key")
	} else if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if _, err := registry.RouteInfos("missing.local:8080"); err == nil {
		t.Fatal("RouteInfos returned nil error for missing config")
	} else if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("error = %v, want ErrConfigNotFound", err)
	}
}

func TestRegistryRouteInfosReturnsCopiedRoles(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(Config{
		Key: "localhost:8080",
		Entries: []Entry{
			{Path: "/orders", HttpMethod: "GET", Roles: []string{"ADMIN"}},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	routeInfos, err := registry.RouteInfos("localhost:8080")
	if err != nil {
		t.Fatalf("RouteInfos returned error: %v", err)
	}

	routeInfos[0].Roles[0] = "OPS"

	routeInfos, err = registry.RouteInfos("localhost:8080")
	if err != nil {
		t.Fatalf("RouteInfos returned error: %v", err)
	}

	if routeInfos[0].Roles[0] != "ADMIN" {
		t.Fatalf("routeInfos[0].Roles[0] = %q, want %q", routeInfos[0].Roles[0], "ADMIN")
	}
}
