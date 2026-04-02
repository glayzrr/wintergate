package config

import "time"

// Entry 하나의 URL 경로와 대상 서비스 매핑을 표현합니다.
type Entry struct {
	Path     string
	Service  string
	ClientIP string
	Port     int
}

// RuntimeConfig 라우팅 런타임 설정과 엔트리를 함께 보관합니다.
type RuntimeConfig struct {
	RouteServiceHeader          string
	RouteUpstreamRequestTimeout time.Duration
	Entries                     []Entry
}

type RouteInfo struct {
	Service  string
	ClientIP string
	Port     int
}
