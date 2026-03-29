package client

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	authconfig "sidecargo/internal/auth/config"

	"golang.org/x/sync/singleflight"
)

// ProviderConfig 키 조회와 캐시 동작을 설정합니다.
type ProviderConfig struct {
	URL             string
	RequestTimeout  time.Duration
	RefreshInterval time.Duration
}

// Provider auth-service 엔드포인트에서 RSA 공개키를 조회하고 캐시합니다.
type Provider struct {
	fetcher         fetcher
	refreshInterval time.Duration
	now             func() time.Time

	mu          sync.RWMutex
	keys        map[string]*rsa.PublicKey
	refreshedAt time.Time

	refreshMu sync.Mutex
	sfGroup   singleflight.Group
}

// NewProvider 메모리 캐시를 사용하는 Provider를 생성합니다.
func NewProvider(cfg ProviderConfig) (*Provider, error) {
	trimmedURL := strings.TrimSpace(cfg.URL)
	if trimmedURL == "" {
		return nil, fmt.Errorf("%w: url is required", ErrInvalidProviderConfig)
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return nil, fmt.Errorf("%w: parse url: %w", ErrInvalidProviderConfig, err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("%w: url must include scheme and host", ErrInvalidProviderConfig)
	}

	if cfg.RequestTimeout <= 0 {
		return nil, fmt.Errorf("%w: request timeout must be greater than zero", ErrInvalidProviderConfig)
	}

	if cfg.RefreshInterval <= 0 {
		return nil, fmt.Errorf("%w: refresh interval must be greater than zero", ErrInvalidProviderConfig)
	}

	return &Provider{
		fetcher: fetcher{
			client: &http.Client{
				Timeout: cfg.RequestTimeout,
			},
			url: trimmedURL,
		},
		refreshInterval: cfg.RefreshInterval,
		now:             time.Now,
		keys:            make(map[string]*rsa.PublicKey),
	}, nil
}

// NewProviderFromEnvConfig 인증 환경 설정으로 Provider를 생성합니다.
func NewProviderFromEnvConfig(cfg authconfig.EnvConfig) (*Provider, error) {
	return NewProvider(ProviderConfig{
		URL:             cfg.AuthJWKSURL,
		RequestTimeout:  cfg.AuthJWKSRequestTimeout,
		RefreshInterval: cfg.AuthJWKSRefreshInterval,
	})
}

// PublicKey 주어진 kid에 해당하는 RSA 공개키를 반환합니다.
func (p *Provider) PublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	trimmedKeyID := strings.TrimSpace(kid)
	if trimmedKeyID == "" {
		return nil, fmt.Errorf("%w: kid is required", ErrInvalidKeyID)
	}

	cachedKey, found, expired := p.cachedKey(trimmedKeyID)
	if found && !expired {
		return cachedKey, nil
	}

	if err := p.Refresh(ctx); err != nil {
		if found {
			return cachedKey, nil
		}

		return nil, err
	}

	refreshedKey, refreshedFound, _ := p.cachedKey(trimmedKeyID)
	if !refreshedFound {
		return nil, fmt.Errorf("%w: kid %q", ErrKeyNotFound, trimmedKeyID)
	}

	return refreshedKey, nil
}

// Refresh 최신 키 세트 응답을 가져와 캐시를 교체합니다.
func (p *Provider) Refresh(ctx context.Context) error {
	_, err, _ := p.sfGroup.Do(JWKSRefreshSingleFlightKey, func() (interface{}, error) {
		keys, fetchErr := p.fetcher.fetch(ctx)
		if fetchErr != nil {
			return nil, fetchErr
		}

		p.mu.Lock()
		defer p.mu.Unlock()

		p.keys = keys
		p.refreshedAt = p.now()

		return nil, nil
	})

	return err
}

func (p *Provider) cachedKey(kid string) (*rsa.PublicKey, bool, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key, found := p.keys[kid]
	if !found {
		return nil, false, true
	}

	return key, true, p.cacheExpiredLocked()
}

func (p *Provider) cacheExpiredLocked() bool {
	if p.refreshedAt.IsZero() {
		return true
	}

	return p.now().Sub(p.refreshedAt) >= p.refreshInterval
}
