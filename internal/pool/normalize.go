package pool

import (
	"fmt"
	"strings"
)

func normalizeConfigKey(configKey string) string {
	return strings.ToLower(strings.TrimSpace(configKey))
}

func normalizeTier(tier Tier) (Tier, error) {
	normalizedTier := Tier(strings.ToLower(strings.TrimSpace(string(tier))))
	if normalizedTier == "" {
		return TierNormal, nil
	}

	switch normalizedTier {
	case TierNormal, TierHot, TierSuper:
		return normalizedTier, nil
	default:
		return "", fmt.Errorf("%w: unsupported tier %q", ErrInvalidConfig, tier)
	}
}
