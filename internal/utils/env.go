package utils

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// RequireString 환경 변수 문자열 값을 읽습니다.
func RequireString(key string, baseErr error) (string, error) {
	if envValue := strings.TrimSpace(os.Getenv(key)); envValue != "" {
		return envValue, nil
	}

	return "", fmt.Errorf("%w: %s is required", baseErr, key)
}

// RequireDuration 환경 변수 duration 값을 읽습니다.
func RequireDuration(key string, baseErr error) (time.Duration, error) {
	rawValue, err := RequireString(key, baseErr)
	if err != nil {
		return 0, err
	}

	duration, err := time.ParseDuration(rawValue)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid duration for %s: %v", baseErr, key, err)
	}

	return duration, nil
}
