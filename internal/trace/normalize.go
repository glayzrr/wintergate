package trace

import "strings"

// NormalizeID 요청 ID 헤더에 사용할 수 있는 값으로 정리합니다.
func NormalizeID(id string) (string, bool) {
	normalizedID := strings.TrimSpace(id)
	if normalizedID == "" || len(normalizedID) > maxRequestIDLength {
		return "", false
	}
	if strings.ContainsAny(normalizedID, "\r\n") {
		return "", false
	}

	return normalizedID, true
}
