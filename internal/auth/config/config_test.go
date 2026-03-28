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
	envPath := writeEnvFile(t, map[string]string{
		envAuthPublicKeyURL:             "http://auth-service.local/public.pem",
		envAuthPublicKeyRequestTimeout:  "3s",
		envAuthPublicKeyRefreshInterval: "10m",
		envJWTAlgorithm:                 supportedJWTAlgorithm,
		envJWTAudience:                  "sidecargo",
		envJWTClockSkew:                 "45s",
		envJWTIssuer:                    "auth-service",
	})

	cfg, err := LoadEnvConfig(envPath)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}

	if cfg.AuthPublicKeyURL != "http://auth-service.local/public.pem" {
		t.Fatalf("AuthPublicKeyURL = %q, want %q", cfg.AuthPublicKeyURL, "http://auth-service.local/public.pem")
	}

	if cfg.AuthPublicKeyRequestTimeout != 3*time.Second {
		t.Fatalf("AuthPublicKeyRequestTimeout = %s, want %s", cfg.AuthPublicKeyRequestTimeout, 3*time.Second)
	}

	if cfg.AuthPublicKeyRefreshInterval != 10*time.Minute {
		t.Fatalf("AuthPublicKeyRefreshInterval = %s, want %s", cfg.AuthPublicKeyRefreshInterval, 10*time.Minute)
	}

	if cfg.JWTAlgorithm != supportedJWTAlgorithm {
		t.Fatalf("JWTAlgorithm = %q, want %q", cfg.JWTAlgorithm, supportedJWTAlgorithm)
	}

	if cfg.JWTAudience != "sidecargo" {
		t.Fatalf("JWTAudience = %q, want %q", cfg.JWTAudience, "sidecargo")
	}

	if cfg.JWTClockSkew != 45*time.Second {
		t.Fatalf("JWTClockSkew = %s, want %s", cfg.JWTClockSkew, 45*time.Second)
	}

	if cfg.JWTIssuer != "auth-service" {
		t.Fatalf("JWTIssuer = %q, want %q", cfg.JWTIssuer, "auth-service")
	}
}

func TestLoadEnvConfigPrefersProcessEnv(t *testing.T) {
	envPath := writeEnvFile(t, map[string]string{
		envAuthPublicKeyURL:             "http://auth-service.local/public.pem",
		envAuthPublicKeyRequestTimeout:  "3s",
		envAuthPublicKeyRefreshInterval: "10m",
		envJWTAlgorithm:                 supportedJWTAlgorithm,
		envJWTAudience:                  "sidecargo",
		envJWTClockSkew:                 "45s",
		envJWTIssuer:                    "auth-service",
	})

	t.Setenv(envJWTIssuer, "override-issuer")

	cfg, err := LoadEnvConfig(envPath)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}

	if cfg.JWTIssuer != "override-issuer" {
		t.Fatalf("JWTIssuer = %q, want %q", cfg.JWTIssuer, "override-issuer")
	}
}

func TestLoadEnvConfigReturnsErrorWhenRequiredKeyMissing(t *testing.T) {
	envPath := writeEnvFile(t, map[string]string{
		envAuthPublicKeyURL:             "http://auth-service.local/public.pem",
		envAuthPublicKeyRequestTimeout:  "3s",
		envAuthPublicKeyRefreshInterval: "10m",
		envJWTAlgorithm:                 supportedJWTAlgorithm,
		envJWTClockSkew:                 "45s",
		envJWTIssuer:                    "auth-service",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envJWTAudience) {
		t.Fatalf("error = %q, want missing key %q in message", err.Error(), envJWTAudience)
	}
}

func TestLoadEnvConfigReturnsErrorWhenDurationInvalid(t *testing.T) {
	envPath := writeEnvFile(t, map[string]string{
		envAuthPublicKeyURL:             "http://auth-service.local/public.pem",
		envAuthPublicKeyRequestTimeout:  "not-a-duration",
		envAuthPublicKeyRefreshInterval: "10m",
		envJWTAlgorithm:                 supportedJWTAlgorithm,
		envJWTAudience:                  "sidecargo",
		envJWTClockSkew:                 "45s",
		envJWTIssuer:                    "auth-service",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envAuthPublicKeyRequestTimeout) {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), envAuthPublicKeyRequestTimeout)
	}
}

func TestLoadEnvConfigReturnsErrorWhenAlgorithmUnsupported(t *testing.T) {
	envPath := writeEnvFile(t, map[string]string{
		envAuthPublicKeyURL:             "http://auth-service.local/public.pem",
		envAuthPublicKeyRequestTimeout:  "3s",
		envAuthPublicKeyRefreshInterval: "10m",
		envJWTAlgorithm:                 "HS256",
		envJWTAudience:                  "sidecargo",
		envJWTClockSkew:                 "45s",
		envJWTIssuer:                    "auth-service",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envJWTAlgorithm) {
		t.Fatalf("error = %q, want unsupported key %q in message", err.Error(), envJWTAlgorithm)
	}
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
