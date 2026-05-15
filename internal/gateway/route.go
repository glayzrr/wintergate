package gateway

import (
	"context"
	"fmt"

	internalconfig "wintergate/internal/config"

	routeconfig "wintergate/internal/route/config"
)

// RouteTask 요청 주소에 대응하는 서비스 이름과 라우트 정책을 찾습니다.
type RouteTask struct {
	settingsProvider SettingsProvider
	router           Router
	instanceSelector InstanceSelector
}

// SettingsProvider 현재 활성 설정 snapshot을 제공합니다.
type SettingsProvider interface {
	Settings() *internalconfig.Snapshot
}

// Router HTTP method와 path에 대응하는 라우팅 정보를 조회합니다.
type Router interface {
	RouteFor(snapshot *internalconfig.Snapshot, method, path string) (routeconfig.RouteInfo, bool)
}

// InstanceSelector 라우팅된 서비스의 업스트림 인스턴스를 선택합니다.
type InstanceSelector interface {
	NextInstance(snapshot *internalconfig.Snapshot, serviceName string) (internalconfig.InstanceSettings, error)
}

// NewRouteTask 라우트 정책 조회용 RouteTask를 생성합니다.
func NewRouteTask(settingsProvider SettingsProvider, router Router, instanceSelector InstanceSelector) *RouteTask {
	return &RouteTask{
		settingsProvider: settingsProvider,
		router:           router,
		instanceSelector: instanceSelector,
	}
}

// Run 요청 method와 path에 대응하는 라우트 정책과 서비스 이름을 상태에 기록합니다.
func (t *RouteTask) Run(_ context.Context, state *State) error {
	// 라우트 정책 저장소가 없으면 요청을 분류할 수 없으므로 즉시 실패합니다.
	if t.settingsProvider == nil {
		return fmt.Errorf("%w: settings provider is required", ErrInvalidRequest)
	}
	if t.router == nil {
		return fmt.Errorf("%w: router is required", ErrInvalidRequest)
	}
	if t.instanceSelector == nil {
		return fmt.Errorf("%w: instance selector is required", ErrInvalidRequest)
	}

	// 요청 처리 중 설정 버전이 섞이지 않도록 활성 snapshot을 한 번만 잡아 이후 조회에 전달합니다.
	snapshot := t.settingsProvider.Settings()
	state.Settings = snapshot
	if snapshot == nil {
		return fmt.Errorf("%w: settings snapshot is required", routeconfig.ErrConfigNotFound)
	}

	routeInfo, found := t.router.RouteFor(snapshot, state.Request.Method, state.Request.Path)
	if !found {
		return fmt.Errorf("%w: route %s %s", routeconfig.ErrConfigNotFound, state.Request.Method, state.Request.Path)
	}

	instance, err := t.instanceSelector.NextInstance(snapshot, routeInfo.ServiceName)
	if err != nil {
		return fmt.Errorf("select service instance: %w", err)
	}
	routeInfo.Instance = instance

	state.Request.ServiceName = routeInfo.ServiceName
	state.Route = &routeInfo

	return nil
}
