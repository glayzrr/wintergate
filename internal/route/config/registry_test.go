package config

import (
	"errors"
	"testing"
)

func TestRegistryRegisterStoresRuntimeConfigAndEntries(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		Entries: []Entry{
			{Path: "/orders", Service: "order-service", HttpMethod: "get", Roles: []string{"ADMIN"}},
			{Path: "/orders/history", Service: "order-service", HttpMethod: "GET", Roles: []string{"OPS"}},
			{Path: "/users", Service: "user-service", HttpMethod: "POST"},
		},
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	routeInfos, found := registry.RouteInfos("order-service")
	if !found {
		t.Fatal("RouteInfos did not find order-service")
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

func TestRegistryRegisterReturnsErrorWhenServiceRouteDuplicated(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		Entries: []Entry{
			{Path: "/orders", Service: "order-service", HttpMethod: "GET"},
			{Path: "/orders", Service: "order-service", HttpMethod: "GET"},
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

	err := registry.Register(RuntimeConfig{
		Entries: []Entry{
			{Path: "/orders", Service: "order-service"},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}
