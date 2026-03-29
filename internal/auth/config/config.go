package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"sidecargo/internal/utils"
)

// EnvConfig 환경 파일에서 읽은 인증 설정을 보관합니다.
type EnvConfig struct {
	AuthJWKSURL             string
	AuthJWKSRequestTimeout  time.Duration
	AuthJWKSRefreshInterval time.Duration
	JWTAlgorithm            string
	JWTAudience             string
	JWTClockSkew            time.Duration
	JWTIssuer               string
}

// LoadEnvConfig 지정한 환경 파일에서 인증 설정을 읽어옵니다.
func LoadEnvConfig(path string) (EnvConfig, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultEnvPath
	}

	if err := godotenv.Load(path); err != nil {
		return EnvConfig{}, fmt.Errorf("%w: load %s: %v", ErrInvalidConfig, path, err)
	}

	authJWKSURL, err := utils.RequireString(envAuthJWKSURL, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	jwtIssuer, err := utils.RequireString(envJWTIssuer, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	jwtAudience, err := utils.RequireString(envJWTAudience, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	jwtAlgorithm, err := utils.RequireString(envJWTAlgorithm, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	authJWKSRequestTimeout, err := utils.RequireDuration(envAuthJWKSRequestTimeout, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	authJWKSRefreshInterval, err := utils.RequireDuration(envAuthJWKSRefreshInterval, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	jwtClockSkew, err := utils.RequireDuration(envJWTClockSkew, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	cfg := EnvConfig{
		AuthJWKSURL:             authJWKSURL,
		JWTIssuer:               jwtIssuer,
		JWTAudience:             jwtAudience,
		JWTAlgorithm:            jwtAlgorithm,
		AuthJWKSRequestTimeout:  authJWKSRequestTimeout,
		AuthJWKSRefreshInterval: authJWKSRefreshInterval,
		JWTClockSkew:            jwtClockSkew,
	}

	if cfg.JWTAlgorithm != supportedJWTAlgorithm {
		return EnvConfig{}, fmt.Errorf("%w: unsupported %s %q", ErrInvalidConfig, envJWTAlgorithm, cfg.JWTAlgorithm)
	}

	return cfg, nil
}
