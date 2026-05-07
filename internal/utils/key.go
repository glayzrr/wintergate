package utils

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ConfigKey host와 port를 런타임 설정 조회에 쓰는 공통 키로 정규화합니다.
func ConfigKey(host, port string) (string, error) {
	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		return "", fmt.Errorf("host is required")
	}

	trimmedPort := strings.TrimSpace(port)
	if trimmedPort == "" {
		return "", fmt.Errorf("port is required")
	}

	parsedPort, err := strconv.Atoi(trimmedPort)
	if err != nil {
		return "", fmt.Errorf("parse port: %w", err)
	}
	if parsedPort <= 0 || parsedPort > 65535 {
		return "", fmt.Errorf("port is invalid")
	}

	if strings.Contains(trimmedHost, "://") || strings.ContainsAny(trimmedHost, "/?#@") {
		return "", fmt.Errorf("host must not include scheme, path, query, or user info")
	}

	return net.JoinHostPort(strings.ToLower(trimmedHost), strconv.Itoa(parsedPort)), nil
}
