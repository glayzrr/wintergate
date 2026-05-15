package config

import (
	"errors"
	"testing"

	internalconfig "wintergate/internal/config"
)

func TestLoadBalancerNextInstanceRotatesInstances(t *testing.T) {
	snapshot := &internalconfig.Snapshot{
		Services: map[string]internalconfig.ServiceSettings{
			"order-service": {
				ServiceName: "order-service",
				Instances: []internalconfig.InstanceSettings{
					{Scheme: "http", Host: "127.0.0.1", Port: "8080"},
					{Scheme: "http", Host: "127.0.0.2", Port: "8080"},
				},
			},
		},
	}
	loadBalancer := NewLoadBalancer()

	first, err := loadBalancer.NextInstance(snapshot, "order-service")
	if err != nil {
		t.Fatalf("NextInstance returned error for first call: %v", err)
	}
	second, err := loadBalancer.NextInstance(snapshot, "order-service")
	if err != nil {
		t.Fatalf("NextInstance returned error for second call: %v", err)
	}
	third, err := loadBalancer.NextInstance(snapshot, "order-service")
	if err != nil {
		t.Fatalf("NextInstance returned error for third call: %v", err)
	}

	if first.Host != "127.0.0.1" || second.Host != "127.0.0.2" || third.Host != "127.0.0.1" {
		t.Fatalf("rotation hosts = %q, %q, %q", first.Host, second.Host, third.Host)
	}
}

func TestLoadBalancerNextInstanceReturnsConfigNotFoundWhenSnapshotNil(t *testing.T) {
	_, err := NewLoadBalancer().NextInstance(nil, "order-service")
	if err == nil {
		t.Fatal("NextInstance returned nil error")
	}
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("error = %v, want ErrConfigNotFound", err)
	}
}

func TestLoadBalancerNextInstanceReturnsConfigNotFoundWhenServiceMissing(t *testing.T) {
	_, err := NewLoadBalancer().NextInstance(&internalconfig.Snapshot{
		Services: map[string]internalconfig.ServiceSettings{},
	}, "order-service")
	if err == nil {
		t.Fatal("NextInstance returned nil error")
	}
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("error = %v, want ErrConfigNotFound", err)
	}
}
