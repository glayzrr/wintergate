package gateway

import (
	"context"
	"fmt"
	"strings"

	internalconfig "wintergate/internal/route/config"
	"wintergate/internal/utils"
)

// RouteTask 요청 주소에 대응하는 설정 키와 라우트 정책을 찾습니다.
type RouteTask struct {
	registry *internalconfig.Registry
}

// NewRouteTask 라우트 정책 조회용 RouteTask를 생성합니다.
func NewRouteTask(registry *internalconfig.Registry) *RouteTask {
	return &RouteTask{
		registry: registry,
	}
}

// Run 요청의 host와 port로 설정 키를 만들고 매칭된 라우트 정책을 상태에 기록합니다.
func (t *RouteTask) Run(_ context.Context, state *State) error {
	// 라우트 정책 저장소가 없으면 요청을 분류할 수 없으므로 즉시 실패합니다.
	if t.registry == nil {
		return fmt.Errorf("%w: route registry is required", ErrInvalidRequest)
	}

	configKey, err := utils.ConfigKey(state.Request.Host, state.Request.Port)
	if err != nil {
		return fmt.Errorf("%w: build config key: %w", ErrInvalidRequest, err)
	}
	state.Request.ConfigKey = configKey

	// 식별된 설정 키에 등록된 엔드포인트 정책 목록을 조회합니다.
	routeInfos, err := t.registry.RouteInfos(configKey)
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
