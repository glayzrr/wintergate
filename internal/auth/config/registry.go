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
	configs map[string]runtimeConfig

	// mu는 configs map의 동시 조회와 설정 교체를 보호합니다.
	mu sync.RWMutex
}

type runtimeConfig struct {
	algorithm string
	audience  string
	clockSkew time.Duration
	issuer    string
	secret    []byte
	jwks      []byte
	keys      map[string]*rsa.PublicKey
}

// NewRegistry 빈 인증 설정 Registry를 생성합니다.
func NewRegistry() *Registry {
	return &Registry{
		configs: make(map[string]runtimeConfig),
	}
}

// Register 전달받은 기본 인증 런타임 설정과 JWKS로 현재 값을 교체합니다.
func (r *Registry) Register(cfg Config) error {
	return r.RegisterFor("", cfg)
}

// RegisterFor 전달받은 설정 키의 인증 런타임 설정과 JWKS로 현재 값을 교체합니다.
func (r *Registry) RegisterFor(configKey string, cfg Config) error {
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

	r.configs[normalizeConfigKey(configKey)] = runtimeConfig{
		algorithm: cfg.JWTAlgorithm,
		audience:  cfg.JWTAudience,
		clockSkew: cfg.JWTClockSkew,
		issuer:    cfg.JWTIssuer,
		secret:    jwtSecret,
		jwks:      jwks,
		keys:      keys,
	}

	return nil
}

// Snapshot 현재 인증 런타임 설정의 사본을 반환합니다.
func (r *Registry) Snapshot() (Config, bool) {
	return r.SnapshotFor("")
}

// SnapshotFor 지정한 설정 키의 인증 런타임 설정의 사본을 반환합니다.
func (r *Registry) SnapshotFor(configKey string) (Config, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, found := r.configs[normalizeConfigKey(configKey)]
	if !found {
		return Config{}, false
	}

	snapshot := Config{
		JWTAlgorithm: cfg.algorithm,
		JWTAudience:  cfg.audience,
		JWTClockSkew: cfg.clockSkew,
		JWTIssuer:    cfg.issuer,
		JWTSecret:    append([]byte(nil), cfg.secret...),
		JWKS:         append([]byte(nil), cfg.jwks...),
	}

	return snapshot, true
}

// PublicKey 주어진 kid에 해당하는 RSA 공개키를 반환합니다.
func (r *Registry) PublicKey(kid string) (*rsa.PublicKey, error) {
	return r.PublicKeyFor("", kid)
}

// PublicKeyFor 지정한 설정 키의 kid에 해당하는 RSA 공개키를 반환합니다.
func (r *Registry) PublicKeyFor(configKey, kid string) (*rsa.PublicKey, error) {
	trimmedKeyID := strings.TrimSpace(kid)
	if trimmedKeyID == "" {
		return nil, fmt.Errorf("%w: kid is required", ErrInvalidKeyID)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, found := r.configs[normalizeConfigKey(configKey)]
	if !found || len(cfg.keys) == 0 {
		return nil, fmt.Errorf("%w: no registered keys", ErrKeySetUnavailable)
	}

	publicKey, found := cfg.keys[trimmedKeyID]
	if !found {
		return nil, fmt.Errorf("%w: kid %q", ErrKeyNotFound, trimmedKeyID)
	}

	return publicKey, nil
}
