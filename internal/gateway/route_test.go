package gateway

import (
	"context"
	"errors"
	"testing"

	routeconfig "wintergate/internal/route/config"
)

func TestRouteTaskRunStoresMatchedRoutePolicy(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.Config{
		Services: []routeconfig.Service{
			{Name: "order-service", Host: "localhost", Port: 8080},
		},
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

	task := NewRouteTask(registry)
	state := &State{
		Request: Request{
			Host:   "localhost",
			Port:   "8080",
			Method: "GET",
			Path:   "/orders",
		},
	}

	if err := task.Run(context.Background(), state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Request.Service != "order-service" {
		t.Fatalf("state.Request.Service = %q, want %q", state.Request.Service, "order-service")
	}
	if state.Route == nil {
		t.Fatal("state.Route is nil")
	}
	if state.Route.Path != "/orders" {
		t.Fatalf("state.Route.Path = %q, want %q", state.Route.Path, "/orders")
	}
	if len(state.Route.Roles) != 1 || state.Route.Roles[0] != "ADMIN" {
		t.Fatalf("state.Route.Roles = %#v, want %#v", state.Route.Roles, []string{"ADMIN"})
	}
}

func TestRouteTaskRunReturnsRoutingError(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.Config{
		Services: []routeconfig.Service{
			{Name: "order-service", Host: "localhost", Port: 8080},
		},
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

	task := NewRouteTask(registry)
	state := &State{
		Request: Request{
			Host:   "missing.local",
			Port:   "8080",
			Method: "GET",
			Path:   "/missing",
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, routeconfig.ErrServiceNotFound) {
		t.Fatalf("error = %v, want ErrServiceNotFound", err)
	}
}

func TestRouteTaskRunLeavesRouteNilWhenRouteDoesNotMatchRequest(t *testing.T) {
	registry := routeconfig.NewRegistry()
	if err := registry.Register(routeconfig.Config{
		Services: []routeconfig.Service{
			{Name: "order-service", Host: "localhost", Port: 8080},
		},
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

	task := NewRouteTask(registry)
	state := &State{
		Request: Request{
			Host:   "localhost",
			Port:   "8080",
			Method: "POST",
			Path:   "/orders",
		},
	}

	if err := task.Run(context.Background(), state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if state.Route != nil {
		t.Fatalf("state.Route = %#v, want nil", state.Route)
	}
}

func TestMatchRoute(t *testing.T) {
	tests := []struct {
		name      string
		routeInfo routeconfig.RouteInfo
		method    string
		path      string
		matched   bool
	}{
		{
			name:      "exact",
			routeInfo: routeconfig.RouteInfo{Path: "/orders", HttpMethod: "GET"},
			method:    "GET",
			path:      "/orders",
			matched:   true,
		},
		{
			name:      "all method",
			routeInfo: routeconfig.RouteInfo{Path: "/orders", HttpMethod: "ALL"},
			method:    "POST",
			path:      "/orders",
			matched:   true,
		},
		{
			name:      "wildcard path",
			routeInfo: routeconfig.RouteInfo{Path: "/actuator/**", HttpMethod: "GET"},
			method:    "GET",
			path:      "/actuator/health",
			matched:   true,
		},
		{
			name:      "mismatched method",
			routeInfo: routeconfig.RouteInfo{Path: "/orders", HttpMethod: "GET"},
			method:    "POST",
			path:      "/orders",
			matched:   false,
		},
		{
			name:      "mismatched path",
			routeInfo: routeconfig.RouteInfo{Path: "/orders", HttpMethod: "GET"},
			method:    "GET",
			path:      "/payments",
			matched:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if matched := matchRoute(tt.routeInfo, tt.method, tt.path); matched != tt.matched {
				t.Fatalf("matchRoute = %v, want %v", matched, tt.matched)
			}
		})
	}
}
