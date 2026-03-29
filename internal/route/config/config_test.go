package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadEnvConfig(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteTableURL:               "http://route-service.local/routes",
		envRouteTableRequestTimeout:    "2s",
		envRouteTableRefreshInterval:   "30s",
		envRouteUpstreamRequestTimeout: "3s",
	})

	cfg, err := LoadEnvConfig(envPath)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}

	if cfg.RouteServiceHeader != "X-Wintergate-Service" {
		t.Fatalf("RouteServiceHeader = %q, want %q", cfg.RouteServiceHeader, "X-Wintergate-Service")
	}

	if cfg.RouteTableURL != "http://route-service.local/routes" {
		t.Fatalf("RouteTableURL = %q, want %q", cfg.RouteTableURL, "http://route-service.local/routes")
	}

	if cfg.RouteTableRequestTimeout != 2*time.Second {
		t.Fatalf("RouteTableRequestTimeout = %s, want %s", cfg.RouteTableRequestTimeout, 2*time.Second)
	}

	if cfg.RouteTableRefreshInterval != 30*time.Second {
		t.Fatalf("RouteTableRefreshInterval = %s, want %s", cfg.RouteTableRefreshInterval, 30*time.Second)
	}

	if cfg.RouteUpstreamRequestTimeout != 3*time.Second {
		t.Fatalf("RouteUpstreamRequestTimeout = %s, want %s", cfg.RouteUpstreamRequestTimeout, 3*time.Second)
	}
}

func TestLoadEnvConfigPrefersProcessEnv(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteTableURL:               "http://route-service.local/routes",
		envRouteTableRequestTimeout:    "2s",
		envRouteTableRefreshInterval:   "30s",
		envRouteUpstreamRequestTimeout: "3s",
	})

	t.Setenv(envRouteServiceHeader, "X-Route-Service")

	cfg, err := LoadEnvConfig(envPath)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}

	if cfg.RouteServiceHeader != "X-Route-Service" {
		t.Fatalf("RouteServiceHeader = %q, want %q", cfg.RouteServiceHeader, "X-Route-Service")
	}
}

func TestLoadEnvConfigReturnsErrorWhenRequiredKeyMissing(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteTableRequestTimeout:    "2s",
		envRouteTableRefreshInterval:   "30s",
		envRouteUpstreamRequestTimeout: "3s",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envRouteTableURL) {
		t.Fatalf("error = %q, want missing key %q in message", err.Error(), envRouteTableURL)
	}
}

func TestLoadEnvConfigReturnsErrorWhenDurationInvalid(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteTableURL:               "http://route-service.local/routes",
		envRouteTableRequestTimeout:    "2s",
		envRouteTableRefreshInterval:   "30s",
		envRouteUpstreamRequestTimeout: "not-a-duration",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envRouteUpstreamRequestTimeout) {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), envRouteUpstreamRequestTimeout)
	}
}

func TestLoadEnvConfigReturnsErrorWhenRouteTableRequestTimeoutInvalid(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteTableURL:               "http://route-service.local/routes",
		envRouteTableRequestTimeout:    "not-a-duration",
		envRouteTableRefreshInterval:   "30s",
		envRouteUpstreamRequestTimeout: "3s",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envRouteTableRequestTimeout) {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), envRouteTableRequestTimeout)
	}
}

func TestLoadEnvConfigReturnsErrorWhenRouteTableRefreshIntervalInvalid(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteTableURL:               "http://route-service.local/routes",
		envRouteTableRequestTimeout:    "2s",
		envRouteTableRefreshInterval:   "not-a-duration",
		envRouteUpstreamRequestTimeout: "3s",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envRouteTableRefreshInterval) {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), envRouteTableRefreshInterval)
	}
}

func resetEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		envRouteServiceHeader,
		envRouteTableURL,
		envRouteTableRequestTimeout,
		envRouteTableRefreshInterval,
		envRouteUpstreamRequestTimeout,
	}

	originalValues := make(map[string]string, len(keys))
	originalState := make(map[string]bool, len(keys))
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if ok {
			originalValues[key] = value
		}
		originalState[key] = ok

		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
	}

	t.Cleanup(func() {
		for _, key := range keys {
			if !originalState[key] {
				if err := os.Unsetenv(key); err != nil {
					t.Fatalf("cleanup unset env %s: %v", key, err)
				}

				continue
			}

			if err := os.Setenv(key, originalValues[key]); err != nil {
				t.Fatalf("cleanup restore env %s: %v", key, err)
			}
		}
	})
}

func writeEnvFile(t *testing.T, values map[string]string) string {
	t.Helper()

	var builder strings.Builder
	for key, value := range values {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(value)
		builder.WriteString("\n")
	}

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	return path
}
