package config

import (
	"crypto/rsa"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Registry 인증 런타임 설정과 JWKS를 메모리에 보관합니다.
type Registry struct {
	mu         sync.RWMutex
	algorithm  string
	audience   string
	clockSkew  time.Duration
	issuer     string
	secret     []byte
	jwks       []byte
	keys       map[string]*rsa.PublicKey
	set        bool
}

// NewRegistry 빈 인증 설정 Registry를 생성합니다.
func NewRegistry() *Registry {
	return &Registry{
		keys: make(map[string]*rsa.PublicKey),
	}
}

// Register 전달받은 인증 런타임 설정과 JWKS로 현재 값을 교체합니다.
func (r *Registry) Register(cfg RuntimeConfig) error {
	if strings.TrimSpace(cfg.JWTAlgorithm) == "" {
		return fmt.Errorf("%w: jwt_algorithm is required", ErrInvalidConfig)
	}

	if strings.TrimSpace(cfg.JWTAudience) == "" {
		return fmt.Errorf("%w: jwt_audience is required", ErrInvalidConfig)
	}

	if cfg.JWTClockSkew <= 0 {
		return fmt.Errorf("%w: jwt_clock_skew must be greater than zero", ErrInvalidConfig)
	}

	if strings.TrimSpace(cfg.JWTIssuer) == "" {
		return fmt.Errorf("%w: jwt_issuer is required", ErrInvalidConfig)
	}

	jwtSecret := append([]byte(nil), cfg.JWTSecret...)
	jwks := append([]byte(nil), cfg.JWKS...)
	var keys map[string]*rsa.PublicKey

	switch cfg.JWTAlgorithm {
	case supportedJWTAlgorithm:
		if len(jwks) == 0 {
			return fmt.Errorf("%w: jwks is required", ErrInvalidConfig)
		}

		keyDocument, err := newDocumentFromBytes(jwks)
		if err != nil {
			return fmt.Errorf("decode jwks: %w", err)
		}

		keys, err = keyDocument.publicKeys()
		if err != nil {
			return fmt.Errorf("parse jwks: %w", err)
		}
	case supportedHMACJWTAlgorithm:
		if strings.TrimSpace(string(jwtSecret)) == "" {
			return fmt.Errorf("%w: jwt_secret is required", ErrInvalidConfig)
		}

		jwks = nil
		keys = make(map[string]*rsa.PublicKey)
	default:
		return fmt.Errorf("%w: unsupported jwt_algorithm %q", ErrInvalidConfig, cfg.JWTAlgorithm)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.algorithm = cfg.JWTAlgorithm
	r.audience = cfg.JWTAudience
	r.clockSkew = cfg.JWTClockSkew
	r.issuer = cfg.JWTIssuer
	r.secret = jwtSecret
	r.jwks = jwks
	r.keys = keys
	r.set = true

	return nil
}

// Snapshot 현재 인증 런타임 설정의 사본을 반환합니다.
func (r *Registry) Snapshot() (RuntimeConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.set {
		return RuntimeConfig{}, false
	}

	cfg := RuntimeConfig{
		JWTAlgorithm: r.algorithm,
		JWTAudience:  r.audience,
		JWTClockSkew: r.clockSkew,
		JWTIssuer:    r.issuer,
		JWTSecret:    append([]byte(nil), r.secret...),
		JWKS:         append([]byte(nil), r.jwks...),
	}

	return cfg, true
}

// PublicKey 주어진 kid에 해당하는 RSA 공개키를 반환합니다.
func (r *Registry) PublicKey(kid string) (*rsa.PublicKey, error) {
	trimmedKeyID := strings.TrimSpace(kid)
	if trimmedKeyID == "" {
		return nil, fmt.Errorf("%w: kid is required", ErrInvalidKeyID)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.set || len(r.keys) == 0 {
		return nil, fmt.Errorf("%w: no registered keys", ErrKeySetUnavailable)
	}

	publicKey, found := r.keys[trimmedKeyID]
	if !found {
		return nil, fmt.Errorf("%w: kid %q", ErrKeyNotFound, trimmedKeyID)
	}

	return publicKey, nil
}
