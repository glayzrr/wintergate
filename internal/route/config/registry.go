package config

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Registry 라우팅 런타임 설정과 엔트리를 메모리에 보관합니다.
type Registry struct {
	routes       map[string]routes
	serviceNames map[serviceKey]string

	// routes의 경쟁조건을 막기 위한 락입니다.
	routeMu sync.RWMutex

	// serviceNames의 경쟁조건을 막기 위한 락입니다.
	serviceMu sync.RWMutex
}

type serviceKey struct {
	host string
	port int
}

type routes struct {
	infos []RegistryRouteInfo
}

// NewRegistry 빈 라우팅 설정 Registry를 생성합니다.
func NewRegistry() *Registry {
	return &Registry{
		routes:       make(map[string]routes),
		serviceNames: make(map[serviceKey]string),
	}
}

// Register 전달받은 라우팅 설정과 엔트리로 현재 값을 교체합니다.
func (r *Registry) Register(cfg Config) error {
	if len(cfg.Entries) == 0 {
		return fmt.Errorf("%w: entries are required", ErrInvalidConfig)
	}

	registeredServiceNames, err := buildServiceNames(cfg.Services)
	if err != nil {
		return err
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
			continue
		}

		serviceRoutes.infos = append(serviceRoutes.infos, RegistryRouteInfo{
			Path:       path,
			HttpMethod: httpMethod,
			Roles:      roles,
		})
		registeredRoutes[service] = serviceRoutes
	}

	r.serviceMu.Lock()
	defer r.serviceMu.Unlock()

	r.routeMu.Lock()
	defer r.routeMu.Unlock()

	r.routes = registeredRoutes
	r.serviceNames = registeredServiceNames

	return nil
}

// RouteInfos 지정한 서비스에 대응하는 라우팅 정보 목록을 반환합니다.
func (r *Registry) RouteInfos(service string) ([]RouteInfo, error) {
	trimmedService := strings.TrimSpace(service)
	if trimmedService == "" {
		return nil, fmt.Errorf("%w: service is required", ErrInvalidConfig)
	}

	r.routeMu.RLock()
	defer r.routeMu.RUnlock()

	serviceRoutes, found := r.routes[trimmedService]
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotFound, trimmedService)
	}

	return toRouteInfos(serviceRoutes.infos), nil
}

// ServiceFor host와 port에 대응하는 서비스 이름을 반환합니다.
func (r *Registry) ServiceFor(host, port string) (string, error) {
	key, err := buildServiceKey(host, port)
	if err != nil {
		return "", err
	}

	r.serviceMu.RLock()
	defer r.serviceMu.RUnlock()

	serviceName, found := r.serviceNames[key]
	if !found {
		return "", fmt.Errorf("%w: host %q port %d", ErrServiceNotFound, key.host, key.port)
	}

	return serviceName, nil
}

func buildServiceNames(services []Service) (map[serviceKey]string, error) {
	names := make(map[serviceKey]string, len(services))
	for _, service := range services {
		key, err := buildServiceKey(service.Host, strconv.Itoa(service.Port))
		if err != nil {
			return nil, err
		}

		name := strings.TrimSpace(service.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: service name is required", ErrInvalidConfig)
		}

		if _, exists := names[key]; exists {
			return nil, fmt.Errorf("%w: duplicate service address %q:%d", ErrInvalidConfig, key.host, key.port)
		}

		names[key] = name
	}

	return names, nil
}

func buildServiceKey(host, port string) (serviceKey, error) {
	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		return serviceKey{}, fmt.Errorf("%w: host is required", ErrInvalidConfig)
	}

	trimmedPort := strings.TrimSpace(port)
	if trimmedPort == "" {
		return serviceKey{}, fmt.Errorf("%w: port is required", ErrInvalidConfig)
	}

	parsedPort, err := strconv.Atoi(trimmedPort)
	if err != nil {
		return serviceKey{}, fmt.Errorf("%w: parse port: %w", ErrInvalidConfig, err)
	}
	if parsedPort <= 0 || parsedPort > 65535 {
		return serviceKey{}, fmt.Errorf("%w: port is invalid", ErrInvalidConfig)
	}

	return serviceKey{
		host: strings.ToLower(trimmedHost),
		port: parsedPort,
	}, nil
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
