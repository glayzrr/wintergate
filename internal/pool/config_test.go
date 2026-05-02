package pool

import (
	"errors"
	"testing"
)

func TestConfigForTierReturnsNormalWhenTierBlank(t *testing.T) {
	config, err := GetConfig("")
	if err != nil {
		t.Fatalf("ConfigForTier returned error: %v", err)
	}

	if config.Tier != TierNormal {
		t.Fatalf("config.Tier = %q, want %q", config.Tier, TierNormal)
	}
	if config.MaxIdleConns != 100 {
		t.Fatalf("config.MaxIdleConns = %d, want %d", config.MaxIdleConns, 100)
	}
}

func TestConfigForTierReturnsTierConfig(t *testing.T) {
	tests := []struct {
		name                string
		tier                Tier
		maxIdleConns        int
		maxIdleConnsPerHost int
		maxConnsPerHost     int
	}{
		{
			name:                "normal",
			tier:                TierNormal,
			maxIdleConns:        100,
			maxIdleConnsPerHost: 2,
			maxConnsPerHost:     0,
		},
		{
			name:                "hot",
			tier:                TierHot,
			maxIdleConns:        200,
			maxIdleConnsPerHost: 4,
			maxConnsPerHost:     0,
		},
		{
			name:                "super",
			tier:                TierSuper,
			maxIdleConns:        400,
			maxIdleConnsPerHost: 8,
			maxConnsPerHost:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetConfig(tt.tier)
			if err != nil {
				t.Fatalf("ConfigForTier returned error: %v", err)
			}

			if config.Tier != tt.tier {
				t.Fatalf("config.Tier = %q, want %q", config.Tier, tt.tier)
			}
			if config.MaxIdleConns != tt.maxIdleConns {
				t.Fatalf("config.MaxIdleConns = %d, want %d", config.MaxIdleConns, tt.maxIdleConns)
			}
			if config.MaxIdleConnsPerHost != tt.maxIdleConnsPerHost {
				t.Fatalf("config.MaxIdleConnsPerHost = %d, want %d", config.MaxIdleConnsPerHost, tt.maxIdleConnsPerHost)
			}
			if config.MaxConnsPerHost != tt.maxConnsPerHost {
				t.Fatalf("config.MaxConnsPerHost = %d, want %d", config.MaxConnsPerHost, tt.maxConnsPerHost)
			}
		})
	}
}

func TestConfigForTierNormalizesTier(t *testing.T) {
	config, err := GetConfig(" HOT ")
	if err != nil {
		t.Fatalf("ConfigForTier returned error: %v", err)
	}

	if config.Tier != TierHot {
		t.Fatalf("config.Tier = %q, want %q", config.Tier, TierHot)
	}
}

func TestConfigForTierReturnsErrorWhenTierUnsupported(t *testing.T) {
	_, err := GetConfig("burst")
	if err == nil {
		t.Fatal("ConfigForTier returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}
