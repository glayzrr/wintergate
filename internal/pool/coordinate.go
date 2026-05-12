package pool

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"wintergate/internal/utils"
)

// Coordinator 공유 pool과 서비스별 전용 pool의 client 생명주기를 관리합니다.
type Coordinator struct {
	shared    map[Tier]*managedClient
	dedicated map[string]*managedClient

	// shared/dedicated http.Client 캐시의 동시 조회와 교체를 보호합니다.
	mu sync.RWMutex
}

// ClientLease 요청에 사용할 http.Client와 사용 완료 함수를 묶어 반환합니다.
type ClientLease struct {
	Client *http.Client
	Finish func()
}

// ClientProvider 요청별 pool 결정 결과에 맞는 http.Client를 대여합니다.
type ClientProvider interface {
	Acquire(Assignment) (ClientLease, error)
}

// NewCoordinator 현재 pool 설정으로 Coordinator를 생성합니다.
func NewCoordinator() *Coordinator {
	store := &Coordinator{
		shared:    make(map[Tier]*managedClient, 3),
		dedicated: make(map[string]*managedClient),
	}

	for _, tier := range []Tier{TierNormal, TierHot, TierSuper} {
		client, err := newManagedClient(tier)
		if err != nil {
			panic(err)
		}

		store.shared[tier] = client
	}

	return store
}

// Acquire 요청별 pool 결정 결과에 맞는 http.Client lease를 반환합니다.
func (p *Coordinator) Acquire(assignment Assignment) (ClientLease, error) {
	client, err := p.clientFor(assignment)
	if err != nil {
		return ClientLease{}, err
	}

	return ClientLease{
		Client: client.client,
		Finish: client.release,
	}, nil
}

func (p *Coordinator) clientFor(assignment Assignment) (*managedClient, error) {
	normalizedTier, err := poolTier(assignment.Tier)
	if err != nil {
		return nil, err
	}
	assignment.Tier = normalizedTier

	if !assignment.Dedicated {
		return p.sharedClient(assignment)
	}

	return p.dedicatedClient(assignment)
}

func (p *Coordinator) sharedClient(assignment Assignment) (*managedClient, error) {
	configKey := utils.NormalizeServiceName(assignment.ServiceName)

	// 전용 client가 없는 서비스는 read lock만으로 shared client를 바로 반환합니다.
	p.mu.RLock()
	cached := p.shared[assignment.Tier]
	_, hasDedicated := p.dedicated[configKey]
	if cached != nil && (!hasDedicated || configKey == "") {
		cached.acquire()
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	// 전용 client에서 shared client로 복귀해야 할 때만 write lock을 잡습니다.
	p.mu.Lock()
	defer p.mu.Unlock()

	// Dedicated=false 판단이 내려진 서비스는 전용 풀을 해제하고 shared 풀로 합류합니다.
	if configKey != "" {
		if oldClient, found := p.dedicated[configKey]; found {
			oldClient.retire()
			delete(p.dedicated, configKey)
		}
	}

	// shared client는 normal/hot/super tier별로 하나씩 유지합니다.
	cached = p.shared[assignment.Tier]
	if cached == nil {
		var err error
		cached, err = newManagedClient(assignment.Tier)
		if err != nil {
			return nil, err
		}
		p.shared[assignment.Tier] = cached
	}

	cached.acquire()
	return cached, nil
}

func (p *Coordinator) dedicatedClient(assignment Assignment) (*managedClient, error) {
	configKey := utils.NormalizeServiceName(assignment.ServiceName)
	if configKey == "" {
		return nil, fmt.Errorf("%w: config key is required for dedicated pool", ErrInvalidConfigKey)
	}

	// 이미 같은 tier의 전용 client가 있으면 read lock만으로 재사용합니다.
	p.mu.RLock()
	cached := p.dedicated[configKey]
	if cached != nil && cached.tier == assignment.Tier {
		cached.acquire()
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	// 전용 client가 없거나 tier가 바뀐 경우에만 write lock으로 생성/교체합니다.
	p.mu.Lock()
	defer p.mu.Unlock()

	// lock 대기 중 다른 고루틴이 같은 tier client를 만들었을 수 있어 다시 확인합니다.
	cached = p.dedicated[configKey]
	if cached != nil && cached.tier == assignment.Tier {
		cached.acquire()
		return cached, nil
	}

	// Transport 설정은 생성 후 변경하지 않으므로 tier 변경 시 새 client로 교체합니다.
	nextClient, err := newManagedClient(assignment.Tier)
	if err != nil {
		return nil, err
	}

	// 기존 전용 client는 새 요청에서 제외하고, 진행 중 요청이 끝난 뒤 idle connection을 닫습니다.
	previousTier := ""
	if cached != nil {
		previousTier = string(cached.tier)
		cached.retire()
	}
	p.dedicated[configKey] = nextClient

	slog.Info(
		logDedicatedPoolReplaced,
		logAttrServiceName, configKey,
		logAttrTier, assignment.Tier,
		logAttrPreviousTier, previousTier,
		logAttrRPS, assignment.Status.RPS,
		logAttrInFlight, assignment.Status.InFlight,
		logAttrRequestsInWindow, assignment.Status.RequestsInWindow,
		logAttrStartedRequests, assignment.Status.StartedRequests,
		logAttrFinishedRequests, assignment.Status.FinishedRequests,
		logAttrAverageLatency, assignment.Status.AverageLatency,
	)

	nextClient.acquire()
	return nextClient, nil
}

func (p *Coordinator) count() (int, int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.shared), len(p.dedicated)
}

func (p *Coordinator) dedicatedTier(configKey string) (Tier, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cached := p.dedicated[utils.NormalizeServiceName(configKey)]
	if cached == nil {
		return "", false
	}

	return cached.tier, true
}

func (p *Coordinator) sharedClientForTier(tier Tier) (*http.Client, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cached := p.shared[tier]
	if cached == nil {
		return nil, false
	}

	return cached.client, true
}
