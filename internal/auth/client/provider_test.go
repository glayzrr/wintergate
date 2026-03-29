package client

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	authconfig "sidecargo/internal/auth/config"
)

func TestNewProviderFromEnvConfig(t *testing.T) {
	provider, err := NewProviderFromEnvConfig(authconfig.EnvConfig{
		AuthJWKSURL:             "https://auth-service.local/.well-known/jwks.json",
		AuthJWKSRequestTimeout:  3 * time.Second,
		AuthJWKSRefreshInterval: 10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewProviderFromEnvConfig returned error: %v", err)
	}

	if provider.fetcher.url != "https://auth-service.local/.well-known/jwks.json" {
		t.Fatalf("fetcher.url = %q, want %q", provider.fetcher.url, "https://auth-service.local/.well-known/jwks.json")
	}

	if provider.fetcher.client.Timeout != 3*time.Second {
		t.Fatalf("client timeout = %s, want %s", provider.fetcher.client.Timeout, 3*time.Second)
	}

	if provider.refreshInterval != 10*time.Minute {
		t.Fatalf("refreshInterval = %s, want %s", provider.refreshInterval, 10*time.Minute)
	}
}

func TestNewProviderReturnsErrorWhenConfigInvalid(t *testing.T) {
	testCases := []struct {
		name string
		cfg  ProviderConfig
	}{
		{
			name: "empty url",
			cfg: ProviderConfig{
				RequestTimeout:  time.Second,
				RefreshInterval: time.Minute,
			},
		},
		{
			name: "invalid url",
			cfg: ProviderConfig{
				URL:             "://invalid",
				RequestTimeout:  time.Second,
				RefreshInterval: time.Minute,
			},
		},
		{
			name: "missing host",
			cfg: ProviderConfig{
				URL:             "https:///missing-host",
				RequestTimeout:  time.Second,
				RefreshInterval: time.Minute,
			},
		},
		{
			name: "zero request timeout",
			cfg: ProviderConfig{
				URL:             "https://auth-service.local/keys",
				RequestTimeout:  0,
				RefreshInterval: time.Minute,
			},
		},
		{
			name: "zero refresh interval",
			cfg: ProviderConfig{
				URL:             "https://auth-service.local/keys",
				RequestTimeout:  time.Second,
				RefreshInterval: 0,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewProvider(testCase.cfg)
			if err == nil {
				t.Fatal("NewProvider returned nil error")
			}

			if !errors.Is(err, ErrInvalidProviderConfig) {
				t.Fatalf("error = %v, want ErrInvalidProviderConfig", err)
			}
		})
	}
}

func TestProviderPublicKeyFetchesAndCaches(t *testing.T) {
	privateKey := generateRSAKey(t)
	service := newTestKeyService(t, mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey)))
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)

	firstKey, err := provider.PublicKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error: %v", err)
	}

	secondKey, err := provider.PublicKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error on cached lookup: %v", err)
	}

	if !equalPublicKeys(firstKey, &privateKey.PublicKey) {
		t.Fatal("firstKey does not match the served key")
	}

	if !equalPublicKeys(secondKey, &privateKey.PublicKey) {
		t.Fatal("secondKey does not match the cached key")
	}

	if service.requests() != 1 {
		t.Fatalf("requests = %d, want %d", service.requests(), 1)
	}
}

func TestProviderPublicKeyRefreshesExpiredCache(t *testing.T) {
	firstPrivateKey := generateRSAKey(t)
	secondPrivateKey := generateRSAKey(t)
	service := newTestKeyService(t, mustMarshalDocument(t, newRSAKey("key-1", &firstPrivateKey.PublicKey)))
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)
	now := time.Unix(100, 0)
	provider.now = func() time.Time {
		return now
	}

	initialKey, err := provider.PublicKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error: %v", err)
	}

	service.setResponse(http.StatusOK, mustMarshalDocument(t, newRSAKey("key-1", &secondPrivateKey.PublicKey)), 0)
	now = now.Add(time.Minute + time.Second)

	refreshedKey, err := provider.PublicKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error after refresh: %v", err)
	}

	if !equalPublicKeys(initialKey, &firstPrivateKey.PublicKey) {
		t.Fatal("initialKey does not match the first served key")
	}

	if !equalPublicKeys(refreshedKey, &secondPrivateKey.PublicKey) {
		t.Fatal("refreshedKey does not match the refreshed served key")
	}

	if service.requests() != 2 {
		t.Fatalf("requests = %d, want %d", service.requests(), 2)
	}
}

