package config

import "testing"

func TestCloneRouteInfoCopiesRoles(t *testing.T) {
	routeInfo := RouteInfo{
		ServiceName: "order-service",
		Path:        "/orders",
		HttpMethod:  "GET",
		Roles:       []string{"ADMIN"},
	}

	clonedRouteInfo := cloneRouteInfo(routeInfo)
	clonedRouteInfo.Roles[0] = "OPS"

	if routeInfo.Roles[0] != "ADMIN" {
		t.Fatalf("routeInfo.Roles[0] = %q, want %q", routeInfo.Roles[0], "ADMIN")
	}
}
