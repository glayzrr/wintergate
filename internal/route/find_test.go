package route

import (
	"errors"
	"testing"
	"time"

	routeconfig "wintergate/internal/route/config"
)

func TestRouterRouteReturnsJoinedAddress(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		RouteServiceHeader:          "X-Wintergate-Service",
		RouteUpstreamRequestTimeout: 2 * time.Second,
		Entries: []routeconfig.Entry{
			{
				Path:     "/orders",
				Service:  "order-service",
				ClientIP: "192.0.2.10",
				Port:     8080,
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	router := NewRouter(registry)

	addr, err := router.Route("/orders")
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}

	if addr != "192.0.2.10:8080/orders" {
		t.Fatalf("addr = %q, want %q", addr, "192.0.2.10:8080/orders")
	}
}

func TestRouterReplaceRegistryReturnsErrorWhenRegistryNil(t *testing.T) {
	router := NewRouter(routeconfig.NewRegistry())

	err := router.ReplaceRegistry(nil)
	if err == nil {
		t.Fatal("ReplaceRegistry returned nil error")
	}

	if !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("error = %v, want ErrNilRegistry", err)
	}
}

func TestRouterReplaceRegistryUsesReplacement(t *testing.T) {
	replacement := routeconfig.NewRegistry()
	if err := replacement.Register(routeconfig.RuntimeConfig{
		RouteServiceHeader:          "X-Wintergate-Service",
		RouteUpstreamRequestTimeout: 2 * time.Second,
		Entries: []routeconfig.Entry{
			{
				Path:     "/payments",
				Service:  "payment-service",
				ClientIP: "192.0.2.20",
				Port:     8081,
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	router := NewRouter(routeconfig.NewRegistry())
	if err := router.ReplaceRegistry(replacement); err != nil {
		t.Fatalf("ReplaceRegistry returned error: %v", err)
	}

	addr, err := router.Route("/payments")
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}

	if addr != "192.0.2.20:8081/payments" {
		t.Fatalf("addr = %q, want %q", addr, "192.0.2.20:8081/payments")
	}
}

func TestRouterRouteReturnsErrorWhenRegistryNil(t *testing.T) {
	router := NewRouter(nil)

	_, err := router.Route("/orders")
	if err == nil {
		t.Fatal("Route returned nil error")
	}

	if !errors.Is(err, ErrNilRegistry) {
		t.Fatalf("error = %v, want ErrNilRegistry", err)
	}
}

func TestRouterRouteReturnsErrorWhenServiceMissing(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		RouteServiceHeader:          "X-Wintergate-Service",
		RouteUpstreamRequestTimeout: 2 * time.Second,
		Entries: []routeconfig.Entry{
			{
				Path:     "/orders",
				Service:  "order-service",
				ClientIP: "192.0.2.10",
				Port:     8080,
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	router := NewRouter(registry)

	_, err := router.Route("/missing")
	if err == nil {
		t.Fatal("Route returned nil error")
	}

	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("error = %v, want ErrServiceNotFound", err)
	}
}
