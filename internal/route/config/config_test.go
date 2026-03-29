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
		envRouteUpstreamRequestTimeout: "3s",
		envRouteUpstreams:             `{"user-service":"http://user-service.default.svc.cluster.local:8080","order-service":"http://order-service.default.svc.cluster.local:8080"}`,
	})

	cfg, err := LoadEnvConfig(envPath)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}

	if cfg.RouteServiceHeader != "X-Wintergate-Service" {
		t.Fatalf("RouteServiceHeader = %q, want %q", cfg.RouteServiceHeader, "X-Wintergate-Service")
	}

	if cfg.RouteUpstreamRequestTimeout != 3*time.Second {
		t.Fatalf("RouteUpstreamRequestTimeout = %s, want %s", cfg.RouteUpstreamRequestTimeout, 3*time.Second)
	}

	if cfg.RouteUpstreams["user-service"] != "http://user-service.default.svc.cluster.local:8080" {
		t.Fatalf("RouteUpstreams[user-service] = %q, want %q", cfg.RouteUpstreams["user-service"], "http://user-service.default.svc.cluster.local:8080")
	}

	if cfg.RouteUpstreams["order-service"] != "http://order-service.default.svc.cluster.local:8080" {
		t.Fatalf("RouteUpstreams[order-service] = %q, want %q", cfg.RouteUpstreams["order-service"], "http://order-service.default.svc.cluster.local:8080")
	}
}

func TestLoadEnvConfigPrefersProcessEnv(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteUpstreamRequestTimeout: "3s",
		envRouteUpstreams:             `{"user-service":"http://user-service.default.svc.cluster.local:8080"}`,
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
		envRouteUpstreamRequestTimeout: "3s",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envRouteUpstreams) {
		t.Fatalf("error = %q, want missing key %q in message", err.Error(), envRouteUpstreams)
	}
}

func TestLoadEnvConfigReturnsErrorWhenDurationInvalid(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteUpstreamRequestTimeout: "not-a-duration",
		envRouteUpstreams:             `{"user-service":"http://user-service.default.svc.cluster.local:8080"}`,
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

func TestLoadEnvConfigReturnsErrorWhenUpstreamsInvalidJSON(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteUpstreamRequestTimeout: "3s",
		envRouteUpstreams:             `{invalid-json}`,
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envRouteUpstreams) {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), envRouteUpstreams)
	}
}

func TestLoadEnvConfigReturnsErrorWhenUpstreamURLInvalid(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteUpstreamRequestTimeout: "3s",
		envRouteUpstreams:             `{"user-service":"user-service.default.svc.cluster.local:8080"}`,
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envRouteUpstreams) {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), envRouteUpstreams)
	}
}

func TestLoadEnvConfigReturnsErrorWhenServiceNameEmpty(t *testing.T) {
	resetEnv(t)

	envPath := writeEnvFile(t, map[string]string{
		envRouteServiceHeader:          "X-Wintergate-Service",
		envRouteUpstreamRequestTimeout: "3s",
		envRouteUpstreams:             `{"":"http://user-service.default.svc.cluster.local:8080"}`,
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envRouteUpstreams) {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), envRouteUpstreams)
	}
}

func resetEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		envRouteServiceHeader,
		envRouteUpstreamRequestTimeout,
		envRouteUpstreams,
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
