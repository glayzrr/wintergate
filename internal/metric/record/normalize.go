package record

import (
	"net/http"
	"strings"
)

func normalizeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodGet
	}

	return method
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return routeGateway
	}

	return path
}

func normalizeStatusCode(statusCode int) int {
	if statusCode == 0 {
		return http.StatusOK
	}

	return statusCode
}

func normalizeMetricValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return unknown
	}

	return value
}
