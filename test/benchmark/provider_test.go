package benchmark

import (
	"sync"

	internalhealth "wintergate/internal/health"
)

type benchmarkHealthProvider interface {
	IsRoutableKey(healthKey string) bool
	setRoutable(healthKey string, routable bool)
}

type benchmarkProviderFactory struct {
	name        string
	newProvider func([]string) benchmarkHealthProvider
}

// benchmarkProviderFactories health 상태 저장소 구현별 비교군을 반환합니다.
func benchmarkProviderFactories() []benchmarkProviderFactory {
	return []benchmarkProviderFactory{
		{
			name: "rwmutex",
			newProvider: func(keys []string) benchmarkHealthProvider {
				return newBenchmarkMutexHealthProvider(keys)
			},
		},
		{
			name: "atomic_cow",
			newProvider: func(keys []string) benchmarkHealthProvider {
				return newBenchmarkAtomicHealthProvider(keys)
			},
		},
	}
}

// benchmarkMutexHealthProvider는 개선 전 상태 저장소를 흉내 내는
// RWMutex 비교군입니다.
type benchmarkMutexHealthProvider struct {
	// mu는 요청 경로의 RLock과 health 갱신 경로의 Lock 경합을
	// 재현합니다.
	mu       sync.RWMutex
	routable map[string]bool
}

// newBenchmarkMutexHealthProvider 모든 인스턴스를 routable 상태로
// 시작하는 RWMutex 비교군을 생성합니다.
func newBenchmarkMutexHealthProvider(healthKeys []string) *benchmarkMutexHealthProvider {
	provider := &benchmarkMutexHealthProvider{
		routable: make(map[string]bool, len(healthKeys)),
	}
	for _, healthKey := range healthKeys {
		provider.routable[healthKey] = true
	}

	return provider
}

func (p *benchmarkMutexHealthProvider) IsRoutableKey(healthKey string) bool {
	if p == nil || healthKey == "" {
		return true
	}

	// 개선 전 모델은 매 요청마다 RLock을 잡아 health map을 읽는
	// 상황을 재현합니다.
	p.mu.RLock()
	routable, found := p.routable[healthKey]
	p.mu.RUnlock()
	if !found {
		return true
	}

	return routable
}

func (p *benchmarkMutexHealthProvider) setRoutable(healthKey string, routable bool) {
	if p == nil || healthKey == "" {
		return
	}

	// health flapping은 쓰기 lock을 잡으므로 요청 goroutine의
	// RLock 대기 시간을 만들 수 있습니다.
	p.mu.Lock()
	p.routable[healthKey] = routable
	p.mu.Unlock()
}

// benchmarkAtomicHealthProvider는 실제 Store 구현을 사용하는
// copy-on-write + atomic.Pointer 비교군입니다.
type benchmarkAtomicHealthProvider struct {
	store *internalhealth.Store
}

// newBenchmarkAtomicHealthProvider 모든 인스턴스를 healthy 상태로
// 초기화한 atomic 비교군을 생성합니다.
func newBenchmarkAtomicHealthProvider(healthKeys []string) *benchmarkAtomicHealthProvider {
	store := internalhealth.NewStore()
	for _, healthKey := range healthKeys {
		store.SetUnknown(healthKey, 1)
		store.UpdateStatus(healthKey, 1, internalhealth.StatusHealthy, 0, 1, nil)
	}

	return &benchmarkAtomicHealthProvider{
		store: store,
	}
}

func (p *benchmarkAtomicHealthProvider) IsRoutableKey(healthKey string) bool {
	if p == nil || p.store == nil {
		return true
	}

	return p.store.IsRoutableKey(healthKey)
}

func (p *benchmarkAtomicHealthProvider) setRoutable(healthKey string, routable bool) {
	if p == nil || p.store == nil || healthKey == "" {
		return
	}

	status := internalhealth.StatusUnhealthy
	if routable {
		status = internalhealth.StatusHealthy
	}

	// Store는 write path에서 새 map을 만든 뒤 atomic pointer를
	// 교체하므로 read path가 lock을 기다리지 않습니다.
	p.store.UpdateStatus(healthKey, 1, status, 0, 1, nil)
}
