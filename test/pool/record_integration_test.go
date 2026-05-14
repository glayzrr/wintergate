package pool_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	internalconfig "wintergate/internal/config"
	internalpool "wintergate/internal/pool"
	"wintergate/test/harness"
)

func TestTrafficRecorderRecordsGatewayRequestLifecycle(t *testing.T) {
	upstream := newLifecycleUpstream(t)

	runtime := harness.NewRuntime()
	runtime.Register(t, serviceSettingsForEndpoint(t, "order-service", upstream.URL(), "/orders"))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	resultCh := receiveAsync(t, orchestrator, http.MethodGet, "/orders")
	waitForSignal(t, upstream.started, "upstream request")

	inFlightStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error while request in-flight: %v", err)
	}
	if inFlightStatus.InFlight != 1 {
		t.Fatalf("in-flight status InFlight = %d, want 1", inFlightStatus.InFlight)
	}
	if inFlightStatus.StartedRequests != 1 {
		t.Fatalf("in-flight status StartedRequests = %d, want 1", inFlightStatus.StartedRequests)
	}
	if inFlightStatus.FinishedRequests != 0 {
		t.Fatalf("in-flight status FinishedRequests = %d, want 0", inFlightStatus.FinishedRequests)
	}
	if inFlightStatus.RequestsInWindow != 1 {
		t.Fatalf("in-flight status RequestsInWindow = %d, want 1", inFlightStatus.RequestsInWindow)
	}
	if inFlightStatus.RPS <= 0 {
		t.Fatalf("in-flight status RPS = %f, want greater than 0", inFlightStatus.RPS)
	}

	upstream.release()
	result := waitForResult(t, resultCh, "gateway request")
	if result.err != nil {
		t.Fatalf("Receive returned error: %v", result.err)
	}
	if result.statusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", result.statusCode, http.StatusOK)
	}
	if result.body != "ok" {
		t.Fatalf("body = %q, want %q", result.body, "ok")
	}

	finishedStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error after request finished: %v", err)
	}
	if finishedStatus.InFlight != 0 {
		t.Fatalf("finished status InFlight = %d, want 0", finishedStatus.InFlight)
	}
	if finishedStatus.FinishedRequests != 1 {
		t.Fatalf("finished status FinishedRequests = %d, want 1", finishedStatus.FinishedRequests)
	}
	if finishedStatus.AverageLatency <= 0 {
		t.Fatalf("finished status AverageLatency = %s, want greater than 0", finishedStatus.AverageLatency)
	}
}

func TestTrafficRecorderAggregatesConcurrentGatewayRequests(t *testing.T) {
	const requestCount = 4

	upstream := newConcurrentBlockingUpstream(t, requestCount)

	runtime := harness.NewRuntime()
	runtime.Register(t, serviceSettingsForEndpoint(t, "order-service", upstream.URL(), "/orders"))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	resultChs := make([]<-chan receiveResult, 0, requestCount)
	for range requestCount {
		resultChs = append(resultChs, receiveAsync(t, orchestrator, http.MethodGet, "/orders"))
	}
	upstream.waitStarted(t)

	inFlightStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error while requests in-flight: %v", err)
	}
	if inFlightStatus.InFlight != requestCount {
		t.Fatalf("in-flight status InFlight = %d, want %d", inFlightStatus.InFlight, requestCount)
	}
	if inFlightStatus.StartedRequests != requestCount {
		t.Fatalf("in-flight status StartedRequests = %d, want %d", inFlightStatus.StartedRequests, requestCount)
	}
	if inFlightStatus.RequestsInWindow != requestCount {
		t.Fatalf("in-flight status RequestsInWindow = %d, want %d", inFlightStatus.RequestsInWindow, requestCount)
	}

	upstream.release()
	for index, resultCh := range resultChs {
		result := waitForResult(t, resultCh, fmt.Sprintf("gateway request %d", index))
		if result.err != nil {
			t.Fatalf("Receive returned error for request %d: %v", index, result.err)
		}
		if result.statusCode != http.StatusOK {
			t.Fatalf("status for request %d = %d, want %d", index, result.statusCode, http.StatusOK)
		}
	}

	finishedStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error after requests finished: %v", err)
	}
	if finishedStatus.InFlight != 0 {
		t.Fatalf("finished status InFlight = %d, want 0", finishedStatus.InFlight)
	}
	if finishedStatus.FinishedRequests != requestCount {
		t.Fatalf("finished status FinishedRequests = %d, want %d", finishedStatus.FinishedRequests, requestCount)
	}
}

