package config

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Entry 하나의 URL 경로와 대상 서비스 매핑을 표현합니다.
type Entry struct {
	Path    string
	Service string
}

// RuntimeConfig 라우팅 런타임 설정과 엔트리를 함께 보관합니다.
type RuntimeConfig struct {
	RouteServiceHeader          string
	RouteUpstreamRequestTimeout time.Duration
	Entries                     []Entry
}

// Registry 라우팅 런타임 설정과 엔트리를 메모리에 보관합니다.
type Registry struct {
	mu     sync.RWMutex
	config RuntimeConfig
	set    bool
}

// NewRegistry 빈 라우팅 설정 Registry를 생성합니다.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register 전달받은 라우팅 설정과 엔트리로 현재 값을 교체합니다.
func (r *Registry) Register(cfg RuntimeConfig) error {
	if strings.TrimSpace(cfg.RouteServiceHeader) == "" {
		return fmt.Errorf("%w: route_service_header is required", ErrInvalidConfig)
	}

	if cfg.RouteUpstreamRequestTimeout <= 0 {
		return fmt.Errorf("%w: route_upstream_request_timeout must be greater than zero", ErrInvalidConfig)
	}

	if len(cfg.Entries) == 0 {
		return fmt.Errorf("%w: entries are required", ErrInvalidConfig)
	}

	registeredEntries := make(map[string]string, len(cfg.Entries))
	for _, entry := range cfg.Entries {
		path := strings.TrimSpace(entry.Path)
		if path == "" {
			return fmt.Errorf("%w: path is required", ErrInvalidConfig)
		}

		service := strings.TrimSpace(entry.Service)
		if service == "" {
			return fmt.Errorf("%w: service is required for path %q", ErrInvalidConfig, path)
		}

		if _, exists := registeredEntries[path]; exists {
			return fmt.Errorf("%w: duplicate path %q", ErrInvalidConfig, path)
		}

		registeredEntries[path] = service
	}

	normalizedEntries := make([]Entry, 0, len(cfg.Entries))
	for path, service := range registeredEntries {
		normalizedEntries = append(normalizedEntries, Entry{
			Path:    path,
			Service: service,
		})
	}

	sort.Slice(normalizedEntries, func(left int, right int) bool {
		return normalizedEntries[left].Path < normalizedEntries[right].Path
	})

	r.mu.Lock()
	defer r.mu.Unlock()

	r.config = RuntimeConfig{
		RouteServiceHeader:          strings.TrimSpace(cfg.RouteServiceHeader),
		RouteUpstreamRequestTimeout: cfg.RouteUpstreamRequestTimeout,
		Entries:                     normalizedEntries,
	}
	r.set = true

	return nil
}

// Service 지정한 경로에 대응하는 서비스 이름을 반환합니다.
func (r *Registry) Service(path string) (string, bool) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.config.Entries {
		if entry.Path == trimmedPath {
			return entry.Service, true
		}
	}

	return "", false
}

// Snapshot 현재 라우팅 런타임 설정의 사본을 반환합니다.
func (r *Registry) Snapshot() (RuntimeConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.set {
		return RuntimeConfig{}, false
	}

	cfg := r.config
	cfg.Entries = append([]Entry(nil), r.config.Entries...)

	return cfg, true
}
