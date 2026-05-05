package pool

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewTransportAppliesTierConfig(t *testing.T) {
	transport, err := NewTransport(TierSuper)
	if err != nil {
		t.Fatalf("NewTransport returned error: %v", err)
	}

	if transport.MaxIdleConns != 400 {
		t.Fatalf("transport.MaxIdleConns = %d, want %d", transport.MaxIdleConns, 400)
	}
	if transport.MaxIdleConnsPerHost != 8 {
		t.Fatalf("transport.MaxIdleConnsPerHost = %d, want %d", transport.MaxIdleConnsPerHost, 8)
	}
	if transport.MaxConnsPerHost != 0 {
		t.Fatalf("transport.MaxConnsPerHost = %d, want %d", transport.MaxConnsPerHost, 0)
	}
	if transport.IdleConnTimeout != 360*time.Second {
		t.Fatalf("transport.IdleConnTimeout = %s, want %s", transport.IdleConnTimeout, 360*time.Second)
	}
	if transport.ResponseHeaderTimeout != 0 {
		t.Fatalf("transport.ResponseHeaderTimeout = %s, want %s", transport.ResponseHeaderTimeout, time.Duration(0))
	}
	if transport.TLSHandshakeTimeout != 40*time.Second {
		t.Fatalf("transport.TLSHandshakeTimeout = %s, want %s", transport.TLSHandshakeTimeout, 40*time.Second)
	}
	if transport.ExpectContinueTimeout != 4*time.Second {
		t.Fatalf("transport.ExpectContinueTimeout = %s, want %s", transport.ExpectContinueTimeout, 4*time.Second)
	}
}

func TestNewTransportClonesDefaultTransport(t *testing.T) {
	transport, err := NewTransport("")
	if err != nil {
		t.Fatalf("NewTransport returned error: %v", err)
	}

	defaultTransport := http.DefaultTransport.(*http.Transport)
	if transport == defaultTransport {
		t.Fatal("NewTransport returned http.DefaultTransport")
	}
	if transport.Proxy == nil {
		t.Fatal("transport.Proxy is nil, want cloned default proxy function")
	}
	if transport.DialContext == nil {
		t.Fatal("transport.DialContext is nil, want cloned default dialer")
	}
}

func TestHandleRequestForwardsUpstreamResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/orders" {
			t.Fatalf("path = %q, want %q", r.URL.Path, "/orders")
		}
		if r.URL.RawQuery != "page=1" {
			t.Fatalf("raw query = %q, want %q", r.URL.RawQuery, "page=1")
		}
		if r.Header.Get("X-Request-ID") != "request-1" {
			t.Fatalf("X-Request-ID = %q, want %q", r.Header.Get("X-Request-ID"), "request-1")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll returned error: %v", err)
		}
		if string(body) != `{"id":1}` {
			t.Fatalf("body = %q, want %q", string(body), `{"id":1}`)
		}

		w.Header().Set("X-Upstream", "order")
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte("created")); err != nil {
			t.Fatalf("Write returned error: %v", err)
		}
	}))
	defer upstream.Close()

	request := httptest.NewRequest(http.MethodPost, "/orders?page=1", strings.NewReader(`{"id":1}`))
	request.Header.Set("X-Request-ID", "request-1")
	recorder := httptest.NewRecorder()

	if err := HandleRequest("order-service", upstream.URL, recorder, request, nil); err != nil {
		t.Fatalf("HandleRequest returned error: %v", err)
	}

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if response.Header.Get("X-Upstream") != "order" {
		t.Fatalf("X-Upstream = %q, want %q", response.Header.Get("X-Upstream"), "order")
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(body) != "created" {
		t.Fatalf("body = %q, want %q", string(body), "created")
	}
}

