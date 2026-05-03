package gateway

import (
	"context"
	"fmt"
	"strings"

	internalconfig "wintergate/internal/route/config"
)

// RouteTask 요청 주소에 대응하는 서비스와 라우트 정책을 찾습니다.
type RouteTask struct {
	registry *internalconfig.Registry
}

// NewRouteTask 라우트 정책 조회용 RouteTask를 생성합니다.
func NewRouteTask(registry *internalconfig.Registry) *RouteTask {
	return &RouteTask{
		registry: registry,
	}
}

// Run 요청의 host와 port로 서비스를 식별하고 매칭된 라우트 정책을 상태에 기록합니다.
func (t *RouteTask) Run(ctx context.Context, state *State) error {
	// 라우트 정책 저장소가 없으면 요청을 분류할 수 없으므로 즉시 실패합니다.
	if t.registry == nil {
		return fmt.Errorf("%w: route registry is required", ErrInvalidRequest)
	}

	// nginx가 전달한 host와 port로 서비스 이름을 식별합니다.
	service, err := t.registry.ServiceFor(state.Request.Host, state.Request.Port)
	if err != nil {
		return fmt.Errorf("find service: %w", err)
	}
	state.Request.Service = service

	// 식별된 서비스에 등록된 엔드포인트 정책 목록을 조회합니다.
	routeInfos, err := t.registry.RouteInfos(service)
	if err != nil {
		return fmt.Errorf("route infos: %w", err)
	}

	// 요청 method와 path에 맞는 정책을 찾아 이후 task가 사용할 수 있도록 상태에 저장합니다.
	for _, routeInfo := range routeInfos {
		if matchRoute(routeInfo, state.Request.Method, state.Request.Path) {
			matchedRoute := routeInfo
			state.Route = &matchedRoute
			return nil
		}
	}

	return nil
}

func matchRoute(routeInfo internalconfig.RouteInfo, method, path string) bool {
	if routeInfo.HttpMethod != "ALL" && routeInfo.HttpMethod != method {
		return false
	}

	routePath := strings.TrimSpace(routeInfo.Path)
	if strings.HasSuffix(routePath, "/**") {
		return strings.HasPrefix(path, strings.TrimSuffix(routePath, "/**"))
	}
	if routePath == path {
		return true
	}

	return false
}
