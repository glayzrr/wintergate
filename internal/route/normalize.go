package route

import "strings"

func NormalizeConfigKey(configKey string) string {
	return strings.ToLower(strings.TrimSpace(configKey))
}

func NormalizeHTTPMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

func NormalizeHTTPPath(path string) string {
	return strings.TrimSpace(path)
}

func NormalizeRoles(roles []string) ([]string, bool) {
	if len(roles) == 0 {
		return nil, true
	}

	normalized := make([]string, 0, len(roles))
	for _, role := range roles {
		trimmedRole := strings.TrimSpace(role)
		if trimmedRole == "" {
			return nil, false
		}

		normalized = append(normalized, trimmedRole)
	}

	return normalized, true
}
