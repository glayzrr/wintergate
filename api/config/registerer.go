package configapi

import (
	"fmt"
	"strings"
	"time"

	authconfig "wintergate/internal/auth/config"
	routeconfig "wintergate/internal/route/config"
)

// Registerer 설정 스냅샷을 받아 필요한 내부 저장소에 일괄 등록합니다.
type Registerer struct {
	authRegistry  *authconfig.Registry
	routeRegistry *routeconfig.Registry
}

// NewRegisterer 설정 스냅샷 등록용 Registerer를 생성합니다.
func NewRegisterer() *Registerer {
	return &Registerer{
		authRegistry:  authconfig.NewRegistry(),
		routeRegistry: routeconfig.NewRegistry(),
	}
}

// AuthRegistry 인증 런타임 설정 저장소를 반환합니다.
func (r *Registerer) AuthRegistry() *authconfig.Registry {
	return r.authRegistry
}

// RouteRegistry 보호 라우트 런타임 설정 저장소를 반환합니다.
func (r *Registerer) RouteRegistry() *routeconfig.Registry {
	return r.routeRegistry
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

	if snapshot.Routes != nil && len(snapshot.Routes.Protected) > 0 {
		if err := r.routeRegistry.Register(r.routeRuntimeConfig(snapshot.Routes)); err != nil {
			return fmt.Errorf("register route config: %w", err)
		}
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

func (r *Registerer) routeRuntimeConfig(routesSection *RoutesSection) routeconfig.RuntimeConfig {
	entries := make([]routeconfig.Entry, 0, len(routesSection.Protected))
	for _, protected := range routesSection.Protected {
		entries = append(entries, routeconfig.Entry{
			Path:       protected.Path,
			Service:    protected.Service,
			HttpMethod: protected.Method,
			Roles:      append([]string(nil), protected.Roles...),
		})
	}

	return routeconfig.RuntimeConfig{
		Entries: entries,
	}
}
