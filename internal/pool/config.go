package pool

import (
	"fmt"
	"strings"
	"time"
)

// Config http.Transport 커넥션 풀 관련 설정입니다.
type Config struct {
	Tier                  Tier
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	MaxConnsPerHost       int
	IdleConnTimeout       time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
}

var (
	defaultTier    Tier
	defaultConfigs = map[Tier]Config{
		TierNormal: {
			Tier:                  TierNormal,
			MaxIdleConns:          1024,
			MaxIdleConnsPerHost:   512,
			MaxConnsPerHost:       1024,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 0,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		TierHot: {
			Tier:                  TierHot,
			MaxIdleConns:          2048,
			MaxIdleConnsPerHost:   1024,
			MaxConnsPerHost:       2048,
			IdleConnTimeout:       180 * time.Second,
			ResponseHeaderTimeout: 0,
			TLSHandshakeTimeout:   20 * time.Second,
			ExpectContinueTimeout: 2 * time.Second,
		},
		TierSuper: {
			Tier:                  TierSuper,
			MaxIdleConns:          4096,
			MaxIdleConnsPerHost:   2048,
			MaxConnsPerHost:       4096,
			IdleConnTimeout:       360 * time.Second,
			ResponseHeaderTimeout: 0,
			TLSHandshakeTimeout:   40 * time.Second,
			ExpectContinueTimeout: 4 * time.Second,
		},
	}
)

// Configure 서버 시작 시 읽은 커넥션 풀 설정을 기본 설정으로 반영합니다.
func Configure(configs map[Tier]Config, tier Tier) error {
	if strings.TrimSpace(string(tier)) == "" {
		return fmt.Errorf("%w: default tier is required", ErrInvalidConfig)
	}

	normalizedDefaultTier, err := normalizeTier(tier)
	if err != nil {
		return fmt.Errorf("normalize default tier: %w", err)
	}

	nextConfigs := make(map[Tier]Config, len(configs))
	for tier, config := range configs {
		if strings.TrimSpace(string(tier)) == "" {
			return fmt.Errorf("%w: tier is required", ErrInvalidConfig)
		}

		normalizedTier, err := normalizeTier(tier)
		if err != nil {
			return fmt.Errorf("normalize tier: %w", err)
		}

		config.Tier = normalizedTier
		if err := validateConfig(config); err != nil {
			return fmt.Errorf("validate %s pool config: %w", normalizedTier, err)
		}

		nextConfigs[normalizedTier] = config
	}

	for _, tier := range []Tier{TierNormal, TierHot, TierSuper} {
		if _, found := nextConfigs[tier]; !found {
			return fmt.Errorf("%w: %s tier config is required", ErrInvalidConfig, tier)
		}
	}

	if _, found := nextConfigs[normalizedDefaultTier]; !found {
		return fmt.Errorf("%w: default tier config is required", ErrInvalidConfig)
	}

	defaultTier = normalizedDefaultTier
	defaultConfigs = nextConfigs
	return nil
}

// DefaultTier 설정 파일에서 읽은 기본 공유 풀 티어를 반환합니다.
func DefaultTier() Tier {
	return defaultTier
}

// ConfigFor 지정한 티어의 풀 설정을 반환합니다.
func ConfigFor(tier Tier) (Config, error) {
	normalizedTier, err := normalizeTier(tier)
	if err != nil {
		return Config{}, err
	}

	config, found := defaultConfigs[normalizedTier]
	if !found {
		return Config{}, fmt.Errorf("%w: unsupported tier %q", ErrInvalidConfig, tier)
	}

	return config, nil
}

func validateConfig(config Config) error {
	if config.MaxIdleConns < 0 {
		return fmt.Errorf("%w: max idle connections must be greater than or equal to zero", ErrInvalidConfig)
	}
	if config.MaxIdleConnsPerHost < 0 {
		return fmt.Errorf("%w: max idle connections per host must be greater than or equal to zero", ErrInvalidConfig)
	}
	if config.MaxConnsPerHost < 0 {
		return fmt.Errorf("%w: max connections per host must be greater than or equal to zero", ErrInvalidConfig)
	}
	if config.IdleConnTimeout < 0 {
		return fmt.Errorf("%w: idle connection timeout must be greater than or equal to zero", ErrInvalidConfig)
	}
	if config.ResponseHeaderTimeout < 0 {
		return fmt.Errorf("%w: response header timeout must be greater than or equal to zero", ErrInvalidConfig)
	}
	if config.TLSHandshakeTimeout < 0 {
		return fmt.Errorf("%w: tls handshake timeout must be greater than or equal to zero", ErrInvalidConfig)
	}
	if config.ExpectContinueTimeout < 0 {
		return fmt.Errorf("%w: expect continue timeout must be greater than or equal to zero", ErrInvalidConfig)
	}

	return nil
}
