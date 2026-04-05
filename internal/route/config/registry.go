package config

import (
	"fmt"
	"strings"
	"sync"
)

// Registry 라우팅 런타임 설정과 엔트리를 메모리에 보관합니다.
type Registry struct {
	routes map[string]routes

	// routes의 경쟁조건을 막기 위한 락입니다.
	mu sync.RWMutex
}

type routes struct {
	infos []RegistryRouteInfo
}

// NewRegistry 빈 라우팅 설정 Registry를 생성합니다.
func NewRegistry() *Registry {
	return &Registry{
		routes: make(map[string]routes),
	}
}

// Register 전달받은 라우팅 설정과 엔트리로 현재 값을 교체합니다.
func (r *Registry) Register(cfg RuntimeConfig) error {
	if len(cfg.Entries) == 0 {
		return fmt.Errorf("%w: entries are required", ErrInvalidConfig)
	}

	registeredRoutes := make(map[string]routes, len(cfg.Entries))
	for _, entry := range cfg.Entries {
		path := strings.TrimSpace(entry.Path)
		if path == "" {
			return fmt.Errorf("%w: path is required", ErrInvalidConfig)
		}

		service := strings.TrimSpace(entry.Service)
		if service == "" {
			return fmt.Errorf("%w: service is required for path %q", ErrInvalidConfig, path)
		}

		httpMethod := strings.ToUpper(strings.TrimSpace(entry.HttpMethod))
		if httpMethod == "" {
			return fmt.Errorf("%w: http method is required for service %q", ErrInvalidConfig, service)
		}

		roles, err := normalizedRoles(entry.Roles)
		if err != nil {
			return fmt.Errorf("normalize roles: %w", err)
		}

		serviceRoutes := registeredRoutes[service]
		if hasRouteInfo(serviceRoutes.infos, path, httpMethod) {
			return fmt.Errorf("%w: duplicate route for service %q", ErrInvalidConfig, service)
		}

		serviceRoutes.infos = append(serviceRoutes.infos, RegistryRouteInfo{
			Path:       path,
			HttpMethod: httpMethod,
			Roles:      roles,
		})
		registeredRoutes[service] = serviceRoutes
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes = registeredRoutes

	return nil
}

// RouteInfos 지정한 서비스에 대응하는 라우팅 정보 목록을 반환합니다.
func (r *Registry) RouteInfos(service string) ([]RouteInfo, bool) {
	trimmedService := strings.TrimSpace(service)
	if trimmedService == "" {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	serviceRoutes, found := r.routes[trimmedService]
	if !found {
		return nil, false
	}

	return toRouteInfos(serviceRoutes.infos), true
}

func normalizedRoles(roles []string) ([]string, error) {
	if len(roles) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(roles))
	for _, role := range roles {
		trimmedRole := strings.TrimSpace(role)
		if trimmedRole == "" {
			return nil, fmt.Errorf("%w: role is required", ErrInvalidConfig)
		}

		normalized = append(normalized, trimmedRole)
	}

	return normalized, nil
}

func hasRouteInfo(routeInfos []RegistryRouteInfo, path string, httpMethod string) bool {
	for _, routeInfo := range routeInfos {
		if routeInfo.Path == path && routeInfo.HttpMethod == httpMethod {
			return true
		}
	}

	return false
}
