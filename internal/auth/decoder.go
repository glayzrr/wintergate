package auth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	authconfig "wintergate/internal/auth/config"
)

const (
	jwtAlgorithmHS256 = "HS256"
	jwtAlgorithmRS256 = "RS256"
)

// Decoder 인증 설정을 바탕으로 JWT를 검증하고 claims를 추출합니다.
type Decoder struct {
	registry *authconfig.Registry
	now      func() time.Time
}

// NewDecoder JWT 검증용 Decoder를 생성합니다.
func NewDecoder(registry *authconfig.Registry) *Decoder {
	return &Decoder{
		registry: registry,
		now:      time.Now,
	}
}

// ReplaceRegistry Decoder가 사용할 인증 설정 저장소를 교체합니다.
func (d *Decoder) ReplaceRegistry(registry *authconfig.Registry) error {
	if registry == nil {
		return fmt.Errorf("%w: registry is required", ErrNilRegistry)
	}

	d.registry = registry

	return nil
}

// BearerToken Authorization 헤더에서 Bearer 토큰 값을 추출합니다.
func BearerToken(authorizationHeader string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(authorizationHeader))
	if len(fields) != 2 {
		return "", fmt.Errorf("%w: bearer token is required", ErrInvalidAuthorizationHeader)
	}

	if !strings.EqualFold(fields[0], "Bearer") {
		return "", fmt.Errorf("%w: bearer scheme is required", ErrInvalidAuthorizationHeader)
	}

	return fields[1], nil
}

// Decode JWT의 서명과 표준 claims를 검증한 뒤 결과를 반환합니다.
func (d *Decoder) Decode(token string) (Claims, error) {
	return d.DecodeFor("", token)
}

// DecodeFor 지정한 설정 키의 인증 설정으로 JWT를 검증하고 claims를 반환합니다.
func (d *Decoder) DecodeFor(configKey, token string) (Claims, error) {
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return Claims{}, fmt.Errorf("%w: token is required", ErrInvalidToken)
	}

	cfg, found := d.registry.SnapshotFor(configKey)
	if !found {
		return Claims{}, fmt.Errorf("%w: auth config is not registered", ErrConfigUnavailable)
	}

	headerPayload, payload, signature, err := splitToken(trimmedToken)
	if err != nil {
		return Claims{}, err
	}

	header, err := decodeTokenHeader(headerPayload.header)
	if err != nil {
		return Claims{}, err
	}

	if err := d.verifySignature(context.Background(), configKey, cfg, header, headerPayload.signingInput, signature); err != nil {
		return Claims{}, err
	}

	claims, err := decodeClaims(payload)
	if err != nil {
		return Claims{}, err
	}

	if err := d.validateClaims(cfg, claims); err != nil {
		return Claims{}, err
	}

	return claims.Claims, nil
}

type tokenPayload struct {
	header       []byte
	signingInput string
}

func splitToken(token string) (tokenPayload, []byte, []byte, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return tokenPayload{}, nil, nil, fmt.Errorf("%w: token must have three parts", ErrInvalidToken)
	}

	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return tokenPayload{}, nil, nil, fmt.Errorf("%w: decode token header: %w", ErrInvalidToken, err)
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenPayload{}, nil, nil, fmt.Errorf("%w: decode token payload: %w", ErrInvalidToken, err)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return tokenPayload{}, nil, nil, fmt.Errorf("%w: decode token signature: %w", ErrInvalidToken, err)
	}

	return tokenPayload{
		header:       header,
		signingInput: parts[0] + "." + parts[1],
	}, payload, signature, nil
}

func decodeTokenHeader(payload []byte) (tokenHeader, error) {
	var header tokenHeader
	if err := json.Unmarshal(payload, &header); err != nil {
		return tokenHeader{}, fmt.Errorf("%w: decode token header: %w", ErrInvalidToken, err)
	}

	if strings.TrimSpace(header.Algorithm) == "" {
		return tokenHeader{}, fmt.Errorf("%w: alg is required", ErrInvalidToken)
	}

	return header, nil
}

func decodeClaims(payload []byte) (decodedClaims, error) {
	var rawClaims map[string]any
	if err := json.Unmarshal(payload, &rawClaims); err != nil {
		return decodedClaims{}, fmt.Errorf("%w: decode token claims: %w", ErrInvalidToken, err)
	}

	var claims claimsPayload
	if err := json.Unmarshal(payload, &claims); err != nil {
		return decodedClaims{}, fmt.Errorf("%w: decode standard claims: %w", ErrInvalidToken, err)
	}

	return decodedClaims{
		Claims: Claims{
			Subject:   claims.Subject,
			Issuer:    claims.Issuer,
			Audience:  []string(claims.Audience),
			Roles:     append([]string(nil), claims.Roles...),
			ExpiresAt: claims.ExpiresAt.time,
			IssuedAt:  claims.IssuedAt.time,
			NotBefore: claims.NotBefore.time,
			Raw:       rawClaims,
		},
		hasExpiresAt: claims.ExpiresAt.set,
		hasIssuedAt:  claims.IssuedAt.set,
		hasNotBefore: claims.NotBefore.set,
	}, nil
}

