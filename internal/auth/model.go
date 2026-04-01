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
