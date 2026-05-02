package pool

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

var defaultClients = newClientStore()

type clientStore struct {
	shared    map[Tier]*cachedClient
	dedicated map[string]*cachedClient

	// shared/dedicated http.Client 캐시의 동시 조회와 교체를 보호합니다.
	mu sync.RWMutex
}

func newClientStore() *clientStore {
	store := &clientStore{
		shared:    make(map[Tier]*cachedClient, 3),
		dedicated: make(map[string]*cachedClient),
	}

	for _, tier := range []Tier{TierNormal, TierHot, TierSuper} {
		client, err := newCachedClient(tier)
		if err != nil {
			panic(err)
		}

		store.shared[tier] = client
	}

	return store
}

func (s *clientStore) client(decision Decision) (*cachedClient, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: client store is nil", ErrInvalidConfig)
	}

	tier, err := normalizeDecisionTier(decision.Tier)
	if err != nil {
		return nil, err
	}
	decision.Tier = tier

	if !decision.Dedicated {
		return s.sharedClient(decision)
	}

	return s.dedicatedClient(decision)
}

func (s *clientStore) sharedClient(decision Decision) (*cachedClient, error) {
	service := normalizeService(decision.Service)

	// 전용 client가 없는 서비스는 read lock만으로 shared client를 바로 반환합니다.
	s.mu.RLock()
	cached := s.shared[decision.Tier]
	_, hasDedicated := s.dedicated[service]
	if cached != nil && (!hasDedicated || service == "") {
		cached.acquire()
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// 전용 client에서 shared client로 복귀해야 할 때만 write lock을 잡습니다.
	s.mu.Lock()
	defer s.mu.Unlock()

	// Dedicated=false 판단이 내려진 서비스는 전용 풀을 해제하고 shared 풀로 합류합니다.
	if service != "" {
		if oldClient, found := s.dedicated[service]; found {
			oldClient.retire()
			delete(s.dedicated, service)
		}
	}

	// shared client는 normal/hot/super tier별로 하나씩 유지합니다.
	cached = s.shared[decision.Tier]
	if cached == nil {
		var err error
		cached, err = newCachedClient(decision.Tier)
		if err != nil {
			return nil, err
		}
		s.shared[decision.Tier] = cached
	}

	cached.acquire()
	return cached, nil
}

func (s *clientStore) dedicatedClient(decision Decision) (*cachedClient, error) {
	service := normalizeService(decision.Service)
	if service == "" {
		return nil, fmt.Errorf("%w: service is required for dedicated pool", ErrInvalidService)
	}

	// 이미 같은 tier의 전용 client가 있으면 read lock만으로 재사용합니다.
	s.mu.RLock()
	cached := s.dedicated[service]
	if cached != nil && cached.tier == decision.Tier {
		cached.acquire()
		s.mu.RUnlock()
		return cached, nil
	}
	s.mu.RUnlock()

	// 전용 client가 없거나 tier가 바뀐 경우에만 write lock으로 생성/교체합니다.
	s.mu.Lock()
	defer s.mu.Unlock()

	// lock 대기 중 다른 고루틴이 같은 tier client를 만들었을 수 있어 다시 확인합니다.
	cached = s.dedicated[service]
	if cached != nil && cached.tier == decision.Tier {
		cached.acquire()
		return cached, nil
	}

	// Transport 설정은 생성 후 변경하지 않으므로 tier 변경 시 새 client로 교체합니다.
	nextClient, err := newCachedClient(decision.Tier)
	if err != nil {
		return nil, err
	}

	// 기존 전용 client는 새 요청에서 제외하고, 진행 중 요청이 끝난 뒤 idle connection을 닫습니다.
	if cached != nil {
		cached.retire()
	}
	s.dedicated[service] = nextClient

	nextClient.acquire()
	return nextClient, nil
}

func normalizeDecisionTier(tier Tier) (Tier, error) {
	if strings.TrimSpace(string(tier)) == "" {
		return TierNormal, nil
	}

	normalizedTier, err := normalizeTier(tier)
	if err != nil {
		return "", err
	}

	return normalizedTier, nil
}

func (s *clientStore) count() (int, int) {
	if s == nil {
		return 0, 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.shared), len(s.dedicated)
}

func (s *clientStore) dedicatedTier(service string) (Tier, bool) {
	if s == nil {
		return "", false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	cached := s.dedicated[normalizeService(service)]
	if cached == nil {
		return "", false
	}

	return cached.tier, true
}

func (s *clientStore) sharedClientForTier(tier Tier) (*http.Client, bool) {
	if s == nil {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	cached := s.shared[tier]
	if cached == nil {
		return nil, false
	}

	return cached.client, true
}
