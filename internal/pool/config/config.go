package config

import (
	"fmt"
	"time"

	"wintergate/internal/utils"
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
	sharedConfig Config
	tierConfigs  map[Tier]Config
	configured   bool
)

// Configure 서버 시작 시 읽은 공유풀과 티어별 풀 설정을 런타임 설정으로 반영합니다.
func Configure(shared Config, tiers map[Tier]Config) error {
	shared.Tier = ""
	if err := validateConfig(shared); err != nil {
		return fmt.Errorf("validate shared pool config: %w", err)
	}

	nextTierConfigs := make(map[Tier]Config, len(tiers))
	for tier, config := range tiers {
		if utils.NormalizeTrimmed(string(tier)) == "" {
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

		nextTierConfigs[normalizedTier] = config
	}

	for _, tier := range []Tier{TierNormal, TierHot, TierSuper} {
		if _, found := nextTierConfigs[tier]; !found {
			return fmt.Errorf("%w: %s tier config is required", ErrInvalidConfig, tier)
		}
	}

	sharedConfig = shared
	tierConfigs = nextTierConfigs
	configured = true
	return nil
}

// SharedConfig 공유풀 설정을 반환합니다.
func SharedConfig() (Config, error) {
	if !configured {
		return Config{}, fmt.Errorf("%w: pool config is not loaded", ErrInvalidConfig)
	}

	return sharedConfig, nil
}

// ConfigFor 지정한 티어의 풀 설정을 반환합니다.
func ConfigFor(tier Tier) (Config, error) {
	if !configured {
		return Config{}, fmt.Errorf("%w: pool config is not loaded", ErrInvalidConfig)
	}

	normalizedTier, err := normalizeTier(tier)
	if err != nil {
		return Config{}, err
	}

	config, found := tierConfigs[normalizedTier]
	if !found {
		return Config{}, fmt.Errorf("%w: unsupported tier %q", ErrInvalidConfig, tier)
	}

	return config, nil
}

func normalizeTier(tier Tier) (Tier, error) {
	normalizedTier, ok := utils.NormalizeEnum(
		string(tier),
		"",
		string(TierNormal),
		string(TierHot),
		string(TierSuper),
	)
	if !ok {
		return "", fmt.Errorf("%w: unsupported tier %q", ErrInvalidConfig, tier)
	}

	return Tier(normalizedTier), nil
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
