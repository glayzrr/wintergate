package configapi

import "encoding/json"

// Snapshot 외부에서 전달하는 Wintergate 설정 스냅샷입니다.
type Snapshot struct {
	Auth      *AuthSection        `json:"auth,omitempty"`
	Routes    *RoutesSection      `json:"routes,omitempty"`
	RateLimit []RateLimitEndpoint `json:"rate_limit,omitempty"`
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

// RoutesSection 보호 엔드포인트 설정을 보관합니다.
type RoutesSection struct {
	Protected []ProtectedEndpoint
}

// ProtectedEndpoint 추가 인가 조건이 필요한 엔드포인트를 표현합니다.
type ProtectedEndpoint struct {
	Endpoint
	Roles      []string    `json:"roles"`
	TimeWindow *TimeWindow `json:"time_window,omitempty"`
}

// RateLimitEndpoint 요청 제한 규칙이 적용되는 엔드포인트를 표현합니다.
type RateLimitEndpoint struct {
	Endpoint
	Roles    []string `json:"roles"`
	Duration string   `json:"duration"`
	Limit    int      `json:"limit"`
}

// TimeWindow 엔드포인트 접근 가능 시간을 표현합니다.
type TimeWindow struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// Endpoint 라우트 매칭에 필요한 기본 엔드포인트 정보를 표현합니다.
type Endpoint struct {
	Path    string `json:"path"`
	Method  string `json:"method"`
	Service string `json:"service"`
}
