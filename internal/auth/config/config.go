package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type EnvConfig struct {
	AuthPublicKeyURL             string
	AuthPublicKeyRequestTimeout  time.Duration
	AuthPublicKeyRefreshInterval time.Duration
	JWTAlgorithm                 string
	JWTAudience                  string
	JWTClockSkew                 time.Duration
	JWTIssuer                    string
}

func LoadEnvConfig(path string) (EnvConfig, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultEnvPath
	}

	values, err := godotenv.Read(path)
	if err != nil {
		return EnvConfig{}, fmt.Errorf("%w: read %s: %v", ErrInvalidConfig, path, err)
	}

	authPublicKeyURL, err := requireString(values, envAuthPublicKeyURL)
	if err != nil {
		return EnvConfig{}, err
	}

	jwtIssuer, err := requireString(values, envJWTIssuer)
	if err != nil {
		return EnvConfig{}, err
	}

	jwtAudience, err := requireString(values, envJWTAudience)
	if err != nil {
		return EnvConfig{}, err
	}

	jwtAlgorithm, err := requireString(values, envJWTAlgorithm)
	if err != nil {
		return EnvConfig{}, err
	}

	authPublicKeyRequestTimeout, err := requireDuration(values, envAuthPublicKeyRequestTimeout)
	if err != nil {
		return EnvConfig{}, err
	}

	authPublicKeyRefreshInterval, err := requireDuration(values, envAuthPublicKeyRefreshInterval)
	if err != nil {
		return EnvConfig{}, err
	}

	jwtClockSkew, err := requireDuration(values, envJWTClockSkew)
	if err != nil {
		return EnvConfig{}, err
	}

	cfg := EnvConfig{
		AuthPublicKeyURL:             authPublicKeyURL,
		JWTIssuer:                    jwtIssuer,
		JWTAudience:                  jwtAudience,
		JWTAlgorithm:                 jwtAlgorithm,
		AuthPublicKeyRequestTimeout:  authPublicKeyRequestTimeout,
		AuthPublicKeyRefreshInterval: authPublicKeyRefreshInterval,
		JWTClockSkew:                 jwtClockSkew,
	}

	if cfg.JWTAlgorithm != supportedJWTAlgorithm {
		return EnvConfig{}, fmt.Errorf("%w: unsupported %s %q", ErrInvalidConfig, envJWTAlgorithm, cfg.JWTAlgorithm)
	}

	return cfg, nil
}

func requireString(values map[string]string, key string) (string, error) {
	if envValue := strings.TrimSpace(os.Getenv(key)); envValue != "" {
		return envValue, nil
	}

	if value, ok := values[key]; ok {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return trimmedValue, nil
		}
	}

	return "", fmt.Errorf("%w: %s is required", ErrInvalidConfig, key)
}

func requireDuration(values map[string]string, key string) (time.Duration, error) {
	rawValue, err := requireString(values, key)
	if err != nil {
		return 0, err
	}

	duration, err := time.ParseDuration(rawValue)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid duration for %s: %v", ErrInvalidConfig, key, err)
	}

	return duration, nil
}
