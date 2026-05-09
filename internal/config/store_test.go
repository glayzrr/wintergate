package config

import (
	"errors"
	"testing"
)

func TestStoreRegisterServiceStoresRouteBinding(t *testing.T) {
	store := NewStore()
	settings := validSettings()
	settings.ServiceName = " Order-Service "
	settings.Endpoints = []EndpointSettings{
		{Path: " /api/order ", Method: "post", Roles: []string{" ADMIN "}},
	}

	if err := store.RegisterService(settings, InstanceSettings{Host: "localhost", Port: "8080"}); err != nil {
		t.Fatalf("RegisterService returned error: %v", err)
	}

	binding, found := store.RouteFor("POST", "/api/order")
	if !found {
		t.Fatal("RouteFor did not find route binding")
	}
	if binding.ServiceName != "order-service" {
		t.Fatalf("binding.ServiceName = %q, want %q", binding.ServiceName, "order-service")
	}
	if binding.Method != "POST" {
		t.Fatalf("binding.Method = %q, want %q", binding.Method, "POST")
	}
	if len(binding.Roles) != 1 || binding.Roles[0] != "ADMIN" {
		t.Fatalf("binding.Roles = %#v, want %#v", binding.Roles, []string{"ADMIN"})
	}

	service, found := store.ServiceFor("ORDER-service")
	if !found {
		t.Fatal("ServiceFor did not find service")
	}
	if service.ServiceName != "order-service" {
		t.Fatalf("service.ServiceName = %q, want %q", service.ServiceName, "order-service")
	}
	if len(service.Endpoints) != 1 || service.Endpoints[0].Method != "POST" {
		t.Fatalf("service.Endpoints = %#v, want normalized POST endpoint", service.Endpoints)
	}
}

func TestStoreNextInstanceReturnsRoundRobinInstances(t *testing.T) {
	store := NewStore()
	settings := validSettings()
	settings.ServiceName = "order-service"

	if err := store.RegisterService(settings, InstanceSettings{Host: "localhost", Port: "8080"}); err != nil {
		t.Fatalf("RegisterService returned error: %v", err)
	}
	if err := store.RegisterService(settings, InstanceSettings{Host: "localhost", Port: "8081"}); err != nil {
		t.Fatalf("second RegisterService returned error: %v", err)
	}

	for index, wantPort := range []string{"8080", "8081", "8080"} {
		instance, err := store.NextInstance("order-service")
		if err != nil {
			t.Fatalf("NextInstance %d returned error: %v", index, err)
		}
		if instance.Port != wantPort {
			t.Fatalf("NextInstance %d port = %q, want %q", index, instance.Port, wantPort)
		}
	}
}

func TestStoreNextInstanceReturnsErrorWhenInstancesMissing(t *testing.T) {
	store := NewStore()
	settings := validSettings()
	settings.ServiceName = "order-service"

	if err := store.RegisterService(settings, InstanceSettings{Host: "localhost", Port: "8080"}); err != nil {
		t.Fatalf("RegisterService returned error: %v", err)
	}

	_, err := store.NextInstance("payment-service")
	if err == nil {
		t.Fatal("NextInstance returned nil error")
	}
	if !errors.Is(err, ErrServiceNotFound) {
		t.Fatalf("error = %v, want ErrServiceNotFound", err)
	}
}

func TestStoreRegisterServiceRejectsRouteOwnedByAnotherService(t *testing.T) {
	store := NewStore()
	orderSettings := validSettings()
	orderSettings.ServiceName = "order-service"
	paymentSettings := validSettings()
	paymentSettings.ServiceName = "payment-service"

	if err := store.RegisterService(orderSettings, InstanceSettings{Host: "localhost", Port: "8080"}); err != nil {
		t.Fatalf("first RegisterService returned error: %v", err)
	}

	err := store.RegisterService(paymentSettings, InstanceSettings{Host: "localhost", Port: "8081"})
	if err == nil {
		t.Fatal("RegisterService returned nil error")
	}
	if !errors.Is(err, ErrInvalidSettings) {
		t.Fatalf("error = %v, want ErrInvalidSettings", err)
	}
}

func TestStoreRegisterServicePreservesRegisteredInstances(t *testing.T) {
	store := NewStore()
	settings := validSettings()
	settings.ServiceName = "order-service"

	if err := store.RegisterService(settings, InstanceSettings{Host: "localhost", Port: "8080"}); err != nil {
		t.Fatalf("RegisterService returned error: %v", err)
	}

	nextSettings := validSettings()
	nextSettings.ServiceName = "order-service"
	nextSettings.Endpoints = []EndpointSettings{{Path: "/api/order/v2", Method: "GET"}}
	if err := store.RegisterService(nextSettings, InstanceSettings{Host: "localhost", Port: "8081"}); err != nil {
		t.Fatalf("second RegisterService returned error: %v", err)
	}

	service, found := store.ServiceFor("order-service")
	if !found {
		t.Fatal("ServiceFor did not find service")
	}
	if len(service.Instances) != 2 || service.Instances[0].Port != "8080" || service.Instances[1].Port != "8081" {
		t.Fatalf("service.Instances = %#v, want registered instances", service.Instances)
	}
	if _, found := store.RouteFor("POST", "/api/order"); found {
		t.Fatal("RouteFor found old service route")
	}
	if _, found := store.RouteFor("GET", "/api/order/v2"); !found {
		t.Fatal("RouteFor did not find replacement route")
	}
}
