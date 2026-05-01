package pool

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var defaultClients = mustNewClientStore()

type clientStore struct {
	shared    map[Tier]*cachedClient
	dedicated map[string]*cachedClient

	// shared/dedicated http.Client 캐시의 동시 조회와 교체를 보호합니다.
	mu sync.RWMutex
}

type cachedClient struct {
	tier   Tier
	client *http.Client

	// 커넥션 풀 교체시 미완료된 요청을 기다리기 위해 사용됩니다.
	wg sync.WaitGroup
}

// NewTransport 티어 풀 설정을 반영한 새 http.Transport를 생성합니다.
func NewTransport(tier Tier) (*http.Transport, error) {
	config, err := GetConfig(tier)
	if err != nil {
		return nil, err
	}

	return makePool(config)
}

func makePool(config Config) (*http.Transport, error) {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("%w: default transport is not *http.Transport", ErrInvalidConfig)
	}

	transport := defaultTransport.Clone()
	transport.MaxIdleConns = config.MaxIdleConns
	transport.MaxIdleConnsPerHost = config.MaxIdleConnsPerHost
	transport.MaxConnsPerHost = config.MaxConnsPerHost
	transport.IdleConnTimeout = config.IdleConnTimeout
	transport.ResponseHeaderTimeout = config.ResponseHeaderTimeout
	transport.TLSHandshakeTimeout = config.TLSHandshakeTimeout
	transport.ExpectContinueTimeout = config.ExpectContinueTimeout

	return transport, nil
}

func HandleRequest(serviceName, host string, w http.ResponseWriter, r *http.Request) error {
	doneFunc := StartRecord(serviceName)
	defer doneFunc()

	status, err := StatusFor(serviceName)
	if err != nil {
		return err
	}

	decision := DecidePolicy(status)
	cachedClient, err := defaultClients.client(decision)
	if err != nil {
		return err
	}
	defer cachedClient.release()

	outReq, err := upstreamRequest(host, r)
	if err != nil {
		return err
	}

	resp, err := cachedClient.client.Do(outReq)
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return err
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	removeHopByHopHeaders(w.Header())
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy upstream response body: %w", err)
	}

	return nil
}

func newClientStore() *clientStore {
	store, err := newClientStoreWithSharedClients()
	if err != nil {
		return &clientStore{
			shared:    make(map[Tier]*cachedClient),
			dedicated: make(map[string]*cachedClient),
		}
	}

	return store
}

func mustNewClientStore() *clientStore {
	store, err := newClientStoreWithSharedClients()
	if err != nil {
		panic(err)
	}

	return store
}

func newClientStoreWithSharedClients() (*clientStore, error) {
	store := &clientStore{
		shared:    make(map[Tier]*cachedClient, 3),
		dedicated: make(map[string]*cachedClient),
	}

	for _, tier := range []Tier{TierNormal, TierHot, TierSuper} {
		client, err := newCachedClient(tier)
		if err != nil {
			return nil, err
		}

		store.shared[tier] = client
	}

	return store, nil
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

func newCachedClient(tier Tier) (*cachedClient, error) {
	transport, err := NewTransport(tier)
	if err != nil {
		return nil, err
	}

	return &cachedClient{
		tier: tier,
		client: &http.Client{
			Transport: transport,
		},
	}, nil
}

func (c *cachedClient) closeIdleConnections() {
	if c == nil || c.client == nil {
		return
	}

	transport, ok := c.client.Transport.(*http.Transport)
	if !ok {
		return
	}

	transport.CloseIdleConnections()
}

func (c *cachedClient) acquire() {
	if c == nil {
		return
	}

	c.wg.Add(1)
}

func (c *cachedClient) release() {
	if c == nil {
		return
	}

	c.wg.Done()
}

func (c *cachedClient) retire() {
	if c == nil {
		return
	}

	c.closeIdleConnections()
	go func() {
		c.wg.Wait()
		c.closeIdleConnections()
	}()
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

func upstreamRequest(host string, r *http.Request) (*http.Request, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrInvalidConfig)
	}

	targetURL, err := upstreamURL(host, r.URL)
	if err != nil {
		return nil, err
	}

	outReq := r.Clone(r.Context())
	outReq.URL = targetURL
	outReq.Host = targetURL.Host
	outReq.RequestURI = ""
	outReq.Header = r.Header.Clone()
	removeHopByHopHeaders(outReq.Header)

	return outReq, nil
}

func upstreamURL(host string, requestURL *url.URL) (*url.URL, error) {
	base, err := url.Parse(strings.TrimSpace(host))
	if err != nil {
		return nil, fmt.Errorf("%w: parse upstream host: %w", ErrInvalidConfig, err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("%w: upstream host must include scheme and host", ErrInvalidConfig)
	}

	target := *base
	if requestURL != nil {
		target.Path = joinURLPath(base.Path, requestURL.Path)
		target.RawQuery = requestURL.RawQuery
	}
	target.Fragment = ""

	return &target, nil
}

func joinURLPath(basePath, requestPath string) string {
	switch {
	case basePath == "":
		if requestPath == "" {
			return "/"
		}
		return requestPath
	case requestPath == "":
		return basePath
	case strings.HasSuffix(basePath, "/") && strings.HasPrefix(requestPath, "/"):
		return basePath + requestPath[1:]
	case !strings.HasSuffix(basePath, "/") && !strings.HasPrefix(requestPath, "/"):
		return basePath + "/" + requestPath
	default:
		return basePath + requestPath
	}
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func removeHopByHopHeaders(header http.Header) {
	for _, connectionHeader := range header.Values("Connection") {
		for _, field := range strings.Split(connectionHeader, ",") {
			if trimmedField := strings.TrimSpace(field); trimmedField != "" {
				header.Del(trimmedField)
			}
		}
	}

	for _, key := range hopByHopHeaders {
		header.Del(key)
	}
}

var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}
