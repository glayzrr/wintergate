package config

import "encoding/json"

// Settings 외부에서 전달하는 Wintergate 설정 정보입니다.
type Settings struct {
	Global *GlobalSettings `json:"global"`
	Routes []RouteSettings `json:"routes"`
}

// GlobalSettings 모든 서비스에 공통으로 적용하는 설정 정보입니다.
type GlobalSettings struct {
	Auth *AuthSettings `json:"auth"`
}

// AuthSettings 인증 관련 설정 정보입니다.
type AuthSettings struct {
	JWTAlgorithm string          `json:"jwt_algorithm"`
	JWTAudience  string          `json:"jwt_audience"`
	JWTClockSkew string          `json:"jwt_clock_skew"`
	JWTIssuer    string          `json:"jwt_issuer"`
	JWTSecret    string          `json:"jwt_secret"`
	JWKS         json.RawMessage `json:"jwks"`
}

// RouteSettings 하나의 업스트림 서비스와 엔드포인트 정책 설정 정보입니다.
type RouteSettings struct {
	Name      string             `json:"name"`
	Host      string             `json:"host"`
	Port      int                `json:"port"`
	Threshold *ThresholdSettings `json:"threshold"`
	Endpoints []EndpointSettings `json:"endpoints"`
}

// ThresholdSettings 서비스 트래픽 티어 승격 기준 설정 정보입니다.
type ThresholdSettings struct {
	Hot   ThresholdPoint `json:"hot"`
	Super ThresholdPoint `json:"super"`
}

// ThresholdPoint 하나의 트래픽 티어 승격 기준값입니다.
type ThresholdPoint struct {
	RPS      float64 `json:"rps"`
	InFlight int64   `json:"in-flight"`
}

// EndpointSettings 서비스 엔드포인트별 접근 정책 설정 정보입니다.
type EndpointSettings struct {
	Path   string   `json:"path"`
	Method string   `json:"method"`
	Roles  []string `json:"roles"`
}
