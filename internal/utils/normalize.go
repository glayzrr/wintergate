package utils

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// NormalizeTrimmed 문자열 앞뒤 공백을 제거합니다.
func NormalizeTrimmed(value string) string {
	return strings.TrimSpace(value)
}

// NormalizeLower 문자열 앞뒤 공백을 제거하고 소문자로 변환합니다.
func NormalizeLower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// NormalizeUpper 문자열 앞뒤 공백을 제거하고 대문자로 변환합니다.
func NormalizeUpper(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

// NormalizeConfigKey 설정 조회 키를 공통 형식으로 변환합니다.
func NormalizeConfigKey(configKey string) string {
	return NormalizeLower(configKey)
}

// NormalizeServiceName 서비스 이름을 공통 형식으로 변환합니다.
func NormalizeServiceName(serviceName string) string {
	return NormalizeLower(serviceName)
}

// NormalizeHTTPMethod HTTP method를 공통 형식으로 변환합니다.
func NormalizeHTTPMethod(method string) string {
	return NormalizeUpper(method)
}

// NormalizeHTTPPath HTTP path를 공통 형식으로 변환합니다.
func NormalizeHTTPPath(path string) string {
	return NormalizeTrimmed(path)
}

// NormalizeRoles role 목록을 공통 형식으로 변환합니다.
func NormalizeRoles(roles []string) ([]string, bool) {
	if len(roles) == 0 {
		return nil, true
	}

	normalized := make([]string, 0, len(roles))
	for _, role := range roles {
		trimmedRole := NormalizeTrimmed(role)
		if trimmedRole == "" {
			return nil, false
		}

		normalized = append(normalized, trimmedRole)
	}

	return normalized, true
}

// NormalizeHostPort host와 port를 검증하고 정규화한 값을 반환합니다.
func NormalizeHostPort(host, port string) (string, string, error) {
	instanceKey, err := ConfigKey(host, port)
	if err != nil {
		return "", "", err
	}

	normalizedHost, normalizedPort, err := net.SplitHostPort(instanceKey)
	if err != nil {
		return "", "", fmt.Errorf("split instance address: %w", err)
	}

	return normalizedHost, normalizedPort, nil
}

// NormalizeEnum 허용된 값 목록 안에서 문자열을 정규화합니다.
func NormalizeEnum(value, defaultValue string, allowedValues ...string) (string, bool) {
	normalizedValue := NormalizeLower(value)
	if normalizedValue == "" {
		normalizedValue = NormalizeLower(defaultValue)
	}

	for _, allowedValue := range allowedValues {
		if normalizedValue == NormalizeLower(allowedValue) {
			return normalizedValue, true
		}
	}

	return "", false
}

// NormalizeRequestID 요청 ID 헤더에 사용할 수 있는 값으로 정리합니다.
func NormalizeRequestID(id string, maxLength int) (string, bool) {
	normalizedID := NormalizeTrimmed(id)
	if normalizedID == "" {
		return "", false
	}
	if maxLength > 0 && len(normalizedID) > maxLength {
		return "", false
	}
	if strings.ContainsAny(normalizedID, "\r\n") {
		return "", false
	}

	return normalizedID, true
}

// NormalizeMetricMethod 메트릭 label에 사용할 HTTP method를 정리합니다.
func NormalizeMetricMethod(method string) string {
	method = NormalizeHTTPMethod(method)
	if method == "" {
		return http.MethodGet
	}

	return method
}

// NormalizeMetricPath 메트릭 label에 사용할 path를 정리합니다.
func NormalizeMetricPath(path, defaultPath string) string {
	path = NormalizeHTTPPath(path)
	if path == "" {
		return defaultPath
	}

	return path
}

// NormalizeStatusCode 메트릭 label에 사용할 status code를 정리합니다.
func NormalizeStatusCode(statusCode, defaultStatusCode int) int {
	if statusCode == 0 {
		return defaultStatusCode
	}

	return statusCode
}

// NormalizeMetricValue 메트릭 label에 사용할 문자열 값을 정리합니다.
func NormalizeMetricValue(value, defaultValue string) string {
	value = NormalizeTrimmed(value)
	if value == "" {
		return defaultValue
	}

	return value
}
