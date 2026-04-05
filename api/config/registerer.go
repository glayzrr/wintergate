package configapi

import (
	"fmt"
	"strings"
	"time"

	authconfig "wintergate/internal/auth/config"
)

// Registerer 설정 스냅샷을 받아 필요한 내부 저장소에 일괄 등록합니다.
type Registerer struct {
	authRegistry *authconfig.Registry
}

// NewRegisterer 설정 스냅샷 등록용 Registerer를 생성합니다.
func NewRegisterer() *Registerer {
	return &Registerer{
		authRegistry: authconfig.NewRegistry(),
	}
}

// AuthRegistry 인증 런타임 설정 저장소를 반환합니다.
func (r *Registerer) AuthRegistry() *authconfig.Registry {
	return r.authRegistry
}

// Register 설정 스냅샷 전체를 내부 저장소에 반영합니다.
func (r *Registerer) Register(snapshot Snapshot) error {
	authRuntimeConfig, err := r.authRuntimeConfig(snapshot.Auth)
	if err != nil {
		return err
	}

	if err := r.authRegistry.Register(authRuntimeConfig); err != nil {
		return fmt.Errorf("register auth config: %w", err)
	}

	return nil
}

func (r *Registerer) authRuntimeConfig(authSection *AuthSection) (authconfig.RuntimeConfig, error) {
	if authSection == nil {
		return authconfig.RuntimeConfig{}, fmt.Errorf("%w: auth section is required", ErrInvalidSnapshot)
	}

	authClockSkew, err := time.ParseDuration(authSection.JWTClockSkew)
	if err != nil {
		return authconfig.RuntimeConfig{}, fmt.Errorf("parse auth clock skew: %w", err)
	}

	switch strings.TrimSpace(authSection.JWTAlgorithm) {
	case "HS256":
		if strings.TrimSpace(authSection.JWTSecret) == "" {
			return authconfig.RuntimeConfig{}, fmt.Errorf("%w: auth jwt_secret is required", ErrInvalidSnapshot)
		}
	default:
		if len(authSection.JWKS) == 0 {
			return authconfig.RuntimeConfig{}, fmt.Errorf("%w: auth jwks is required", ErrInvalidSnapshot)
		}
	}

	return authconfig.RuntimeConfig{
		JWTAlgorithm: authSection.JWTAlgorithm,
		JWTAudience:  authSection.JWTAudience,
		JWTClockSkew: authClockSkew,
		JWTIssuer:    authSection.JWTIssuer,
		JWTSecret:    []byte(strings.TrimSpace(authSection.JWTSecret)),
		JWKS:         append([]byte(nil), authSection.JWKS...),
	}, nil
}
