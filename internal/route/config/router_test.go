package config

import (
	"testing"

	internalconfig "wintergate/internal/config"
)

func TestRouterRouteForMatchesExactRoute(t *testing.T) {
	snapshot := routeSnapshot()
	router := NewRouter()

	routeInfo, found := router.RouteFor(snapshot, "GET", "/orders")
	if !found {
		t.Fatal("RouteFor did not find exact route")
	}

	if routeInfo.ServiceName != "order-service" {
		t.Fatalf("ServiceName = %q, want %q", routeInfo.ServiceName, "order-service")
	}
	if routeInfo.HttpMethod != "GET" {
		t.Fatalf("HttpMethod = %q, want %q", routeInfo.HttpMethod, "GET")
	}
	if len(routeInfo.Roles) != 1 || routeInfo.Roles[0] != "ADMIN" {
		t.Fatalf("Roles = %#v, want [ADMIN]", routeInfo.Roles)
	}
}

func TestRouterRouteForMatchesAllMethodRoute(t *testing.T) {
	snapshot := routeSnapshot()
	router := NewRouter()

	routeInfo, found := router.RouteFor(snapshot, "POST", "/health")
	if !found {
		t.Fatal("RouteFor did not find ALL method route")
	}

	if routeInfo.ServiceName != "health-service" {
		t.Fatalf("ServiceName = %q, want %q", routeInfo.ServiceName, "health-service")
	}
}

func TestRouterRouteForMatchesWildcardRoute(t *testing.T) {
	snapshot := routeSnapshot()
	router := NewRouter()

	routeInfo, found := router.RouteFor(snapshot, "DELETE", "/api/orders/1")
	if !found {
		t.Fatal("RouteFor did not find wildcard route")
	}

	if routeInfo.ServiceName != "api-service" {
		t.Fatalf("ServiceName = %q, want %q", routeInfo.ServiceName, "api-service")
	}
}

func TestRouterRouteForCopiesRoles(t *testing.T) {
	snapshot := routeSnapshot()
	router := NewRouter()

	routeInfo, found := router.RouteFor(snapshot, "GET", "/orders")
	if !found {
		t.Fatal("RouteFor did not find exact route")
	}

	routeInfo.Roles[0] = "OPS"
	original := snapshot.Routes[internalconfig.RouteKey{Method: "GET", Path: "/orders"}]
	if original.Roles[0] != "ADMIN" {
		t.Fatalf("snapshot role = %q, want %q", original.Roles[0], "ADMIN")
	}
}

func TestRouterRouteForReturnsFalseWhenSnapshotNil(t *testing.T) {
	_, found := NewRouter().RouteFor(nil, "GET", "/orders")
	if found {
		t.Fatal("RouteFor found route with nil snapshot")
	}
}

func routeSnapshot() *internalconfig.Snapshot {
	return &internalconfig.Snapshot{
		Services: map[string]internalconfig.ServiceSettings{
			"order-service":  {ServiceName: "order-service"},
			"health-service": {ServiceName: "health-service"},
			"api-service":    {ServiceName: "api-service"},
		},
		Routes: map[internalconfig.RouteKey]internalconfig.RouteEntry{
			{Method: "GET", Path: "/orders"}: {
				ServiceName: "order-service",
				Path:        "/orders",
				Method:      "GET",
				Roles:       []string{"ADMIN"},
			},
			{Method: "ALL", Path: "/health"}: {
				ServiceName: "health-service",
				Path:        "/health",
				Method:      "ALL",
			},
			{Method: "ALL", Path: "/api/**"}: {
				ServiceName: "api-service",
				Path:        "/api/**",
				Method:      "ALL",
			},
		},
		WildcardRoutes: []internalconfig.RouteEntry{
			{
				ServiceName: "api-service",
				Path:        "/api/**",
				Method:      "ALL",
			},
		},
	}
}
