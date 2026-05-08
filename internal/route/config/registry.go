package config

import (
	"fmt"
	"sync"

	"wintergate/internal/route"
)

// Registry 라우팅 런타임 설정과 엔트리를 메모리에 보관합니다.
type Registry struct {
	routes map[string]routes

	// routes의 경쟁조건을 막기 위한 락입니다.
	routeMu sync.RWMutex
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

// Register 전달받은 설정 키의 라우팅 설정과 엔트리를 등록하거나 교체합니다.
func (r *Registry) Register(cfg Config) error {
	if len(cfg.Entries) == 0 {
		return fmt.Errorf("%w: entries are required", ErrInvalidConfig)
	}

	key := route.NormalizeConfigKey(cfg.Key)
	if key == "" {
		return fmt.Errorf("%w: config key is required", ErrInvalidConfig)
	}

	registeredRoutes := routes{}
	for _, entry := range cfg.Entries {
		path := route.NormalizeHTTPPath(entry.Path)
		if path == "" {
			return fmt.Errorf("%w: path is required", ErrInvalidConfig)
		}

		httpMethod := route.NormalizeHTTPMethod(entry.HttpMethod)
		if httpMethod == "" {
			return fmt.Errorf("%w: http method is required for config key %q", ErrInvalidConfig, key)
		}

		roles, ok := route.NormalizeRoles(entry.Roles)
		if !ok {
			return fmt.Errorf("%w: role is required", ErrInvalidConfig)
		}

		if hasRouteInfo(registeredRoutes.infos, path, httpMethod) {
			return fmt.Errorf("%w: duplicate route %s %s %s", ErrInvalidConfig, key, httpMethod, path)
		}

		registeredRoutes.infos = append(registeredRoutes.infos, RegistryRouteInfo{
			Path:       path,
			HttpMethod: httpMethod,
			Roles:      roles,
		})
	}

	r.routeMu.Lock()
	defer r.routeMu.Unlock()

	r.routes[key] = registeredRoutes

	return nil
}

// RouteInfos 지정한 설정 키에 대응하는 라우팅 정보 목록을 반환합니다.
func (r *Registry) RouteInfos(configKey string) ([]RouteInfo, error) {
	key := route.NormalizeConfigKey(configKey)
	if key == "" {
		return nil, fmt.Errorf("%w: config key is required", ErrInvalidConfig)
	}

	r.routeMu.RLock()
	defer r.routeMu.RUnlock()

	configRoutes, found := r.routes[key]
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, key)
	}

	return toRouteInfos(configRoutes.infos), nil
}

func hasRouteInfo(routeInfos []RegistryRouteInfo, path string, httpMethod string) bool {
	for _, routeInfo := range routeInfos {
		if routeInfo.Path == path && routeInfo.HttpMethod == httpMethod {
			return true
		}
	}

	return false
}
