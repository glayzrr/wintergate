package config

import (
	"errors"
	"testing"
	"time"
)

func TestRegistryRegisterStoresRuntimeConfigAndEntries(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		RouteServiceHeader:          "X-Wintergate-Service",
		RouteUpstreamRequestTimeout: 2 * time.Second,
		Entries: []Entry{
			{Path: "/orders", Service: "order-service", ClientIP: "192.0.2.10", Port: 8080},
			{Path: "/users", Service: "user-service", ClientIP: "192.0.2.11", Port: 8081},
		},
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	routeInfo, found := registry.Route("/orders")
	if !found {
		t.Fatal("Route did not find /orders")
	}

	if routeInfo.Service != "order-service" {
		t.Fatalf("routeInfo.Service = %q, want %q", routeInfo.Service, "order-service")
	}

	if routeInfo.ClientIP != "192.0.2.10" {
		t.Fatalf("routeInfo.ClientIP = %q, want %q", routeInfo.ClientIP, "192.0.2.10")
	}

	snapshot, found := registry.Snapshot()
	if !found {
		t.Fatal("Snapshot did not return routing info")
	}

	if len(snapshot) != 2 {
		t.Fatalf("len(snapshot) = %d, want %d", len(snapshot), 2)
	}

	snapshotRoute, found := snapshot["/orders"]
	if !found {
		t.Fatal("snapshot did not contain /orders")
	}

	if snapshotRoute.Port != 8080 {
		t.Fatalf("snapshotRoute.Port = %d, want %d", snapshotRoute.Port, 8080)
	}
}

func TestRegistryRegisterReturnsErrorWhenPathDuplicated(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		RouteServiceHeader:          "X-Wintergate-Service",
		RouteUpstreamRequestTimeout: 2 * time.Second,
		Entries: []Entry{
			{Path: "/orders", Service: "order-service", ClientIP: "192.0.2.10", Port: 8080},
			{Path: "/orders", Service: "billing-service", ClientIP: "192.0.2.11", Port: 8081},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestRegistryRegisterReturnsErrorWhenPortMissing(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		RouteServiceHeader:          "X-Wintergate-Service",
		RouteUpstreamRequestTimeout: 2 * time.Second,
		Entries: []Entry{
			{Path: "/orders", Service: "order-service", ClientIP: "192.0.2.10"},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}
