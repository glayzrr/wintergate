package pool

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigForTierReturnsNormalWhenTierBlank(t *testing.T) {
	config, err := ConfigFor("")
	if err != nil {
		t.Fatalf("ConfigFor returned error: %v", err)
	}

	if config.Tier != TierNormal {
		t.Fatalf("config.Tier = %q, want %q", config.Tier, TierNormal)
	}
	if config.MaxIdleConns != 1024 {
		t.Fatalf("config.MaxIdleConns = %d, want %d", config.MaxIdleConns, 1024)
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
			maxIdleConns:        1024,
			maxIdleConnsPerHost: 512,
			maxConnsPerHost:     1024,
		},
		{
			name:                "hot",
			tier:                TierHot,
			maxIdleConns:        2048,
			maxIdleConnsPerHost: 1024,
			maxConnsPerHost:     2048,
		},
		{
			name:                "super",
			tier:                TierSuper,
			maxIdleConns:        4096,
			maxIdleConnsPerHost: 2048,
			maxConnsPerHost:     4096,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ConfigFor(tt.tier)
			if err != nil {
				t.Fatalf("ConfigFor returned error: %v", err)
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
	config, err := ConfigFor(" HOT ")
	if err != nil {
		t.Fatalf("ConfigFor returned error: %v", err)
	}

	if config.Tier != TierHot {
		t.Fatalf("config.Tier = %q, want %q", config.Tier, TierHot)
	}
}

func TestConfigForTierReturnsErrorWhenTierUnsupported(t *testing.T) {
	_, err := ConfigFor("burst")
	if err == nil {
		t.Fatal("ConfigFor returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigAppliesFileConfig(t *testing.T) {
	restoreDefaultConfig(t)

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(`pool:
  tier:
    normal:
      MaxIdleConns: 11
      MaxIdleConnsPerHost: 12
      MaxConnsPerHost: 13
      IdleConnTimeout: 14s
    hot:
      MaxIdleConns: 21
      MaxIdleConnsPerHost: 22
      MaxConnsPerHost: 23
      IdleConnTimeout: 24s
    super:
      MaxIdleConns: 31
      MaxIdleConnsPerHost: 32
      MaxConnsPerHost: 33
      IdleConnTimeout: 34s
  default-tier: hot
`), 0600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if DefaultTier() != TierHot {
		t.Fatalf("DefaultTier = %q, want %q", DefaultTier(), TierHot)
	}

	config, err := ConfigFor(TierNormal)
	if err != nil {
		t.Fatalf("ConfigFor returned error: %v", err)
	}

	if config.MaxIdleConns != 11 {
		t.Fatalf("config.MaxIdleConns = %d, want %d", config.MaxIdleConns, 11)
	}
	if config.IdleConnTimeout != 14*time.Second {
		t.Fatalf("config.IdleConnTimeout = %s, want %s", config.IdleConnTimeout, 14*time.Second)
	}
	if config.TLSHandshakeTimeout != 10*time.Second {
		t.Fatalf("config.TLSHandshakeTimeout = %s, want %s", config.TLSHandshakeTimeout, 10*time.Second)
	}
}

func TestLoadConfigKeepsDefaultWhenFileValueMissing(t *testing.T) {
	restoreDefaultConfig(t)

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(`pool:
  tier:
    normal:
      MaxIdleConns: 11
    hot: {}
    super: {}
  default-tier: normal
`), 0600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	config, err := ConfigFor(TierNormal)
	if err != nil {
		t.Fatalf("ConfigFor returned error: %v", err)
	}

	if config.MaxIdleConns != 11 {
		t.Fatalf("config.MaxIdleConns = %d, want %d", config.MaxIdleConns, 11)
	}
	if config.MaxIdleConnsPerHost != 512 {
		t.Fatalf("config.MaxIdleConnsPerHost = %d, want %d", config.MaxIdleConnsPerHost, 512)
	}
	if config.IdleConnTimeout != 90*time.Second {
		t.Fatalf("config.IdleConnTimeout = %s, want %s", config.IdleConnTimeout, 90*time.Second)
	}
}

func restoreDefaultConfig(t *testing.T) {
	t.Helper()

	previousDefaultTier := defaultTier
	previousDefaultConfigs := make(map[Tier]Config, len(defaultConfigs))
	for tier, config := range defaultConfigs {
		previousDefaultConfigs[tier] = config
	}

	t.Cleanup(func() {
		defaultTier = previousDefaultTier
		defaultConfigs = previousDefaultConfigs
	})
}
