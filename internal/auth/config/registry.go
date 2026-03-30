package config

import (
	"crypto/rsa"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RuntimeConfig 인증 런타임 설정과 JWKS 값을 보관합니다.
type RuntimeConfig struct {
	JWTAlgorithm string
	JWTAudience  string
	JWTClockSkew time.Duration
	JWTIssuer    string
	JWKS         []byte
}

// Registry 인증 런타임 설정과 JWKS를 메모리에 보관합니다.
type Registry struct {
	mu     sync.RWMutex
	config RuntimeConfig
	keys   map[string]*rsa.PublicKey
	set    bool
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

	if cfg.JWTAlgorithm != supportedJWTAlgorithm {
		return fmt.Errorf("%w: unsupported jwt_algorithm %q", ErrInvalidConfig, cfg.JWTAlgorithm)
	}

	if len(cfg.JWKS) == 0 {
		return fmt.Errorf("%w: jwks is required", ErrInvalidConfig)
	}

	jwks := append([]byte(nil), cfg.JWKS...)
	keyDocument, err := newDocumentFromBytes(jwks)
	if err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys, err := keyDocument.publicKeys()
	if err != nil {
		return fmt.Errorf("parse jwks: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	cfg.JWKS = jwks
	r.config = cfg
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

	cfg := r.config
	cfg.JWKS = append([]byte(nil), r.config.JWKS...)

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