func TestTrafficRecorderFinishesRequestWhenUpstreamFails(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	upstreamURL := upstream.URL
	upstream.Close()

	runtime := harness.NewRuntime()
	runtime.Register(t, serviceSettingsForEndpoint(t, "order-service", upstreamURL, "/orders"))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	result := receive(t, orchestrator, http.MethodGet, "/orders")
	if result.err == nil {
		t.Fatal("Receive returned nil error")
	}

	status, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error after upstream failure: %v", err)
	}
	if status.StartedRequests != 1 {
		t.Fatalf("status StartedRequests = %d, want 1", status.StartedRequests)
	}
	if status.FinishedRequests != 1 {
		t.Fatalf("status FinishedRequests = %d, want 1", status.FinishedRequests)
	}
	if status.InFlight != 0 {
		t.Fatalf("status InFlight = %d, want 0", status.InFlight)
	}
}

func TestTrafficRecorderSeparatesGatewayServices(t *testing.T) {
	orderUpstream := newLifecycleUpstream(t)
	paymentUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		if _, err := fmt.Fprint(w, "payment ok"); err != nil {
			t.Errorf("Write returned error: %v", err)
		}
	}))
	defer paymentUpstream.Close()

	runtime := harness.NewRuntime()
	runtime.Register(t, serviceSettingsForEndpoint(t, "order-service", orderUpstream.URL(), "/orders"))
	runtime.Register(t, serviceSettingsForEndpoint(t, "payment-service", paymentUpstream.URL, "/payments"))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	orderResultCh := receiveAsync(t, orchestrator, http.MethodGet, "/orders")
	waitForSignal(t, orderUpstream.started, "order upstream request")

	paymentResult := receive(t, orchestrator, http.MethodGet, "/payments")
	if paymentResult.err != nil {
		t.Fatalf("payment Receive returned error: %v", paymentResult.err)
	}
	if paymentResult.statusCode != http.StatusAccepted {
		t.Fatalf("payment status = %d, want %d", paymentResult.statusCode, http.StatusAccepted)
	}

	orderStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for order-service: %v", err)
	}
	if orderStatus.InFlight != 1 {
		t.Fatalf("order InFlight = %d, want 1", orderStatus.InFlight)
	}

	paymentStatus, err := trafficRecorder.StatusFor("payment-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for payment-service: %v", err)
	}
	if paymentStatus.InFlight != 0 {
		t.Fatalf("payment InFlight = %d, want 0", paymentStatus.InFlight)
	}
	if paymentStatus.FinishedRequests != 1 {
		t.Fatalf("payment FinishedRequests = %d, want 1", paymentStatus.FinishedRequests)
	}

	orderUpstream.release()
	orderResult := waitForResult(t, orderResultCh, "order gateway request")
	if orderResult.err != nil {
		t.Fatalf("order Receive returned error: %v", orderResult.err)
	}
}

type lifecycleUpstream struct {
	server    *httptest.Server
	started   chan struct{}
	releaseCh chan struct{}
	once      sync.Once
}

type concurrentBlockingUpstream struct {
	server      *httptest.Server
	releaseCh   chan struct{}
	startedCh   chan struct{}
	releaseOnce sync.Once
	started     atomic.Int64
	wantStarted int64
}

func newLifecycleUpstream(t *testing.T) *lifecycleUpstream {
	t.Helper()

	upstream := &lifecycleUpstream{
		started:   make(chan struct{}, 1),
		releaseCh: make(chan struct{}),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.started <- struct{}{}
		<-upstream.releaseCh
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, "ok"); err != nil {
			t.Errorf("Write returned error: %v", err)
		}
	}))
	upstream.server = server

	t.Cleanup(func() {
		upstream.release()
		server.Close()
	})

	return upstream
}

func (u *lifecycleUpstream) URL() string {
	return u.server.URL
}

func (u *lifecycleUpstream) release() {
	u.once.Do(func() {
		close(u.releaseCh)
	})
}

func newConcurrentBlockingUpstream(t *testing.T, requestCount int64) *concurrentBlockingUpstream {
	t.Helper()

	upstream := &concurrentBlockingUpstream{
		releaseCh:   make(chan struct{}),
		startedCh:   make(chan struct{}, 1),
		wantStarted: requestCount,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if upstream.started.Add(1) == upstream.wantStarted {
			upstream.startedCh <- struct{}{}
		}
		<-upstream.releaseCh
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, "ok"); err != nil {
			t.Errorf("Write returned error: %v", err)
		}
	}))
	upstream.server = server

	t.Cleanup(func() {
		upstream.release()
		server.Close()
	})

	return upstream
}

func (u *concurrentBlockingUpstream) URL() string {
	return u.server.URL
}

func (u *concurrentBlockingUpstream) waitStarted(t *testing.T) {
	t.Helper()

	waitForSignal(t, u.startedCh, "concurrent upstream requests")
}

func (u *concurrentBlockingUpstream) release() {
	u.releaseOnce.Do(func() {
		close(u.releaseCh)
	})
}

func serviceSettingsForEndpoint(t *testing.T, serviceName, upstreamURL, path string) internalconfig.Settings {
	t.Helper()

	return harness.ServiceSettings(
		serviceName,
		harness.InstanceFromURL(t, upstreamURL),
		[]internalconfig.EndpointSettings{
			{
				Path:   path,
				Method: http.MethodGet,
			},
		},
	)
}
