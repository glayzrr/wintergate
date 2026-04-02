package config

import (
	"fmt"
	"strings"
	"sync"
)

// Registry 라우팅 런타임 설정과 엔트리를 메모리에 보관합니다.
type Registry struct {
	mu        sync.RWMutex
	routeInfo map[string]RouteInfo
	set       bool
}

// NewRegistry 빈 라우팅 설정 Registry를 생성합니다.
func NewRegistry() *Registry {
	return &Registry{
		routeInfo: make(map[string]RouteInfo),
	}
}

// Register 전달받은 라우팅 설정과 엔트리로 현재 값을 교체합니다.
func (r *Registry) Register(cfg RuntimeConfig) error {
	if strings.TrimSpace(cfg.RouteServiceHeader) == "" {
		return fmt.Errorf("%w: route_service_header is required", ErrInvalidConfig)
	}

	if cfg.RouteUpstreamRequestTimeout <= 0 {
		return fmt.Errorf("%w: route_upstream_request_timeout must be greater than zero", ErrInvalidConfig)
	}

	if len(cfg.Entries) == 0 {
		return fmt.Errorf("%w: entries are required", ErrInvalidConfig)
	}

	registeredRoutes := make(map[string]RouteInfo, len(cfg.Entries))
	for _, entry := range cfg.Entries {
		path := strings.TrimSpace(entry.Path)
		if path == "" {
			return fmt.Errorf("%w: path is required", ErrInvalidConfig)
		}

		service := strings.TrimSpace(entry.Service)
		if service == "" {
			return fmt.Errorf("%w: service is required for path %q", ErrInvalidConfig, path)
		}

		clientIP := strings.TrimSpace(entry.ClientIP)
		if clientIP == "" {
			return fmt.Errorf("%w: client ip is required for service %q", ErrInvalidConfig, service)
		}

		if entry.Port <= 0 {
			return fmt.Errorf("%w: port must be greater than zero for path %q", ErrInvalidConfig, path)
		}

		if _, exists := registeredRoutes[path]; exists {
			return fmt.Errorf("%w: duplicate path %q", ErrInvalidConfig, path)
		}

		registeredRoutes[path] = RouteInfo{
			Service:  service,
			ClientIP: clientIP,
			Port:     entry.Port,
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.routeInfo = registeredRoutes
	r.set = true

	return nil
}

// Route 지정한 경로에 대응하는 서비스 이름, IP, 포트를 반환합니다.
func (r *Registry) Route(path string) (RouteInfo, bool) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return RouteInfo{}, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	routeInfo, found := r.routeInfo[trimmedPath]
	if !found {
		return RouteInfo{}, false
	}

	return routeInfo, true
}

// Snapshot 현재 등록된 라우팅 정보의 사본을 반환합니다.
func (r *Registry) Snapshot() (map[string]RouteInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.set {
		return nil, false
	}

	snapshot := make(map[string]RouteInfo, len(r.routeInfo))
	for path, routeInfo := range r.routeInfo {
		snapshot[path] = routeInfo
	}

	return snapshot, true
}
