package gateway

import (
	"context"
	"fmt"

	routeconfig "wintergate/internal/route/config"
)

// RouteTask 요청 주소에 대응하는 서비스 이름과 라우트 정책을 찾습니다.
type RouteTask struct {
	provider RouteProvider
}

// RouteProvider HTTP method와 path에 대응하는 라우팅 정보를 조회합니다.
type RouteProvider interface {
	RouteFor(method, path string) (routeconfig.RouteInfo, bool)
}

// NewRouteTask 라우트 정책 조회용 RouteTask를 생성합니다.
func NewRouteTask(provider RouteProvider) *RouteTask {
	return &RouteTask{
		provider: provider,
	}
}

// Run 요청 method와 path에 대응하는 라우트 정책과 서비스 이름을 상태에 기록합니다.
func (t *RouteTask) Run(_ context.Context, state *State) error {
	// 라우트 정책 저장소가 없으면 요청을 분류할 수 없으므로 즉시 실패합니다.
	if t.provider == nil {
		return fmt.Errorf("%w: route provider is required", ErrInvalidRequest)
	}

	routeInfo, found := t.provider.RouteFor(state.Request.Method, state.Request.Path)
	if !found {
		return fmt.Errorf("%w: route %s %s", routeconfig.ErrConfigNotFound, state.Request.Method, state.Request.Path)
	}

	state.Request.ServiceName = routeInfo.ServiceName
	state.Route = &routeInfo

	return nil
}
