package health

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	internalconfig "wintergate/internal/config"
)

// Status 헬스 체크가 관측한 인스턴스 상태입니다.
type Status string

const (
	// StatusUnknown 아직 라우팅에서 제외할 만큼 충분한 헬스 체크 결과가 없는 상태입니다.
	StatusUnknown Status = "unknown"
	// StatusHealthy 헬스 체크가 인스턴스를 정상으로 판단한 상태입니다.
	StatusHealthy Status = "healthy"
	// StatusUnhealthy 헬스 체크가 인스턴스를 비정상으로 판단한 상태입니다.
	StatusUnhealthy Status = "unhealthy"
)

const healthDrainLimit = 1 << 20

// Manager 중앙 설정 스냅샷에 등록된 인스턴스들의 헬스 체크 생명주기를 관리합니다.
type Manager struct {
	targets map[targetKey]*target
	store   *Store

	client       atomic.Pointer[http.Client]
	transport    *http.Transport
	maxIdleConns int
	generation   uint64

	// mu는 target/client 교체를 보호합니다.
	mu sync.Mutex
}

// NewManager 빈 헬스 체크 Manager를 생성합니다.
func NewManager(stores ...*Store) *Manager {
	store := NewStore()
	if len(stores) > 0 && stores[0] != nil {
		store = stores[0]
	}

	manager := &Manager{
		targets: make(map[targetKey]*target),
		store:   store,
	}
	manager.replaceClientLocked(1)

	return manager
}

// OnSnapshotCommitted 활성 설정 스냅샷 변경에 맞춰 헬스 체크 타겟을 동기화합니다.
func (m *Manager) OnSnapshotCommitted(snapshot *internalconfig.Snapshot) {
	if m == nil {
		return
	}

	m.reconcile(snapshot)
}

// reconcile 중앙 스냅샷과 현재 health target 목록을 맞춥니다.
func (m *Manager) reconcile(snapshot *internalconfig.Snapshot) {
	// snapshot은 설정의 단일 기준이므로, 먼저 snapshot만으로 원하는 target 집합을 계산합니다.
	// 이후에는 desired map에 남은 항목만 신규 생성 대상으로 해석합니다.
	desired := desiredTargets(snapshot)

	// target 목록과 health 전용 client는 서로 맞물린 런타임 상태이므로 같은 lock에서 갱신합니다.
	m.mu.Lock()
	toCancel := make([]context.CancelFunc, 0)

	// 중복 방지용 루프입니다.
	for key, existingTarget := range m.targets {
		nextTarget, found := desired[key]
		if found && existingTarget.settings == nextTarget.settings {
			// 설정이 그대로인 target은 기존 고루틴과 connection 상태를 유지합니다.
			// desired에서 제거해 아래 신규 target 생성 루프가 중복 고루틴을 만들지 않도록 합니다.
			delete(desired, key)
			continue
		}

		// snapshot에서 사라졌거나 health 설정이 바뀐 target은 기존 루프를 종료합니다.
		// status도 함께 제거해 deregister된 인스턴스의 오래된 unhealthy 상태가 라우팅 판단에 남지 않게 합니다.
		toCancel = append(toCancel, existingTarget.cancel)
		delete(m.targets, key)
		m.store.Delete(key.key())
	}

	for key, nextTarget := range desired {
		// 새 target은 unknown 상태로 시작해 실패 임계치 전까지 라우팅 후보에 남깁니다.
		// 등록 직후 첫 health check가 끝나기 전에도 트래픽을 받을 수 있어 cold start 차단을 피할 수 있습니다.
		m.generation++
		ctx, cancel := context.WithCancel(context.Background())
		managedTarget := &target{
			key:        key,
			instance:   nextTarget.instance,
			settings:   nextTarget.settings,
			jitter:     nextTarget.jitter,
			generation: m.generation,
			cancel:     cancel,
		}
		m.targets[key] = managedTarget
		m.store.SetUnknown(key.key(), managedTarget.generation)

		go m.run(ctx, managedTarget)
	}

	// health 전용 pool의 전체 idle connection 수는 현재 target 수와 맞춥니다.
	m.replaceClientLocked(len(m.targets))
	m.mu.Unlock()

	for _, cancel := range toCancel {
		// cancel은 lock 밖에서 호출해 종료 중인 health 루프와 manager lock 경합을 줄입니다.
		// 종료된 루프가 마지막 status 갱신을 시도해도 status map에서 target이 제거되어 즉시 무시됩니다.
		cancel()
	}
}

// replaceClientLocked target 수에 맞춘 health 전용 connection pool을 교체합니다.
func (m *Manager) replaceClientLocked(targetCount int) {
	maxIdleConns := targetCount
	if maxIdleConns < 1 {
		maxIdleConns = 1
	}
	if m.client.Load() != nil && m.maxIdleConns == maxIdleConns {
		return
	}

	// health 전용 transport는 메인 라우팅 transport와 완전히 분리합니다.
	defaultTransport := http.DefaultTransport.(*http.Transport)
	transport := defaultTransport.Clone()
	transport.MaxIdleConns = maxIdleConns
	transport.MaxIdleConnsPerHost = 1
	transport.MaxConnsPerHost = 1
	transport.DisableKeepAlives = false

	oldTransport := m.transport
	m.client.Store(&http.Client{Transport: transport})
	m.transport = transport
	m.maxIdleConns = maxIdleConns
	if oldTransport != nil {
		// 진행 중 요청은 건드리지 않고 이전 health pool의 idle connection만 정리합니다.
		oldTransport.CloseIdleConnections()
	}
}
