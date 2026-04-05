package route

import (
	"errors"
	"testing"

	routeconfig "wintergate/internal/route/config"
)

func TestRouterRouteReturnsRouteInfos(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.RuntimeConfig{
		Entries: []routeconfig.Entry{
			{
				Path:       "/orders",
				Service:    "order-service",
				HttpMethod: "GET",
				Roles:      []string{"ADMIN"},
			},
			{
				Path:       "/orders/history",
				Service:    "order-service",
				HttpMethod: "POST",
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	router := NewRouter(registry)

	routeInfos, err := router.Route("order-service")
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}

	if len(routeInfos) != 2 {
		t.Fatalf("len(routeInfos) = %d, want %d", len(routeInfos), 2)
	}

	if routeInfos[0].Path != "/orders" {
		t.Fatalf("routeInfos[0].Path = %q, want %q", routeInfos[0].Path, "/orders")
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
		Entries: []routeconfig.Entry{
			{
				Path:       "/payments",
				Service:    "payment-service",
				HttpMethod: "GET",
			},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	router := NewRouter(routeconfig.NewRegistry())
	if err := router.ReplaceRegistry(replacement); err != nil {
		t.Fatalf("ReplaceRegistry returned error: %v", err)
	}

	routeInfos, err := router.Route("payment-service")
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}

	if len(routeInfos) != 1 {
		t.Fatalf("len(routeInfos) = %d, want %d", len(routeInfos), 1)
	}
}

func TestRouterRouteReturnsErrorWhenRegistryNil(t *testing.T) {
	router := NewRouter(nil)

	_, err := router.Route("order-service")
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

	router := NewRouter(registry)

	_, err := router.Route("missing-service")
	if err == nil {
		t.Fatal("Route returned nil error")
	}

	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("error = %v, want ErrServiceNotFound", err)
	}
}
