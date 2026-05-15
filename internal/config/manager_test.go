package config

import (
	"errors"
	"testing"
)

func TestManagerRegisterCommitsCandidateAfterValidatorsPass(t *testing.T) {
	manager := NewManager()

	var validated Snapshot
	manager.AddValidator(validatorFunc(func(candidate Snapshot) error {
		validated = candidate
		return nil
	}))

	if err := manager.Register(testSettings("order-service", "/orders")); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if validated.Revision != 1 {
		t.Fatalf("validated.Revision = %d, want %d", validated.Revision, 1)
	}
	if _, found := validated.Services["order-service"]; !found {
		t.Fatal("validated snapshot does not contain order-service")
	}
	if _, found := validated.Routes[RouteKey{Method: "GET", Path: "/orders"}]; !found {
		t.Fatal("validated snapshot does not contain GET /orders route")
	}

	snapshot := manager.Settings()
	if snapshot == nil {
		t.Fatal("manager.Settings returned nil")
	}
	if snapshot.Revision != 1 {
		t.Fatalf("snapshot.Revision = %d, want %d", snapshot.Revision, 1)
	}

	settings, found := manager.ConfigFor("order-service")
	if !found {
		t.Fatal("ConfigFor did not find order-service")
	}
	if settings.Endpoints[0].Path != "/orders" {
		t.Fatalf("settings.Endpoints[0].Path = %q, want %q", settings.Endpoints[0].Path, "/orders")
	}
}

func TestManagerRegisterDoesNotCommitWhenValidatorFails(t *testing.T) {
	manager := NewManager()
	if err := manager.Register(testSettings("order-service", "/orders")); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	rejected := errors.New("rejected candidate")
	manager.AddValidator(validatorFunc(func(candidate Snapshot) error {
		return rejected
	}))

	err := manager.Register(testSettings("order-service", "/payments"))
	if err == nil {
		t.Fatal("Register returned nil error")
	}
	if !errors.Is(err, rejected) {
		t.Fatalf("error = %v, want %v", err, rejected)
	}

	snapshot := manager.Settings()
	if snapshot.Revision != 1 {
		t.Fatalf("snapshot.Revision = %d, want %d", snapshot.Revision, 1)
	}

	settings, found := manager.ConfigFor("order-service")
	if !found {
		t.Fatal("ConfigFor did not find order-service")
	}
	if settings.Endpoints[0].Path != "/orders" {
		t.Fatalf("settings.Endpoints[0].Path = %q, want %q", settings.Endpoints[0].Path, "/orders")
	}
	if _, found := snapshot.Routes[RouteKey{Method: "GET", Path: "/payments"}]; found {
		t.Fatal("failed candidate route was committed")
	}
}

func TestManagerRegisterCoWReusesUnchangedServiceSettings(t *testing.T) {
	manager := NewManager()
	if err := manager.Register(testSettings("order-service", "/orders")); err != nil {
		t.Fatalf("Register returned error for order-service: %v", err)
	}
	before := manager.Settings()

	if err := manager.Register(testSettings("payment-service", "/payments")); err != nil {
		t.Fatalf("Register returned error for payment-service: %v", err)
	}
	after := manager.Settings()

	if before == after {
		t.Fatal("snapshot pointer was reused")
	}
	if before.Revision != 1 || after.Revision != 2 {
		t.Fatalf("revisions = before %d after %d, want 1 and 2", before.Revision, after.Revision)
	}
	if before.Services["order-service"].Global != after.Services["order-service"].Global {
		t.Fatal("unchanged service settings were deep-copied")
	}
	if _, found := after.Services["payment-service"]; !found {
		t.Fatal("new service was not committed")
	}
}

type validatorFunc func(Snapshot) error

func (f validatorFunc) Validate(candidate Snapshot) error {
	return f(candidate)
}

func testSettings(serviceName, path string) Settings {
	return Settings{
		Global: &GlobalSettings{
			Auth: &AuthSettings{
				JWTAlgorithm: "HS256",
				JWTAudience:  "wintergate",
				JWTClockSkew: "1m",
				JWTIssuer:    "auth-service",
				JWTSecret:    "secret",
			},
		},
		Instance: &InstanceSettings{
			Scheme: "http",
			Host:   "localhost",
			Port:   "8080",
		},
		ServiceName: serviceName,
		Endpoints: []EndpointSettings{
			{
				Path:   path,
				Method: "GET",
			},
		},
	}
}
