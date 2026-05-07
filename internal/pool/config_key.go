package pool

import "strings"

func normalizeConfigKey(configKey string) string {
	return strings.TrimSpace(configKey)
}
