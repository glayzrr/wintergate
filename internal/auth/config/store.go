package config

import (
	"crypto/rsa"
	"fmt"
	"strings"
	"sync"
	"time"

	internalconfig "wintergate/internal/config"
	"wintergate/internal/utils"
)

// Store 인증 런타임 설정과 JWKS를 메모리에 보관합니다.
type Store struct {
	configs map[string]authInfo

	// mu는 configs map의 동시 조회와 설정 교체를 보호합니다.
	mu sync.RWMutex
}

// NewStore 빈 인증 설정 Store를 생성합니다.
func NewStore() *Store {
	return &Store{
		configs: make(map[string]authInfo),
	}
}

// Apply 전달받은 서비스 설정의 auth를 서비스 이름별 인증 설정으로 반영합니다.
func (s *Store) Apply(settings internalconfig.Settings, _, _ string) error {
	if s == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidConfig)
	}
	if settings.Global == nil {
		return fmt.Errorf("%w: global settings is required", ErrInvalidConfig)
	}

	serviceName := utils.NormalizeServiceName(settings.ServiceName)
	if serviceName == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidConfig)
	}

	authInfo, err := buildAuthInfo(settings.Global.Auth)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs[serviceName] = authInfo

	return nil
}

// Snapshot 현재 인증 런타임 설정의 사본을 반환합니다.
func (s *Store) Snapshot() (Config, bool) {
	return s.SnapshotFor("")
}

// SnapshotFor 지정한 설정 키의 인증 런타임 설정의 사본을 반환합니다.
func (s *Store) SnapshotFor(configKey string) (Config, bool) {
	if s == nil {
		return Config{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, found := s.configs[utils.NormalizeServiceName(configKey)]
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
func (s *Store) PublicKey(kid string) (*rsa.PublicKey, error) {
	return s.PublicKeyFor("", kid)
}

// PublicKeyFor 지정한 설정 키의 kid에 해당하는 RSA 공개키를 반환합니다.
func (s *Store) PublicKeyFor(configKey, kid string) (*rsa.PublicKey, error) {
	trimmedKeyID := strings.TrimSpace(kid)
	if trimmedKeyID == "" {
		return nil, fmt.Errorf("%w: kid is required", ErrInvalidKeyID)
	}
	if s == nil {
		return nil, fmt.Errorf("%w: no registered keys", ErrKeySetUnavailable)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	cfg, found := s.configs[utils.NormalizeServiceName(configKey)]
	if !found || len(cfg.keys) == 0 {
		return nil, fmt.Errorf("%w: no registered keys", ErrKeySetUnavailable)
	}

	publicKey, found := cfg.keys[trimmedKeyID]
	if !found {
		return nil, fmt.Errorf("%w: kid %q", ErrKeyNotFound, trimmedKeyID)
	}

	return publicKey, nil
}

func buildAuthInfo(settings *internalconfig.AuthSettings) (authInfo, error) {
	if settings == nil {
		return authInfo{}, fmt.Errorf("%w: auth settings is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(settings.JWTAlgorithm) == "" {
		return authInfo{}, fmt.Errorf("%w: jwt_algorithm is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(settings.JWTAudience) == "" {
		return authInfo{}, fmt.Errorf("%w: jwt_audience is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(settings.JWTIssuer) == "" {
		return authInfo{}, fmt.Errorf("%w: jwt_issuer is required", ErrInvalidConfig)
	}

	authClockSkew, err := time.ParseDuration(settings.JWTClockSkew)
	if err != nil {
		return authInfo{}, fmt.Errorf("%w: parse auth clock skew: %w", ErrInvalidConfig, err)
	}
	if authClockSkew <= 0 {
		return authInfo{}, fmt.Errorf("%w: jwt_clock_skew must be greater than zero", ErrInvalidConfig)
	}

	jwtSecret := []byte(strings.TrimSpace(settings.JWTSecret))
	jwks := append([]byte(nil), settings.JWKS...)
	keys := make(map[string]*rsa.PublicKey)

	switch strings.TrimSpace(settings.JWTAlgorithm) {
	case supportedJWTAlgorithm:
		if len(jwks) == 0 {
			return authInfo{}, fmt.Errorf("%w: jwks is required", ErrInvalidConfig)
		}

		keyDocument, err := newDocumentFromBytes(jwks)
		if err != nil {
			return authInfo{}, fmt.Errorf("decode jwks: %w", err)
		}

		keys, err = keyDocument.publicKeys()
		if err != nil {
			return authInfo{}, fmt.Errorf("parse jwks: %w", err)
		}
	case supportedHMACJWTAlgorithm:
		if len(jwtSecret) == 0 {
			return authInfo{}, fmt.Errorf("%w: jwt_secret is required", ErrInvalidConfig)
		}

		jwks = nil
	default:
		return authInfo{}, fmt.Errorf("%w: unsupported jwt_algorithm %q", ErrInvalidConfig, settings.JWTAlgorithm)
	}

	return authInfo{
		algorithm: strings.TrimSpace(settings.JWTAlgorithm),
		audience:  settings.JWTAudience,
		clockSkew: authClockSkew,
		issuer:    settings.JWTIssuer,
		secret:    jwtSecret,
		jwks:      jwks,
		keys:      keys,
	}, nil
}
