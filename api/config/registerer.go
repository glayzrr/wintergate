package configapi

import (
	"fmt"
	"time"

	authconfig "sidecargo/internal/auth/config"
	routeconfig "sidecargo/internal/route/config"
)

// Registerer 설정 스냅샷을 받아 필요한 내부 저장소에 일괄 등록합니다.
type Registerer struct {
	authRegistry    *authconfig.Registry
	routingRegistry *routeconfig.Registry
}

// NewRegisterer 설정 스냅샷 등록용 Registerer를 생성합니다.
func NewRegisterer(
	authRegistry *authconfig.Registry,
	routingRegistry *routeconfig.Registry,
) (*Registerer, error) {
	if authRegistry == nil {
		return nil, fmt.Errorf("%w: auth registry is required", ErrNilAuthRegistry)
	}

	if routingRegistry == nil {
		return nil, fmt.Errorf("%w: routing registry is required", ErrNilRoutingRegistry)
	}

	return &Registerer{
		authRegistry:    authRegistry,
		routingRegistry: routingRegistry,
	}, nil
}

// Register 설정 스냅샷 전체를 내부 저장소에 반영합니다.
func (r *Registerer) Register(snapshot Snapshot) error {
	authRuntimeConfig, err := r.authRuntimeConfig(snapshot.Auth)
	if err != nil {
		return err
	}

	routingRuntimeConfig, err := r.routingRuntimeConfig(snapshot.Routing)
	if err != nil {
		return err
	}

	if err := r.authRegistry.Register(authRuntimeConfig); err != nil {
		return fmt.Errorf("register auth config: %w", err)
	}

	if err := r.routingRegistry.Register(routingRuntimeConfig); err != nil {
		return fmt.Errorf("register routing config: %w", err)
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

	if len(authSection.JWKS) == 0 {
		return authconfig.RuntimeConfig{}, fmt.Errorf("%w: auth jwks is required", ErrInvalidSnapshot)
	}

	return authconfig.RuntimeConfig{
		JWTAlgorithm: authSection.JWTAlgorithm,
		JWTAudience:  authSection.JWTAudience,
		JWTClockSkew: authClockSkew,
		JWTIssuer:    authSection.JWTIssuer,
		JWKS:         append([]byte(nil), authSection.JWKS...),
	}, nil
}

func (r *Registerer) routingRuntimeConfig(routingSection *RoutingSection) (routeconfig.RuntimeConfig, error) {
	if routingSection == nil {
		return routeconfig.RuntimeConfig{}, fmt.Errorf("%w: routing section is required", ErrInvalidSnapshot)
	}

	routeUpstreamTimeout, err := time.ParseDuration(routingSection.RouteUpstreamRequestTimeout)
	if err != nil {
		return routeconfig.RuntimeConfig{}, fmt.Errorf("parse route upstream timeout: %w", err)
	}

	if len(routingSection.Routes) == 0 {
		return routeconfig.RuntimeConfig{}, fmt.Errorf("%w: routing routes are required", ErrInvalidSnapshot)
	}

	entries := make([]routeconfig.Entry, 0, len(routingSection.Routes))
	for _, routeValue := range routingSection.Routes {
		entries = append(entries, routeconfig.Entry{
			Path:    routeValue.Path,
			Service: routeValue.Service,
		})
	}

	return routeconfig.RuntimeConfig{
		RouteServiceHeader:          routingSection.RouteServiceHeader,
		RouteUpstreamRequestTimeout: routeUpstreamTimeout,
		Entries:                     entries,
	}, nil
}
