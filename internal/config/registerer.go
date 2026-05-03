package config

import (
	"fmt"
	"strings"
	"time"

	authconfig "wintergate/internal/auth/config"
	"wintergate/internal/pool"
	routeconfig "wintergate/internal/route/config"
)

// Registerer 설정 정보를 받아 필요한 내부 저장소에 일괄 등록합니다.
type Registerer struct {
	authRegistry  *authconfig.Registry
	routeRegistry *routeconfig.Registry
}

// NewRegisterer 설정 정보 등록용 Registerer를 생성합니다.
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

// Register 설정 정보 전체를 내부 저장소에 반영합니다.
func (r *Registerer) Register(settings Settings) error {
	if settings.Global == nil {
		return fmt.Errorf("%w: global settings is required", ErrInvalidSettings)
	}
	if len(settings.Routes) == 0 {
		return fmt.Errorf("%w: routes are required", ErrInvalidSettings)
	}

	authRuntimeConfig, err := r.registerAuthConfig(settings.Global.Auth)
	if err != nil {
		return err
	}

	if err := r.authRegistry.Register(authRuntimeConfig); err != nil {
		return fmt.Errorf("register auth config: %w", err)
	}

	if err := r.routeRegistry.Register(r.registerRouteConfig(settings.Routes)); err != nil {
		return fmt.Errorf("register route config: %w", err)
	}

	if err := pool.RegisterPolicies(r.poolPolicies(settings.Routes)); err != nil {
		return fmt.Errorf("register pool policies: %w", err)
	}

	return nil
}

func (r *Registerer) registerAuthConfig(authSettings *AuthSettings) (authconfig.Config, error) {
	if authSettings == nil {
		return authconfig.Config{}, fmt.Errorf("%w: auth settings is required", ErrInvalidSettings)
	}

	authClockSkew, err := time.ParseDuration(authSettings.JWTClockSkew)
	if err != nil {
		return authconfig.Config{}, fmt.Errorf("%w: parse auth clock skew: %w", ErrInvalidSettings, err)
	}

	switch strings.TrimSpace(authSettings.JWTAlgorithm) {
	case "HS256":
		if strings.TrimSpace(authSettings.JWTSecret) == "" {
			return authconfig.Config{}, fmt.Errorf("%w: auth jwt_secret is required", ErrInvalidSettings)
		}
	default:
		if len(authSettings.JWKS) == 0 {
			return authconfig.Config{}, fmt.Errorf("%w: auth jwks is required", ErrInvalidSettings)
		}
	}

	return authconfig.Config{
		JWTAlgorithm: authSettings.JWTAlgorithm,
		JWTAudience:  authSettings.JWTAudience,
		JWTClockSkew: authClockSkew,
		JWTIssuer:    authSettings.JWTIssuer,
		JWTSecret:    []byte(strings.TrimSpace(authSettings.JWTSecret)),
		JWKS:         append([]byte(nil), authSettings.JWKS...),
	}, nil
}

func (r *Registerer) registerRouteConfig(routeSettings []RouteSettings) routeconfig.Config {
	services := make([]routeconfig.Service, 0, len(routeSettings))
	var entries []routeconfig.Entry
	for _, service := range routeSettings {
		services = append(services, routeconfig.Service{
			Name: service.Name,
			Host: service.Host,
			Port: service.Port,
		})

		for _, endpoint := range service.Endpoints {
			entries = append(entries, routeconfig.Entry{
				Path:       endpoint.Path,
				Service:    service.Name,
				HttpMethod: endpoint.Method,
				Roles:      append([]string(nil), endpoint.Roles...),
			})
		}
	}

	return routeconfig.Config{
		Services: services,
		Entries:  entries,
	}
}

func (r *Registerer) poolPolicies(serviceSettings []RouteSettings) []pool.Policy {
	policies := make([]pool.Policy, 0, len(serviceSettings))
	for _, service := range serviceSettings {
		policy := pool.Policy{
			Service: service.Name,
		}

		if service.Threshold != nil {
			policy.Hot = pool.Threshold{
				RPS:      service.Threshold.Hot.RPS,
				InFlight: service.Threshold.Hot.InFlight,
			}
			policy.Super = pool.Threshold{
				RPS:      service.Threshold.Super.RPS,
				InFlight: service.Threshold.Super.InFlight,
			}
		}

		policies = append(policies, policy)
	}

	return policies
}
