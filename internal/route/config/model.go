package config

import internalconfig "wintergate/internal/config"

// RouteInfo 외부로 반환하는 라우팅 정보를 표현합니다.
type RouteInfo struct {
	ServiceName string
	Path        string
	HttpMethod string
	Roles       []string
	Instance    internalconfig.InstanceSettings
}
