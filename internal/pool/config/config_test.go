package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigForTierReturnsErrorWhenTierBlank(t *testing.T) {
	configureTestConfig(t)

	_, err := ConfigFor("")
	if err == nil {
		t.Fatal("ConfigFor returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestConfigForTierReturnsTierConfig(t *testing.T) {
	configureTestConfig(t)

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
	configureTestConfig(t)

	config, err := ConfigFor(" HOT ")
	if err != nil {
		t.Fatalf("ConfigFor returned error: %v", err)
	}

	if config.Tier != TierHot {
		t.Fatalf("config.Tier = %q, want %q", config.Tier, TierHot)
	}
}

func TestConfigForTierReturnsErrorWhenTierUnsupported(t *testing.T) {
	configureTestConfig(t)

	_, err := ConfigFor("burst")
	if err == nil {
		t.Fatal("ConfigFor returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadConfigAppliesFileConfig(t *testing.T) {
	restoreRuntimeConfig(t)

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(`pool:
  shared:
    MaxIdleConns: 1
    MaxIdleConnsPerHost: 2
    MaxConnsPerHost: 3
    IdleConnTimeout: 4s
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
`), 0600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	sharedConfig, err := SharedConfig()
	if err != nil {
		t.Fatalf("SharedConfig returned error: %v", err)
	}
	if sharedConfig.MaxIdleConns != 1 {
		t.Fatalf("sharedConfig.MaxIdleConns = %d, want %d", sharedConfig.MaxIdleConns, 1)
	}
	if sharedConfig.IdleConnTimeout != 4*time.Second {
		t.Fatalf("sharedConfig.IdleConnTimeout = %s, want %s", sharedConfig.IdleConnTimeout, 4*time.Second)
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
	if config.TLSHandshakeTimeout != 0 {
		t.Fatalf("config.TLSHandshakeTimeout = %s, want %s", config.TLSHandshakeTimeout, time.Duration(0))
	}
}

func TestLoadConfigReturnsErrorWhenFileValueMissing(t *testing.T) {
	restoreRuntimeConfig(t)

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(`pool:
  shared:
    MaxIdleConns: 1
    MaxIdleConnsPerHost: 2
    MaxConnsPerHost: 3
    IdleConnTimeout: 4s
  tier:
    normal:
      MaxIdleConns: 11
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
`), 0600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	err := LoadConfig(configPath)
	if err == nil {
		t.Fatal("LoadConfig returned nil error")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}
}

func configureTestConfig(t *testing.T) {
	t.Helper()

	restoreRuntimeConfig(t)

	if err := Configure(testSharedConfig(), testTierConfigs()); err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
}

func restoreRuntimeConfig(t *testing.T) {
	t.Helper()

	previousSharedConfig := sharedConfig
	previousTierConfigs := make(map[Tier]Config, len(tierConfigs))
	for tier, config := range tierConfigs {
		previousTierConfigs[tier] = config
	}
	previousConfigured := configured

	t.Cleanup(func() {
		sharedConfig = previousSharedConfig
		tierConfigs = previousTierConfigs
		configured = previousConfigured
	})
}

func testSharedConfig() Config {
	return Config{
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 256,
		MaxConnsPerHost:     512,
		IdleConnTimeout:     45 * time.Second,
	}
}

func testTierConfigs() map[Tier]Config {
	return map[Tier]Config{
		TierNormal: {
			MaxIdleConns:        1024,
			MaxIdleConnsPerHost: 512,
			MaxConnsPerHost:     1024,
			IdleConnTimeout:     90 * time.Second,
		},
		TierHot: {
			MaxIdleConns:        2048,
			MaxIdleConnsPerHost: 1024,
			MaxConnsPerHost:     2048,
			IdleConnTimeout:     180 * time.Second,
		},
		TierSuper: {
			MaxIdleConns:        4096,
			MaxIdleConnsPerHost: 2048,
			MaxConnsPerHost:     4096,
			IdleConnTimeout:     360 * time.Second,
		},
	}
}
