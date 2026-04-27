package config

import "encoding/json"

// Settings 외부에서 전달하는 Wintergate 설정 정보입니다.
type Settings struct {
	Auth      *AuthSettings       `json:"auth,omitempty"`
	Routes    *RouteSettings      `json:"routes,omitempty"`
	RateLimit []RateLimitSettings `json:"rate_limit,omitempty"`
}

// AuthSettings 인증 관련 설정 정보입니다.
type AuthSettings struct {
	JWTAlgorithm string          `json:"jwt_algorithm"`
	JWTAudience  string          `json:"jwt_audience"`
	JWTClockSkew string          `json:"jwt_clock_skew"`
	JWTIssuer    string          `json:"jwt_issuer"`
	JWTSecret    string          `json:"jwt_secret,omitempty"`
	JWKS         json.RawMessage `json:"jwks"`
}

// RouteSettings 라우팅 관련 설정 정보입니다.
type RouteSettings struct {
	Protected []ProtectedRoute `json:"protected"`
}

// ProtectedRoute 추가 인가 조건이 필요한 라우트 설정 정보입니다.
type ProtectedRoute struct {
	Route
	Roles        []string      `json:"roles"`
	AccessWindow *AccessWindow `json:"time_window,omitempty"`
}

// RateLimitSettings 요청 제한 규칙 설정 정보입니다.
type RateLimitSettings struct {
	Route
	Roles    []string `json:"roles"`
	Duration string   `json:"duration"`
	Limit    int      `json:"limit"`
}

// AccessWindow 엔드포인트 접근 가능 시간 설정 정보입니다.
type AccessWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// Route 라우트 매칭에 필요한 기본 설정 정보입니다.
type Route struct {
	Path    string `json:"path"`
	Method  string `json:"method"`
	Service string `json:"service"`
}
