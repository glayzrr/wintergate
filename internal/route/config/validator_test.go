package config

import (
	"errors"
	"testing"

	internalconfig "wintergate/internal/config"
)

func TestValidatorAcceptsConsistentSnapshot(t *testing.T) {
	snapshot := internalconfig.Snapshot{
		Services: map[string]internalconfig.ServiceSettings{
			"order-service": {
				ServiceName: "order-service",
				Endpoints: []internalconfig.EndpointSettings{
					{Path: "/orders", Method: "GET"},
				},
			},
		},
		Routes: map[internalconfig.RouteKey]internalconfig.RouteEntry{
			{Method: "GET", Path: "/orders"}: {
				ServiceName: "order-service",
				Method:      "GET",
				Path:        "/orders",
			},
		},
	}

	if err := NewValidator().Validate(snapshot); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestValidatorRejectsRouteForUnknownService(t *testing.T) {
	snapshot := internalconfig.Snapshot{
		Services: map[string]internalconfig.ServiceSettings{
			"order-service": {
				ServiceName: "order-service",
				Endpoints: []internalconfig.EndpointSettings{
					{Path: "/orders", Method: "GET"},
				},
			},
		},
		Routes: map[internalconfig.RouteKey]internalconfig.RouteEntry{
			{Method: "GET", Path: "/payments"}: {
				ServiceName: "payment-service",
				Method:      "GET",
				Path:        "/payments",
			},
		},
	}

	err := NewValidator().Validate(snapshot)
	if err == nil {
		t.Fatal("Validate returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestValidatorRejectsWildcardListContainingExactRoute(t *testing.T) {
	snapshot := internalconfig.Snapshot{
		Services: map[string]internalconfig.ServiceSettings{
			"order-service": {
				ServiceName: "order-service",
				Endpoints: []internalconfig.EndpointSettings{
					{Path: "/orders", Method: "GET"},
				},
			},
		},
		Routes: map[internalconfig.RouteKey]internalconfig.RouteEntry{},
		WildcardRoutes: []internalconfig.RouteEntry{
			{
				ServiceName: "order-service",
				Method:      "GET",
				Path:        "/orders",
			},
		},
	}

	err := NewValidator().Validate(snapshot)
	if err == nil {
		t.Fatal("Validate returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}
