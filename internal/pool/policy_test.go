package pool

import (
	"errors"
	"testing"
)

func TestPolicyRegistryDecideReturnsNormalWhenPolicyMissing(t *testing.T) {
	registry := NewPolicyRegistry()

	decision := registry.Decide(Status{
		Service:  "order-service",
		RPS:      1000,
		InFlight: 1000,
	})

	if decision.Registered {
		t.Fatal("decision.Registered = true, want false")
	}
	if decision.Tier != TierNormal {
		t.Fatalf("decision.Tier = %q, want %q", decision.Tier, TierNormal)
	}
	if decision.Dedicated {
		t.Fatal("decision.Dedicated = true, want false")
	}
}

func TestPolicyRegistryDecideReturnsHotWhenRegisteredHotThresholdReached(t *testing.T) {
	registry := NewPolicyRegistry()
	if err := registry.Register([]Policy{
		{
			Service: "order-service",
			Hot:     Threshold{RPS: 100, InFlight: 50},
			Super:   Threshold{RPS: 500, InFlight: 200},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decision := registry.Decide(Status{
		Service:  "order-service",
		RPS:      100,
		InFlight: 10,
	})

	if !decision.Registered {
		t.Fatal("decision.Registered = false, want true")
	}
	if decision.Tier != TierHot {
		t.Fatalf("decision.Tier = %q, want %q", decision.Tier, TierHot)
	}
	if decision.Dedicated {
		t.Fatal("decision.Dedicated = true, want false")
	}
}

func TestPolicyRegistryDecideReturnsSuperWhenRegisteredSuperThresholdReached(t *testing.T) {
	registry := NewPolicyRegistry()
	if err := registry.Register([]Policy{
		{
			Service: "order-service",
			Hot:     Threshold{RPS: 100, InFlight: 50},
			Super:   Threshold{RPS: 500, InFlight: 200},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decision := registry.Decide(Status{
		Service:  "order-service",
		RPS:      1,
		InFlight: 200,
	})

	if decision.Tier != TierSuper {
		t.Fatalf("decision.Tier = %q, want %q", decision.Tier, TierSuper)
	}
	if decision.Dedicated {
		t.Fatal("decision.Dedicated = true, want false")
	}
}

func TestPolicyRegistryDecideDoesNotDedicateWhenThresholdReached(t *testing.T) {
	registry := NewPolicyRegistry()
	if err := registry.Register([]Policy{
		{
			Service: "order-service",
			Hot:     Threshold{RPS: 100},
			Super:   Threshold{RPS: 500},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decision := registry.Decide(Status{
		Service: "order-service",
		RPS:     500,
	})

	if decision.Tier != TierSuper {
		t.Fatalf("decision.Tier = %q, want %q", decision.Tier, TierSuper)
	}
	if decision.Dedicated {
		t.Fatal("decision.Dedicated = true, want false")
	}
}

func TestPolicyRegistryRegisterReplacesPolicies(t *testing.T) {
	registry := NewPolicyRegistry()
	if err := registry.Register([]Policy{
		{
			Service: "order-service",
			Hot:     Threshold{RPS: 100},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if err := registry.Register([]Policy{
		{
			Service: "payment-service",
			Hot:     Threshold{RPS: 100},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if _, found := registry.Policy("order-service"); found {
		t.Fatal("Policy found replaced order-service")
	}
	if _, found := registry.Policy("payment-service"); !found {
		t.Fatal("Policy did not find payment-service")
	}
}

func TestPolicyRegistryRegisterReturnsErrorWhenInvalid(t *testing.T) {
	tests := []struct {
		name     string
		policies []Policy
	}{
		{
			name: "blank service",
			policies: []Policy{
				{Service: " "},
			},
		},
		{
			name: "duplicate service",
			policies: []Policy{
				{Service: "order-service"},
				{Service: " order-service "},
			},
		},
		{
			name: "negative rps",
			policies: []Policy{
				{Service: "order-service", Hot: Threshold{RPS: -1}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewPolicyRegistry()
			err := registry.Register(tt.policies)
			if err == nil {
				t.Fatal("Register returned nil error")
			}
			if !errors.Is(err, ErrInvalidPolicy) {
				t.Fatalf("error = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}