func TestProviderPublicKeyReturnsCachedKeyWhenRefreshFails(t *testing.T) {
	privateKey := generateRSAKey(t)
	service := newTestKeyService(t, mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey)))
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)
	now := time.Unix(200, 0)
	provider.now = func() time.Time {
		return now
	}

	cachedKey, err := provider.PublicKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error: %v", err)
	}

	service.setResponse(http.StatusInternalServerError, `{"error":"upstream failed"}`, 0)
	now = now.Add(time.Minute + time.Second)

	keyAfterFailedRefresh, err := provider.PublicKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error after failed refresh: %v", err)
	}

	if !equalPublicKeys(cachedKey, &privateKey.PublicKey) {
		t.Fatal("cachedKey does not match the served key")
	}

	if !equalPublicKeys(keyAfterFailedRefresh, &privateKey.PublicKey) {
		t.Fatal("keyAfterFailedRefresh does not match the cached key")
	}

	if service.requests() != 2 {
		t.Fatalf("requests = %d, want %d", service.requests(), 2)
	}
}

func TestProviderPublicKeyReturnsErrorWhenKeyNotFound(t *testing.T) {
	privateKey := generateRSAKey(t)
	service := newTestKeyService(t, mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey)))
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)

	_, err := provider.PublicKey(context.Background(), "missing-key")
	if err == nil {
		t.Fatal("PublicKey returned nil error")
	}

	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("error = %v, want ErrKeyNotFound", err)
	}
}

func TestProviderPublicKeyReturnsErrorWhenKeyIDEmpty(t *testing.T) {
	provider := newTestProvider(t, "https://auth-service.local/keys", time.Second, time.Minute)

	_, err := provider.PublicKey(context.Background(), "   ")
	if err == nil {
		t.Fatal("PublicKey returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeyID) {
		t.Fatalf("error = %v, want ErrInvalidKeyID", err)
	}
}

func TestProviderRefreshReturnsErrorWhenPayloadInvalid(t *testing.T) {
	service := newTestKeyService(t, `{"keys":[{"kid":"key-1","kty":"RSA","alg":"RS256","use":"sig","e":"AQAB"}]}`)
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)

	err := provider.Refresh(context.Background())
	if err == nil {
		t.Fatal("Refresh returned nil error")
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}

func TestProviderRefreshReturnsErrorWhenRequestTimesOut(t *testing.T) {
	privateKey := generateRSAKey(t)
	body := mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey))
	service := newTestKeyService(t, body)
	service.setResponse(http.StatusOK, body, 50*time.Millisecond)
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, 10*time.Millisecond, time.Minute)

	err := provider.Refresh(context.Background())
	if err == nil {
		t.Fatal("Refresh returned nil error")
	}

	if !errors.Is(err, ErrKeyFetchFailed) {
		t.Fatalf("error = %v, want ErrKeyFetchFailed", err)
	}
}

func TestProviderRefreshDeduplicatesConcurrentCalls(t *testing.T) {
	privateKey := generateRSAKey(t)
	body := mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey))
	service := newTestKeyService(t, body)
	service.setResponse(http.StatusOK, body, 50*time.Millisecond)
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)

	results := runConcurrentRefreshes(provider, 24)
	for index, err := range results {
		if err != nil {
			t.Fatalf("results[%d] returned error: %v", index, err)
		}
	}

	if service.requests() != 1 {
		t.Fatalf("requests = %d, want %d", service.requests(), 1)
	}
}

func TestProviderRefreshDeduplicatesConcurrentFailures(t *testing.T) {
	service := newTestKeyService(t, `{"error":"failed"}`)
	service.setResponse(http.StatusInternalServerError, `{"error":"failed"}`, 50*time.Millisecond)
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)

	results := runConcurrentRefreshes(provider, 24)
	for index, err := range results {
		if err == nil {
			t.Fatalf("results[%d] returned nil error", index)
		}

		if !errors.Is(err, ErrKeyFetchFailed) {
			t.Fatalf("results[%d] error = %v, want ErrKeyFetchFailed", index, err)
		}
	}

	if service.requests() != 1 {
		t.Fatalf("requests = %d, want %d", service.requests(), 1)
	}
}

func TestProviderPublicKeyDeduplicatesConcurrentExpiredRefresh(t *testing.T) {
	firstPrivateKey := generateRSAKey(t)
	secondPrivateKey := generateRSAKey(t)
	service := newTestKeyService(t, mustMarshalDocument(t, newRSAKey("key-1", &firstPrivateKey.PublicKey)))
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)
	now := time.Unix(300, 0)
	provider.now = func() time.Time {
		return now
	}

	initialKey, err := provider.PublicKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error: %v", err)
	}

	if !equalPublicKeys(initialKey, &firstPrivateKey.PublicKey) {
		t.Fatal("initialKey does not match the first served key")
	}

	service.setResponse(http.StatusOK, mustMarshalDocument(t, newRSAKey("key-1", &secondPrivateKey.PublicKey)), 50*time.Millisecond)
	now = now.Add(time.Minute + time.Second)

	results := runConcurrentPublicKeys(provider, "key-1", 24)
	for index, result := range results {
		if result.err != nil {
			t.Fatalf("results[%d] returned error: %v", index, result.err)
		}

		if !equalPublicKeys(result.key, &secondPrivateKey.PublicKey) {
			t.Fatalf("results[%d] returned stale key", index)
		}
	}

	if service.requests() != 2 {
		t.Fatalf("requests = %d, want %d", service.requests(), 2)
	}
}

