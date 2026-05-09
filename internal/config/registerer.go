package config

import (
	"fmt"
	"strings"
	"time"

	authconfig "wintergate/internal/auth/config"
	"wintergate/internal/pool"
	routeconfig "wintergate/internal/route/config"
	"wintergate/internal/utils"
)

// Registerer 설정 정보를 받아 필요한 내부 저장소에 일괄 등록합니다.
type Registerer struct {
	authRegistry  *authconfig.Registry
	routeRegistry *routeconfig.Registry
	store         *Store
}

// NewRegisterer 설정 정보 등록용 Registerer를 생성합니다.
func NewRegisterer() *Registerer {
	return &Registerer{
		authRegistry:  authconfig.NewRegistry(),
		routeRegistry: routeconfig.NewRegistry(),
		store:         NewStore(),
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

// Store 서비스 라우팅과 인스턴스 런타임 저장소를 반환합니다.
func (r *Registerer) Store() *Store {
	return r.store
}

// Register 설정 정보를 요청 서비스와 현재 인스턴스에 대응하는 런타임 저장소에 반영합니다.
func (r *Registerer) Register(settings Settings, host, port string) error {
	if settings.Global == nil {
		return fmt.Errorf("%w: global settings is required", ErrInvalidSettings)
	}

	if normalizeServiceName(settings.ServiceName) == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidSettings)
	}
	if len(settings.Endpoints) == 0 {
		return fmt.Errorf("%w: endpoints are required", ErrInvalidSettings)
	}

	configKey, err := utils.ConfigKey(host, port)
	if err != nil {
		return fmt.Errorf("%w: config address: %w", ErrInvalidSettings, err)
	}

	authRuntimeConfig, err := r.registerAuthConfig(settings.Global.Auth)
	if err != nil {
		return err
	}

	if r.store != nil {
		if err := r.store.RegisterService(settings, InstanceSettings{Host: host, Port: port}); err != nil {
			return fmt.Errorf("register service settings: %w", err)
		}
	}

	if err := r.authRegistry.RegisterFor(configKey, authRuntimeConfig); err != nil {
		return fmt.Errorf("register auth config: %w", err)
	}

	if err := r.routeRegistry.Register(r.registerRouteConfig(configKey, settings.Endpoints)); err != nil {
		return fmt.Errorf("register route config: %w", err)
	}

	if settings.Threshold == nil {
		pool.UnregisterPolicy(configKey)
		return nil
	}

	if err := pool.RegisterPolicies([]pool.Policy{r.poolPolicy(configKey, *settings.Threshold)}); err != nil {
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

func (r *Registerer) registerRouteConfig(configKey string, endpoints []EndpointSettings) routeconfig.Config {
	entries := make([]routeconfig.Entry, 0, len(endpoints))
	for _, endpoint := range endpoints {
		entries = append(entries, routeconfig.Entry{
			Path:       endpoint.Path,
			HttpMethod: endpoint.Method,
			Roles:      append([]string(nil), endpoint.Roles...),
		})
	}

	return routeconfig.Config{
		Key:     configKey,
		Entries: entries,
	}
}

func (r *Registerer) poolPolicy(configKey string, threshold ThresholdSettings) pool.Policy {
	return pool.Policy{
		ConfigKey: configKey,
		Hot: pool.Threshold{
			RPS:      threshold.Hot.RPS,
			InFlight: threshold.Hot.InFlight,
		},
		Super: pool.Threshold{
			RPS:      threshold.Super.RPS,
			InFlight: threshold.Super.InFlight,
		},
	}
}
