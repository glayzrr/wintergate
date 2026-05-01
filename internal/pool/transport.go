package pool

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var defaultClients = newClientStore()

type clientStore struct {
	clients map[clientKey]*http.Client

	// mu는 tier/service별 http.Client 캐시의 동시 조회와 생성을 보호합니다.
	mu sync.RWMutex
}

type clientKey struct {
	service   string
	tier      Tier
	dedicated bool
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
	client, err := defaultClients.client(decision)
	if err != nil {
		return err
	}

	outReq, err := upstreamRequest(host, r)
	if err != nil {
		return err
	}

	resp, err := client.Do(outReq)
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
	return &clientStore{
		clients: make(map[clientKey]*http.Client),
	}
}

func (s *clientStore) client(decision Decision) (*http.Client, error) {
	if s == nil {
		return nil, fmt.Errorf("%w: client store is nil", ErrInvalidConfig)
	}

	key, err := clientStoreKey(decision)
	if err != nil {
		return nil, err
	}

	if client, found := s.find(key); found {
		return client, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if client, found := s.clients[key]; found {
		return client, nil
	}

	transport, err := NewTransport(key.tier)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: transport,
	}
	s.clients[key] = client

	return client, nil
}

func (s *clientStore) find(key clientKey) (*http.Client, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, found := s.clients[key]

	return client, found
}

func clientStoreKey(decision Decision) (clientKey, error) {
	tier := decision.Tier
	if strings.TrimSpace(string(tier)) == "" {
		tier = TierNormal
	}

	key := clientKey{
		tier:      tier,
		dedicated: decision.Dedicated,
	}
	if decision.Dedicated {
		key.service = normalizeService(decision.Service)
		if key.service == "" {
			return clientKey{}, fmt.Errorf("%w: service is required for dedicated pool", ErrInvalidService)
		}
	}

	return key, nil
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