func TestProviderPublicKeyReturnsCachedKeyWhenConcurrentRefreshFails(t *testing.T) {
	privateKey := generateRSAKey(t)
	service := newTestKeyService(t, mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey)))
	server := httptest.NewServer(service)
	defer server.Close()

	provider := newTestProvider(t, server.URL, time.Second, time.Minute)
	now := time.Unix(400, 0)
	provider.now = func() time.Time {
		return now
	}

	cachedKey, err := provider.PublicKey(context.Background(), "key-1")
	if err != nil {
		t.Fatalf("PublicKey returned error: %v", err)
	}

	service.setResponse(http.StatusInternalServerError, `{"error":"failed"}`, 50*time.Millisecond)
	now = now.Add(time.Minute + time.Second)

	results := runConcurrentPublicKeys(provider, "key-1", 24)
	for index, result := range results {
		if result.err != nil {
			t.Fatalf("results[%d] returned error: %v", index, result.err)
		}

		if !equalPublicKeys(result.key, cachedKey) {
			t.Fatalf("results[%d] did not return the cached key", index)
		}
	}

	if service.requests() != 2 {
		t.Fatalf("requests = %d, want %d", service.requests(), 2)
	}
}

type testKeyService struct {
	bodyValue  string
	delay      time.Duration
	requested  int
	statusCode int
	testingT   *testing.T
	mutex      sync.RWMutex
}

func newTestKeyService(t *testing.T, body string) *testKeyService {
	t.Helper()

	return &testKeyService{
		bodyValue:  body,
		statusCode: http.StatusOK,
		testingT:   t,
	}
}

func (s *testKeyService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.mutex.Lock()
	s.requested++
	statusCode := s.statusCode
	bodyValue := s.bodyValue
	delay := s.delay
	s.mutex.Unlock()

	if request.Method != http.MethodGet {
		s.testingT.Errorf("request method = %s, want %s", request.Method, http.MethodGet)
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	writer.WriteHeader(statusCode)
	if _, err := writer.Write([]byte(bodyValue)); err != nil {
		s.testingT.Errorf("write response: %v", err)
	}
}

func (s *testKeyService) setResponse(statusCode int, body string, delay time.Duration) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.statusCode = statusCode
	s.bodyValue = body
	s.delay = delay
}

func (s *testKeyService) requests() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.requested
}

func newTestProvider(t *testing.T, url string, requestTimeout time.Duration, refreshInterval time.Duration) *Provider {
	t.Helper()

	provider, err := NewProvider(ProviderConfig{
		URL:             url,
		RequestTimeout:  requestTimeout,
		RefreshInterval: refreshInterval,
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	return provider
}

type publicKeyResult struct {
	err error
	key *rsa.PublicKey
}

func runConcurrentRefreshes(provider *Provider, workers int) []error {
	start := make(chan struct{})
	results := make(chan error, workers)
	var waitGroup sync.WaitGroup

	waitGroup.Add(workers)
	for index := 0; index < workers; index++ {
		go func() {
			defer waitGroup.Done()
			<-start

			results <- provider.Refresh(context.Background())
		}()
	}

	close(start)
	waitGroup.Wait()
	close(results)

	collected := make([]error, 0, workers)
	for result := range results {
		collected = append(collected, result)
	}

	return collected
}

func runConcurrentPublicKeys(provider *Provider, keyID string, workers int) []publicKeyResult {
	start := make(chan struct{})
	results := make(chan publicKeyResult, workers)
	var waitGroup sync.WaitGroup

	waitGroup.Add(workers)
	for index := 0; index < workers; index++ {
		go func() {
			defer waitGroup.Done()
			<-start

			keyValue, err := provider.PublicKey(context.Background(), keyID)
			results <- publicKeyResult{
				err: err,
				key: keyValue,
			}
		}()
	}

	close(start)
	waitGroup.Wait()
	close(results)

	collected := make([]publicKeyResult, 0, workers)
	for result := range results {
		collected = append(collected, result)
	}

	return collected
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	return privateKey
}

func mustMarshalDocument(t *testing.T, keys ...key) string {
	t.Helper()

	payload, err := json.Marshal(document{Keys: keys})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	return string(payload)
}

func newRSAKey(keyID string, publicKey *rsa.PublicKey) key {
	return key{
		Algorithm: supportedAlgorithm,
		Exponent:  base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
		KeyID:     keyID,
		KeyType:   "RSA",
		Modulus:   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		Use:       "sig",
	}
}

func equalPublicKeys(left *rsa.PublicKey, right *rsa.PublicKey) bool {
	if left == nil || right == nil {
		return false
	}

	return left.E == right.E && left.N.Cmp(right.N) == 0
}
