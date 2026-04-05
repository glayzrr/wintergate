package config

import "testing"

func TestToRouteInfosReturnsNilWhenInputEmpty(t *testing.T) {
	routeInfos := toRouteInfos(nil)
	if routeInfos != nil {
		t.Fatalf("routeInfos = %#v, want nil", routeInfos)
	}
}

func TestToRouteInfosCopiesRoles(t *testing.T) {
	routeInfos := toRouteInfos([]RegistryRouteInfo{
		{
			Path:       "/orders",
			HttpMethod: "GET",
			Roles:      []string{"ADMIN"},
		},
	})

	if len(routeInfos) != 1 {
		t.Fatalf("len(routeInfos) = %d, want %d", len(routeInfos), 1)
	}

	routeInfos[0].Roles[0] = "OPS"

	routeInfos = toRouteInfos([]RegistryRouteInfo{
		{
			Path:       "/orders",
			HttpMethod: "GET",
			Roles:      []string{"ADMIN"},
		},
	})

	if routeInfos[0].Roles[0] != "ADMIN" {
		t.Fatalf("routeInfos[0].Roles[0] = %q, want %q", routeInfos[0].Roles[0], "ADMIN")
	}
}
