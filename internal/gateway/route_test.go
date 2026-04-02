package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	internalroute "wintergate/internal/route"
	routeconfig "wintergate/internal/route/config"
)

func TestRouteTaskRunStoresUpstreamURL(t *testing.T) {
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

	task := NewRouteTask(internalroute.NewRouter(registry))
	state := &State{
		Request: Request{
			Path: "/orders",
		},
	}

	err := task.Run(context.Background(), state)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Result.UpstreamURL != "192.0.2.10:8080/orders" {
		t.Fatalf("state.Result.UpstreamURL = %q, want %q", state.Result.UpstreamURL, "192.0.2.10:8080/orders")
	}
}

func TestRouteTaskRunReturnsRoutingError(t *testing.T) {
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

	task := NewRouteTask(internalroute.NewRouter(registry))
	state := &State{
		Request: Request{
			Path: "/missing",
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
