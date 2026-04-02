package auth

import "time"

// Claims JWT에서 추출한 표준 claims와 원본 payload를 함께 보관합니다.
type Claims struct {
	Subject   string
	Issuer    string
	Audience  []string
	ExpiresAt time.Time
	IssuedAt  time.Time
	NotBefore time.Time
	Raw       map[string]any
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
}

type decodedClaims struct {
	Claims
	hasExpiresAt bool
	hasIssuedAt  bool
	hasNotBefore bool
}

type claimsPayload struct {
	Audience  audienceClaim `json:"aud"`
	ExpiresAt numericDate   `json:"exp"`
	IssuedAt  numericDate   `json:"iat"`
	Issuer    string        `json:"iss"`
	NotBefore numericDate   `json:"nbf"`
	Subject   string        `json:"sub"`
}

type audienceClaim []string

type numericDate struct {
	time time.Time
	set  bool
}
