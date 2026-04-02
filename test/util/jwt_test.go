package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// HS256Token 기본 claims가 포함된 HS256 JWT를 생성합니다.
func HS256Token(secret string) (string, error) {
	now := time.Now().UTC()

	return HS256TokenWithClaims(secret, map[string]any{
		"aud": "wintergate",
		"exp": now.Add(time.Minute).Unix(),
		"iat": now.Unix(),
		"iss": "auth-service",
		"sub": "user-1",
	})
}

// HS256TokenWithClaims 전달받은 claims로 HS256 JWT를 생성합니다.
func HS256TokenWithClaims(secret string, claims map[string]any) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("secret is required")
	}

	headerPayload, err := json.Marshal(map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}

	claimsPayload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerPayload) + "." + base64.RawURLEncoding.EncodeToString(claimsPayload)

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(signingInput)); err != nil {
		return "", fmt.Errorf("hash signing input: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func TestHS256Token(t *testing.T) {
	secret := "secret"
	token, _ := HS256Token(secret)
	fmt.Println(token)
}
