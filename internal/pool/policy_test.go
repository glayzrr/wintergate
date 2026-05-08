package pool

import (
	"errors"
	"testing"
)

func TestPolicyRegistryDecideUsesSharedPoolWhenPolicyMissing(t *testing.T) {
	registry := NewPolicyRegistry()

	decision := registry.Decide(Status{
		ConfigKey: "order-service",
		RPS:       1000,
		InFlight:  1000,
	})

	if decision.Dedicated {
		t.Fatal("decision.Dedicated = true, want false")
	}
}

func TestPolicyRegistryDecideReturnsDedicatedHotWhenThresholdReached(t *testing.T) {
	registry := NewPolicyRegistry()
	if err := registry.Register([]Policy{
		{
			ConfigKey: "order-service",
			Hot:       Threshold{RPS: 100, InFlight: 50},
			Super:     Threshold{RPS: 500, InFlight: 200},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decision := registry.Decide(Status{
		ConfigKey: "order-service",
		RPS:       100,
		InFlight:  10,
	})

	if decision.Tier != TierHot {
		t.Fatalf("decision.Tier = %q, want %q", decision.Tier, TierHot)
	}
	if !decision.Dedicated {
		t.Fatal("decision.Dedicated = false, want true")
	}
}

func TestPolicyRegistryDecideReturnsDedicatedSuperWhenThresholdReached(t *testing.T) {
	registry := NewPolicyRegistry()
	if err := registry.Register([]Policy{
		{
			ConfigKey: "order-service",
			Hot:       Threshold{RPS: 100, InFlight: 50},
			Super:     Threshold{RPS: 500, InFlight: 200},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decision := registry.Decide(Status{
		ConfigKey: "order-service",
		RPS:       1,
		InFlight:  200,
	})

	if decision.Tier != TierSuper {
		t.Fatalf("decision.Tier = %q, want %q", decision.Tier, TierSuper)
	}
	if !decision.Dedicated {
		t.Fatal("decision.Dedicated = false, want true")
	}
}

func TestPolicyRegistryDecideUsesDedicatedPoolWhenPolicyConfigured(t *testing.T) {
	registry := NewPolicyRegistry()
	if err := registry.Register([]Policy{
		{
			ConfigKey: "order-service",
			Hot:       Threshold{RPS: 100},
			Super:     Threshold{RPS: 500},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	decision := registry.Decide(Status{
		ConfigKey: "order-service",
		RPS:       500,
	})

	if decision.Tier != TierSuper {
		t.Fatalf("decision.Tier = %q, want %q", decision.Tier, TierSuper)
	}
	if !decision.Dedicated {
		t.Fatal("decision.Dedicated = false, want true")
	}
}

func TestPolicyRegistryRegisterStoresAndReplacesPolicies(t *testing.T) {
	registry := NewPolicyRegistry()
	if err := registry.Register([]Policy{
		{
			ConfigKey: "order-service",
			Hot:       Threshold{RPS: 100},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if err := registry.Register([]Policy{
		{
			ConfigKey: "order-service",
			Hot:       Threshold{RPS: 200},
		},
		{
			ConfigKey: "payment-service",
			Hot:       Threshold{RPS: 100},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	orderPolicy, found := registry.PolicyFor("order-service")
	if !found {
		t.Fatal("Policy did not find order-service")
	}
	if orderPolicy.Hot.RPS != 200 {
		t.Fatalf("orderPolicy.Hot.RPS = %f, want %f", orderPolicy.Hot.RPS, float64(200))
	}
	if _, found := registry.PolicyFor("payment-service"); !found {
		t.Fatal("Policy did not find payment-service")
	}
}

func TestPolicyRegistryRegisterUsesLastPolicyWhenConfigKeyDuplicated(t *testing.T) {
	registry := NewPolicyRegistry()

	if err := registry.Register([]Policy{
		{
			ConfigKey: "order-service",
			Hot:       Threshold{RPS: 100},
		},
		{
			ConfigKey: " order-service ",
			Hot:       Threshold{RPS: 200},
		},
	}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	policy, found := registry.PolicyFor("order-service")
	if !found {
		t.Fatal("Policy did not find order-service")
	}
	if policy.Hot.RPS != 200 {
		t.Fatalf("policy.Hot.RPS = %f, want %f", policy.Hot.RPS, float64(200))
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
				{ConfigKey: " "},
			},
		},
		{
			name: "negative rps",
			policies: []Policy{
				{ConfigKey: "order-service", Hot: Threshold{RPS: -1}},
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
