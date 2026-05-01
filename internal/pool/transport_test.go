package pool

import (
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

	if transport.MaxIdleConns != 3000 {
		t.Fatalf("transport.MaxIdleConns = %d, want %d", transport.MaxIdleConns, 3000)
	}
	if transport.MaxIdleConnsPerHost != 300 {
		t.Fatalf("transport.MaxIdleConnsPerHost = %d, want %d", transport.MaxIdleConnsPerHost, 300)
	}
	if transport.MaxConnsPerHost != 800 {
		t.Fatalf("transport.MaxConnsPerHost = %d, want %d", transport.MaxConnsPerHost, 800)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Fatalf("transport.IdleConnTimeout = %s, want %s", transport.IdleConnTimeout, 90*time.Second)
	}
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("transport.ResponseHeaderTimeout = %s, want %s", transport.ResponseHeaderTimeout, 30*time.Second)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Fatalf("transport.TLSHandshakeTimeout = %s, want %s", transport.TLSHandshakeTimeout, 10*time.Second)
	}
	if transport.ExpectContinueTimeout != time.Second {
		t.Fatalf("transport.ExpectContinueTimeout = %s, want %s", transport.ExpectContinueTimeout, time.Second)
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

	if err := HandleRequest("order-service", upstream.URL, recorder, request); err != nil {
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

func mustClient(t *testing.T, store *clientStore, decision Decision) *cachedClient {
	t.Helper()

	client, err := store.client(decision)
	if err != nil {
		t.Fatalf("client returned error: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}

	t.Cleanup(client.release)
	return client
}
