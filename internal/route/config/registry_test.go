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
			{Path: "/orders", Service: "order-service"},
			{Path: "/users", Service: "user-service"},
		},
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	service, found := registry.Service("/orders")
	if !found {
		t.Fatal("Service did not find /orders")
	}

	if service != "order-service" {
		t.Fatalf("service = %q, want %q", service, "order-service")
	}

	snapshot, found := registry.Snapshot()
	if !found {
		t.Fatal("Snapshot did not return runtime config")
	}

	if snapshot.RouteServiceHeader != "X-Wintergate-Service" {
		t.Fatalf("RouteServiceHeader = %q, want %q", snapshot.RouteServiceHeader, "X-Wintergate-Service")
	}

	if len(snapshot.Entries) != 2 {
		t.Fatalf("len(snapshot.Entries) = %d, want %d", len(snapshot.Entries), 2)
	}

	if snapshot.Entries[0].Path != "/orders" {
		t.Fatalf("snapshot.Entries[0].Path = %q, want %q", snapshot.Entries[0].Path, "/orders")
	}
}

func TestRegistryRegisterReturnsErrorWhenPathDuplicated(t *testing.T) {
	registry := NewRegistry()

	err := registry.Register(RuntimeConfig{
		RouteServiceHeader:          "X-Wintergate-Service",
		RouteUpstreamRequestTimeout: 2 * time.Second,
		Entries: []Entry{
			{Path: "/orders", Service: "order-service"},
			{Path: "/orders", Service: "billing-service"},
		},
	})
	if err == nil {
		t.Fatal("Register returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}
