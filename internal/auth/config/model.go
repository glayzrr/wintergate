package config

import "time"

// Config 인증 런타임 설정과 JWKS 값을 보관합니다.
type Config struct {
	JWTAlgorithm string
	JWTAudience  string
	JWTClockSkew time.Duration
	JWTIssuer    string
	JWTSecret    []byte
	JWKS         []byte
}

type document struct {
	Keys []key `json:"keys"`
}

type key struct {
	Algorithm string `json:"alg"`
	Exponent  string `json:"e"`
	KeyID     string `json:"kid"`
	KeyType   string `json:"kty"`
	Modulus   string `json:"n"`
	Use       string `json:"use"`
}
