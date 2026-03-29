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
		envAuthJWKSURL:             "http://auth-service.local/.well-known/jwks.json",
		envAuthJWKSRequestTimeout:  "3s",
		envAuthJWKSRefreshInterval: "10m",
		envJWTAlgorithm:            supportedJWTAlgorithm,
		envJWTAudience:             "wintergate",
		envJWTClockSkew:            "45s",
		envJWTIssuer:               "auth-service",
	})

	cfg, err := LoadEnvConfig(envPath)
	if err != nil {
		t.Fatalf("LoadEnvConfig returned error: %v", err)
	}

	if cfg.AuthJWKSURL != "http://auth-service.local/.well-known/jwks.json" {
		t.Fatalf("AuthJWKSURL = %q, want %q", cfg.AuthJWKSURL, "http://auth-service.local/.well-known/jwks.json")
	}

	if cfg.AuthJWKSRequestTimeout != 3*time.Second {
		t.Fatalf("AuthJWKSRequestTimeout = %s, want %s", cfg.AuthJWKSRequestTimeout, 3*time.Second)
	}

	if cfg.AuthJWKSRefreshInterval != 10*time.Minute {
		t.Fatalf("AuthJWKSRefreshInterval = %s, want %s", cfg.AuthJWKSRefreshInterval, 10*time.Minute)
	}

	if cfg.JWTAlgorithm != supportedJWTAlgorithm {
		t.Fatalf("JWTAlgorithm = %q, want %q", cfg.JWTAlgorithm, supportedJWTAlgorithm)
	}

	if cfg.JWTAudience != "wintergate" {
		t.Fatalf("JWTAudience = %q, want %q", cfg.JWTAudience, "wintergate")
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
		envAuthJWKSURL:             "http://auth-service.local/.well-known/jwks.json",
		envAuthJWKSRequestTimeout:  "3s",
		envAuthJWKSRefreshInterval: "10m",
		envJWTAlgorithm:            supportedJWTAlgorithm,
		envJWTAudience:             "wintergate",
		envJWTClockSkew:            "45s",
		envJWTIssuer:               "auth-service",
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
		envAuthJWKSURL:             "http://auth-service.local/.well-known/jwks.json",
		envAuthJWKSRequestTimeout:  "3s",
		envAuthJWKSRefreshInterval: "10m",
		envJWTAlgorithm:            supportedJWTAlgorithm,
		envJWTClockSkew:            "45s",
		envJWTIssuer:               "auth-service",
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
		envAuthJWKSURL:             "http://auth-service.local/.well-known/jwks.json",
		envAuthJWKSRequestTimeout:  "not-a-duration",
		envAuthJWKSRefreshInterval: "10m",
		envJWTAlgorithm:            supportedJWTAlgorithm,
		envJWTAudience:             "wintergate",
		envJWTClockSkew:            "45s",
		envJWTIssuer:               "auth-service",
	})

	_, err := LoadEnvConfig(envPath)
	if err == nil {
		t.Fatal("LoadEnvConfig returned nil error")
	}

	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("error = %v, want ErrInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), envAuthJWKSRequestTimeout) {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), envAuthJWKSRequestTimeout)
	}
}

func TestLoadEnvConfigReturnsErrorWhenAlgorithmUnsupported(t *testing.T) {
	envPath := writeEnvFile(t, map[string]string{
		envAuthJWKSURL:             "http://auth-service.local/.well-known/jwks.json",
		envAuthJWKSRequestTimeout:  "3s",
		envAuthJWKSRefreshInterval: "10m",
		envJWTAlgorithm:            "HS256",
		envJWTAudience:             "wintergate",
		envJWTClockSkew:            "45s",
		envJWTIssuer:               "auth-service",
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
