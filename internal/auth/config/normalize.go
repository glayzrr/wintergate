package config

import "strings"

func normalizeConfigKey(configKey string) string {
	return strings.ToLower(strings.TrimSpace(configKey))
}
