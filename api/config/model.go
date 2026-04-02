package configapi

import "encoding/json"

// Snapshot 외부에서 전달하는 Wintergate 설정 스냅샷입니다.
type Snapshot struct {
	Auth    *AuthSection    `json:"auth,omitempty"`
	Routing *RoutingSection `json:"routing,omitempty"`
}

// AuthSection 인증 관련 설정 섹션입니다.
type AuthSection struct {
	JWTAlgorithm string          `json:"jwt_algorithm"`
	JWTAudience  string          `json:"jwt_audience"`
	JWTClockSkew string          `json:"jwt_clock_skew"`
	JWTIssuer    string          `json:"jwt_issuer"`
	JWTSecret    string          `json:"jwt_secret,omitempty"`
	JWKS         json.RawMessage `json:"jwks"`
}

// RoutingSection 라우팅 관련 설정 섹션입니다.
type RoutingSection struct {
	RouteServiceHeader             string  `json:"route_service_header"`
	RouteUpstreamRequestTimeout    string  `json:"route_upstream_request_timeout"`
	Routes                         []Route `json:"routes"`
}

// Route 하나의 URL 경로와 서비스 매핑을 나타냅니다.
type Route struct {
	Path     string `json:"path"`
	Service  string `json:"service"`
	ClientIP string `json:"client_ip,omitempty"`
	Port     int    `json:"port"`
}
