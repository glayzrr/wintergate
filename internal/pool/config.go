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

var defaultConfigs = map[Tier]Config{
	TierNormal: {
		Tier:                  TierNormal,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   2,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 0,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	TierHot: {
		Tier:                  TierHot,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   4,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       180 * time.Second,
		ResponseHeaderTimeout: 0,
		TLSHandshakeTimeout:   20 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
	},
	TierSuper: {
		Tier:                  TierSuper,
		MaxIdleConns:          400,
		MaxIdleConnsPerHost:   8,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       360 * time.Second,
		ResponseHeaderTimeout: 0,
		TLSHandshakeTimeout:   40 * time.Second,
		ExpectContinueTimeout: 4 * time.Second,
	},
}

// GetConfig 지정한 티어의 풀 설정을 반환합니다.
func GetConfig(tier Tier) (Config, error) {
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

func normalizeTier(tier Tier) (Tier, error) {
	trimmedTier := strings.ToLower(strings.TrimSpace(string(tier)))
	if trimmedTier == "" {
		return TierNormal, nil
	}

	switch Tier(trimmedTier) {
	case TierNormal, TierHot, TierSuper:
		return Tier(trimmedTier), nil
	default:
		return "", fmt.Errorf("%w: unsupported tier %q", ErrInvalidConfig, tier)
	}
}
