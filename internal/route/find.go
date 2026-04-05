package route

import (
	"fmt"
	routeconfig "wintergate/internal/route/config"
)

// Router 서비스 이름으로 등록된 라우팅 정보를 조회합니다.
type Router struct {
	registry *routeconfig.Registry
}

// NewRouter 라우팅 정보 조회용 Router를 생성합니다.
func NewRouter(registry *routeconfig.Registry) *Router {
	return &Router{registry: registry}
}

// ReplaceRegistry Router가 사용할 라우팅 저장소를 교체합니다.
func (r *Router) ReplaceRegistry(registry *routeconfig.Registry) error {
	if registry == nil {
		return fmt.Errorf("%w: registry is required", ErrNilRegistry)
	}

	r.registry = registry

	return nil
}

// Route 서비스에 속한 라우팅 정보 목록을 반환합니다.
func (r *Router) Route(service string) ([]routeconfig.RouteInfo, error) {
	if r.registry == nil {
		return nil, fmt.Errorf("%w: registry is required", ErrNilRegistry)
	}

	routeInfos, found := r.registry.RouteInfos(service)
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrServiceNotFound, service)
	}

	return routeInfos, nil
}
