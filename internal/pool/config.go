package pool

import (
	"fmt"
	"strings"
	"time"
)

// Tier 트래픽 규모별 커넥션 풀 설정 단계를 표현합니다.
type Tier string

const (
	// TierNormal 일반 트래픽용 기본 풀 설정입니다.
	TierNormal Tier = "normal"
	// TierHot 트래픽이 몰리는 서비스용 풀 설정입니다.
	TierHot Tier = "hot"
	// TierSuper 매우 높은 트래픽을 받는 서비스용 풀 설정입니다.
	TierSuper Tier = "super"
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
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   20,
		MaxConnsPerHost:       50,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	TierHot: {
		Tier:                  TierHot,
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   100,
		MaxConnsPerHost:       300,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	TierSuper: {
		Tier:                  TierSuper,
		MaxIdleConns:          3000,
		MaxIdleConnsPerHost:   300,
		MaxConnsPerHost:       800,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
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
