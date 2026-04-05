package gateway

import (
	"context"
	"errors"
	"testing"

	internalauth "wintergate/internal/auth"
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
