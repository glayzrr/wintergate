package gateway

import (
	"context"
	"errors"
	"testing"

	routeconfig "wintergate/internal/route/config"
)

func TestRouteTaskRunStoresMatchedRoutePolicy(t *testing.T) {
	task := NewRouteTask(routeProviderFunc(func(method, path string) (routeconfig.RouteInfo, bool) {
		if method != "GET" || path != "/orders" {
			return routeconfig.RouteInfo{}, false
		}

		return routeconfig.RouteInfo{
			ServiceName: "order-service",
			Path:        "/orders",
			HttpMethod:  "GET",
			Roles:       []string{"ADMIN"},
		}, true
	}))
	state := &State{
		Request: Request{
			Method: "GET",
			Path:   "/orders",
		},
	}

	if err := task.Run(context.Background(), state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Request.ServiceName != "order-service" {
		t.Fatalf("state.Request.ServiceName = %q, want %q", state.Request.ServiceName, "order-service")
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
	task := NewRouteTask(routeProviderFunc(func(string, string) (routeconfig.RouteInfo, bool) {
		return routeconfig.RouteInfo{}, false
	}))
	state := &State{
		Request: Request{
			Method: "GET",
			Path:   "/missing",
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, routeconfig.ErrConfigNotFound) {
		t.Fatalf("error = %v, want ErrConfigNotFound", err)
	}
}

func TestRouteTaskRunReturnsErrorWhenProviderNil(t *testing.T) {
	task := NewRouteTask(nil)

	err := task.Run(context.Background(), &State{})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

type routeProviderFunc func(method, path string) (routeconfig.RouteInfo, bool)

func (fn routeProviderFunc) RouteFor(method, path string) (routeconfig.RouteInfo, bool) {
	return fn(method, path)
}
