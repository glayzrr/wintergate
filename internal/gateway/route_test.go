package gateway

import (
	"context"
	"errors"
	"testing"

	internalconfig "wintergate/internal/config"
	routeconfig "wintergate/internal/route/config"
)

func TestRouteTaskRunStoresMatchedRoutePolicy(t *testing.T) {
	snapshot := &internalconfig.Snapshot{Revision: 7}
	task := NewRouteTask(
		settingsProviderStub{snapshot: snapshot},
		routerStub{
			routeFor: func(receivedSnapshot *internalconfig.Snapshot, method, path string) (routeconfig.RouteInfo, bool) {
				if receivedSnapshot != snapshot {
					t.Fatal("router received unexpected snapshot")
				}
				if method != "GET" || path != "/orders" {
					return routeconfig.RouteInfo{}, false
				}

				return routeconfig.RouteInfo{
					ServiceName: "order-service",
					Path:        "/orders",
					HttpMethod:  "GET",
					Roles:       []string{"ADMIN"},
				}, true
			},
		},
		instanceSelectorStub{
			nextInstance: func(receivedSnapshot *internalconfig.Snapshot, serviceName string) (internalconfig.InstanceSettings, error) {
				if receivedSnapshot != snapshot {
					t.Fatal("instance selector received unexpected snapshot")
				}
				if serviceName != "order-service" {
					t.Fatalf("serviceName = %q, want %q", serviceName, "order-service")
				}

				return internalconfig.InstanceSettings{
					Scheme: "http",
					Host:   "localhost",
					Port:   "8080",
				}, nil
			},
		},
	)
	state := &State{
		Request: Request{
			Method: "GET",
			Path:   "/orders",
		},
	}

	if err := task.Run(context.Background(), state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Settings != snapshot {
		t.Fatal("state.Settings did not store active snapshot")
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
	if state.Route.Instance.Host != "localhost" {
		t.Fatalf("state.Route.Instance.Host = %q, want %q", state.Route.Instance.Host, "localhost")
	}
}

func TestRouteTaskRunReturnsRoutingError(t *testing.T) {
	task := NewRouteTask(
		settingsProviderStub{snapshot: &internalconfig.Snapshot{}},
		routerStub{
			routeFor: func(*internalconfig.Snapshot, string, string) (routeconfig.RouteInfo, bool) {
				return routeconfig.RouteInfo{}, false
			},
		},
		instanceSelectorStub{},
	)
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

func TestRouteTaskRunReturnsErrorWhenSnapshotNil(t *testing.T) {
	task := NewRouteTask(settingsProviderStub{}, routerStub{}, instanceSelectorStub{})

	err := task.Run(context.Background(), &State{})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if !errors.Is(err, routeconfig.ErrConfigNotFound) {
		t.Fatalf("error = %v, want ErrConfigNotFound", err)
	}
}

func TestRouteTaskRunReturnsErrorWhenProviderNil(t *testing.T) {
	task := NewRouteTask(nil, routerStub{}, instanceSelectorStub{})

	err := task.Run(context.Background(), &State{})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

type settingsProviderStub struct {
	snapshot *internalconfig.Snapshot
}

func (s settingsProviderStub) Settings() *internalconfig.Snapshot {
	return s.snapshot
}

type routerStub struct {
	routeFor func(snapshot *internalconfig.Snapshot, method, path string) (routeconfig.RouteInfo, bool)
}

func (s routerStub) RouteFor(snapshot *internalconfig.Snapshot, method, path string) (routeconfig.RouteInfo, bool) {
	if s.routeFor == nil {
		return routeconfig.RouteInfo{}, false
	}

	return s.routeFor(snapshot, method, path)
}

type instanceSelectorStub struct {
	nextInstance func(snapshot *internalconfig.Snapshot, serviceName string) (internalconfig.InstanceSettings, error)
}

func (s instanceSelectorStub) NextInstance(snapshot *internalconfig.Snapshot, serviceName string) (internalconfig.InstanceSettings, error) {
	if s.nextInstance == nil {
		return internalconfig.InstanceSettings{}, nil
	}

	return s.nextInstance(snapshot, serviceName)
}