func TestHandleRequestReleasesDedicatedTierClientAfterContextTimeout(t *testing.T) {
	resetDefaultState(t)

	if err := RegisterPolicies([]Policy{
		{
			Service: "order-service",
			Hot:     Threshold{InFlight: 1},
		},
	}); err != nil {
		t.Fatalf("RegisterPolicies returned error: %v", err)
	}

	upstreamStarted := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamStarted <- struct{}{}
		<-r.Context().Done()
	}))
	defer upstream.Close()

	errCh := handleRequestAsync(t, "order-service", upstream.URL, 200*time.Millisecond)
	waitForSignal(t, upstreamStarted, "first upstream request")

	hotClient := dedicatedCachedClient(t, defaultClients, "order-service")
	if hotClient.tier != TierHot {
		t.Fatalf("dedicated tier = %q, want %q", hotClient.tier, TierHot)
	}
	hotClientDone := waitCachedClient(hotClient)

	assertStillWaiting(t, hotClientDone, 20*time.Millisecond, "dedicated client drained before the in-flight request timed out")

	err := waitForRequestError(t, errCh)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("HandleRequest error = %v, want context deadline exceeded", err)
	}

	waitForDone(t, hotClientDone, time.Second, "dedicated client wait group")

	status, err := StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error: %v", err)
	}
	if status.InFlight != 0 {
		t.Fatalf("status.InFlight = %d, want 0", status.InFlight)
	}
}

func TestClientStoreReusesSharedClient(t *testing.T) {
	store := newClientStore()
	sharedCount, dedicatedCount := store.count()
	if sharedCount != 3 {
		t.Fatalf("sharedCount = %d, want %d", sharedCount, 3)
	}
	if dedicatedCount != 0 {
		t.Fatalf("dedicatedCount = %d, want %d", dedicatedCount, 0)
	}

	decision := Decision{
		Service: "order-service",
		Tier:    TierNormal,
	}

	firstClient := mustClient(t, store, decision)
	secondClient := mustClient(t, store, decision)

	if firstClient != secondClient {
		t.Fatal("client store did not reuse shared client")
	}
}

func TestClientStoreUsesSharedTierClients(t *testing.T) {
	store := newClientStore()

	normalClient := mustClient(t, store, Decision{
		Service: "order-service",
		Tier:    TierNormal,
	})
	hotClient := mustClient(t, store, Decision{
		Service: "payment-service",
		Tier:    TierHot,
	})

	if normalClient == hotClient {
		t.Fatal("client store reused shared client across tiers")
	}
	if sharedCount, dedicatedCount := store.count(); sharedCount != 3 || dedicatedCount != 0 {
		t.Fatalf("count = (%d, %d), want (%d, %d)", sharedCount, dedicatedCount, 3, 0)
	}
}

func TestClientStoreSeparatesDedicatedClients(t *testing.T) {
	store := newClientStore()

	orderClient := mustClient(t, store, Decision{
		Service:   "order-service",
		Tier:      TierHot,
		Dedicated: true,
	})
	paymentClient := mustClient(t, store, Decision{
		Service:   "payment-service",
		Tier:      TierHot,
		Dedicated: true,
	})

	if orderClient == paymentClient {
		t.Fatal("client store reused dedicated client across services")
	}

	if sharedCount, dedicatedCount := store.count(); sharedCount != 3 || dedicatedCount != 2 {
		t.Fatalf("count = (%d, %d), want (%d, %d)", sharedCount, dedicatedCount, 3, 2)
	}
}

func TestClientStoreReplacesDedicatedClientWhenTierChanges(t *testing.T) {
	store := newClientStore()

	hotClient := mustClient(t, store, Decision{
		Service:   "order-service",
		Tier:      TierHot,
		Dedicated: true,
	})
	superClient := mustClient(t, store, Decision{
		Service:   "order-service",
		Tier:      TierSuper,
		Dedicated: true,
	})

	if hotClient == superClient {
		t.Fatal("client store reused dedicated client after tier changed")
	}
	if tier, found := store.dedicatedTier("order-service"); !found || tier != TierSuper {
		t.Fatalf("dedicated tier = (%q, %t), want (%q, %t)", tier, found, TierSuper, true)
	}
	if sharedCount, dedicatedCount := store.count(); sharedCount != 3 || dedicatedCount != 1 {
		t.Fatalf("count = (%d, %d), want (%d, %d)", sharedCount, dedicatedCount, 3, 1)
	}
}