func (d *Decoder) verifySignature(_ context.Context, configKey string, cfg authconfig.Config, header tokenHeader, signingInput string, signature []byte) error {
	if header.Algorithm != cfg.JWTAlgorithm {
		return fmt.Errorf("%w: expected %q, got %q", ErrUnsupportedAlgorithm, cfg.JWTAlgorithm, header.Algorithm)
	}

	switch cfg.JWTAlgorithm {
	case jwtAlgorithmHS256:
		mac := hmac.New(sha256.New, cfg.JWTSecret)
		if _, err := mac.Write([]byte(signingInput)); err != nil {
			return fmt.Errorf("hash signing input: %w", err)
		}

		if !hmac.Equal(mac.Sum(nil), signature) {
			return fmt.Errorf("%w: hs256 signature mismatch", ErrInvalidSignature)
		}

		return nil
	case jwtAlgorithmRS256:
		trimmedKeyID := strings.TrimSpace(header.KeyID)
		if trimmedKeyID == "" {
			return fmt.Errorf("%w: kid is required", ErrInvalidToken)
		}

		publicKey, err := d.registry.PublicKeyFor(configKey, trimmedKeyID)
		if err != nil {
			return fmt.Errorf("load public key: %w", err)
		}

		return verifyRS256Signature(publicKey, signingInput, signature)
	default:
		return fmt.Errorf("%w: %q", ErrUnsupportedAlgorithm, cfg.JWTAlgorithm)
	}
}

func verifyRS256Signature(publicKey *rsa.PublicKey, signingInput string, signature []byte) error {
	hashed := sha256.Sum256([]byte(signingInput))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signature); err != nil {
		return fmt.Errorf("%w: verify rs256 signature: %w", ErrInvalidSignature, err)
	}

	return nil
}

func (d *Decoder) validateClaims(cfg authconfig.Config, claims decodedClaims) error {
	if strings.TrimSpace(claims.Issuer) == "" {
		return fmt.Errorf("%w: iss is required", ErrInvalidIssuer)
	}

	if claims.Issuer != cfg.JWTIssuer {
		return fmt.Errorf("%w: expected %q, got %q", ErrInvalidIssuer, cfg.JWTIssuer, claims.Issuer)
	}

	if !containsAudience(claims.Audience, cfg.JWTAudience) {
		return fmt.Errorf("%w: audience %q is required", ErrInvalidAudience, cfg.JWTAudience)
	}

	if !claims.hasExpiresAt {
		return fmt.Errorf("%w: exp is required", ErrInvalidToken)
	}

	currentTime := d.now()
	if currentTime.After(claims.ExpiresAt.Add(cfg.JWTClockSkew)) {
		return fmt.Errorf("%w: exp %s", ErrTokenExpired, claims.ExpiresAt.UTC().Format(time.RFC3339))
	}

	if claims.hasNotBefore && currentTime.Before(claims.NotBefore.Add(-cfg.JWTClockSkew)) {
		return fmt.Errorf("%w: nbf %s", ErrTokenNotYetValid, claims.NotBefore.UTC().Format(time.RFC3339))
	}

	if claims.hasIssuedAt && currentTime.Before(claims.IssuedAt.Add(-cfg.JWTClockSkew)) {
		return fmt.Errorf("%w: iat %s", ErrTokenNotYetValid, claims.IssuedAt.UTC().Format(time.RFC3339))
	}

	return nil
}

func containsAudience(audiences []string, expected string) bool {
	for _, audience := range audiences {
		if audience == expected {
			return true
		}
	}

	return false
}

func (a *audienceClaim) UnmarshalJSON(payload []byte) error {
	if bytes.Equal(payload, []byte("null")) {
		*a = nil
		return nil
	}

	var audience string
	if err := json.Unmarshal(payload, &audience); err == nil {
		*a = []string{audience}
		return nil
	}

	var audiences []string
	if err := json.Unmarshal(payload, &audiences); err == nil {
		*a = audiences
		return nil
	}

	return fmt.Errorf("aud must be string or string array")
}

func (n *numericDate) UnmarshalJSON(payload []byte) error {
	if bytes.Equal(payload, []byte("null")) {
		*n = numericDate{}
		return nil
	}

	var rawNumber json.Number
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&rawNumber); err != nil {
		return fmt.Errorf("numeric date must be a number: %w", err)
	}

	seconds, err := strconv.ParseFloat(rawNumber.String(), 64)
	if err != nil {
		return fmt.Errorf("parse numeric date: %w", err)
	}

	wholeSeconds, fractionalSeconds := math.Modf(seconds)
	n.time = time.Unix(int64(wholeSeconds), int64(fractionalSeconds*float64(time.Second))).UTC()
	n.set = true

	return nil
}