func TestClientStoreReleasesDedicatedClientWhenDecisionIsShared(t *testing.T) {
	store := newClientStore()

	dedicatedClient := mustClient(t, store, Decision{
		Service:   "order-service",
		Tier:      TierHot,
		Dedicated: true,
	})
	sharedHotClient, found := store.sharedClientForTier(TierHot)
	if !found {
		t.Fatal("shared hot client not found")
	}

	client := mustClient(t, store, Decision{
		Service: "order-service",
		Tier:    TierHot,
	})

	if client.client != sharedHotClient {
		t.Fatal("client store did not return shared client after dedicated release")
	}
	if client == dedicatedClient {
		t.Fatal("client store kept using dedicated client after release")
	}
	if _, found := store.dedicatedTier("order-service"); found {
		t.Fatal("dedicated client still exists after shared decision")
	}
	if sharedCount, dedicatedCount := store.count(); sharedCount != 3 || dedicatedCount != 0 {
		t.Fatalf("count = (%d, %d), want (%d, %d)", sharedCount, dedicatedCount, 3, 0)
	}
}

func resetDefaultState(t *testing.T) {
	t.Helper()

	previousClients := defaultClients
	previousRecorder := defaultRecorder
	previousPolicyRegistry := defaultPolicyRegistry

	defaultClients = newClientStore()
	defaultRecorder = NewRecorder()
	defaultPolicyRegistry = NewPolicyRegistry()

	t.Cleanup(func() {
		defaultClients = previousClients
		defaultRecorder = previousRecorder
		defaultPolicyRegistry = previousPolicyRegistry
	})
}

func handleRequestAsync(t *testing.T, service, host string, timeout time.Duration) <-chan error {
	t.Helper()

	errCh := make(chan error, 1)
	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	ctx, cancel := context.WithTimeout(request.Context(), timeout)
	request = request.WithContext(ctx)
	recorder := httptest.NewRecorder()

	go func() {
		defer cancel()
		errCh <- HandleRequest(service, host, recorder, request, nil)
	}()

	return errCh
}

func sharedCachedClient(t *testing.T, store *clientStore, tier Tier) *cachedClient {
	t.Helper()

	store.mu.RLock()
	defer store.mu.RUnlock()

	client := store.shared[tier]
	if client == nil {
		t.Fatalf("shared client for %q not found", tier)
	}

	return client
}

func dedicatedCachedClient(t *testing.T, store *clientStore, service string) *cachedClient {
	t.Helper()

	store.mu.RLock()
	defer store.mu.RUnlock()

	client := store.dedicated[normalizeService(service)]
	if client == nil {
		t.Fatalf("dedicated client for %q not found", service)
	}

	return client
}

func waitCachedClient(client *cachedClient) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		client.wg.Wait()
		close(done)
	}()

	return done
}

func waitForSignal(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func assertStillWaiting(t *testing.T, ch <-chan struct{}, duration time.Duration, message string) {
	t.Helper()

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ch:
		t.Fatal(message)
	case <-timer.C:
	}
}

func waitForDone(t *testing.T, ch <-chan struct{}, timeout time.Duration, name string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func waitForRequestError(t *testing.T, errCh <-chan error) error {
	t.Helper()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("HandleRequest returned nil error, want timeout error")
		}
		return err
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for HandleRequest error")
	}

	return nil
}

func mustClient(t *testing.T, store *clientStore, decision Decision) *cachedClient {
	t.Helper()

	client, err := store.ClientFor(decision)
	if err != nil {
		t.Fatalf("client returned error: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}

	t.Cleanup(client.release)
	return client
}
