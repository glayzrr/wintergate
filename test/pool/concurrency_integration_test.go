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

func TestPoolRuntimeHandlesConcurrentRequestsAndStatusReads(t *testing.T) {
	const requestCount = 32

	upstream := newGateUpstream(t, requestCount)

	runtime := harness.NewRuntime()
	runtime.Register(t, harness.ServiceSettings(
		"order-service",
		harness.InstanceFromURL(t, upstream.URL()),
		[]internalconfig.EndpointSettings{
			{
				Path:   "/orders",
				Method: http.MethodGet,
			},
		},
		harness.WithPoolThresholds(
			internalconfig.ThresholdPoint{},
			internalconfig.ThresholdPoint{InFlight: 4},
			internalconfig.ThresholdPoint{InFlight: 16},
		),
	))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	resultChs := make([]<-chan receiveResult, 0, requestCount)
	for range requestCount {
		resultChs = append(resultChs, receiveAsync(t, orchestrator, http.MethodGet, "/orders"))
	}
	upstream.waitStarted(t)

	stopReaders := make(chan struct{})
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stopReaders:
				return
			default:
				status, err := trafficRecorder.StatusFor("order-service")
				if err == nil {
					_ = runtime.PoolStore.AssignmentFor(runtime.Manager.Settings(), status)
				}
			}
		}
	}()

	inFlightStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		close(stopReaders)
		<-readerDone
		t.Fatalf("StatusFor returned error while requests in-flight: %v", err)
	}
	if inFlightStatus.InFlight != requestCount {
		close(stopReaders)
		<-readerDone
		t.Fatalf("in-flight status InFlight = %d, want %d", inFlightStatus.InFlight, requestCount)
	}

	upstream.release()
	for index, resultCh := range resultChs {
		result := waitForResult(t, resultCh, fmt.Sprintf("gateway request %d", index))
		if result.err != nil {
			close(stopReaders)
			<-readerDone
			t.Fatalf("Receive returned error for request %d: %v", index, result.err)
		}
		if result.statusCode != http.StatusOK {
			close(stopReaders)
			<-readerDone
			t.Fatalf("status for request %d = %d, want %d", index, result.statusCode, http.StatusOK)
		}
	}

	close(stopReaders)
	<-readerDone

	finishedStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error after requests finished: %v", err)
	}
	if finishedStatus.InFlight != 0 {
		t.Fatalf("finished status InFlight = %d, want 0", finishedStatus.InFlight)
	}
	if finishedStatus.StartedRequests != requestCount {
		t.Fatalf("finished status StartedRequests = %d, want %d", finishedStatus.StartedRequests, requestCount)
	}
	if finishedStatus.FinishedRequests != requestCount {
		t.Fatalf("finished status FinishedRequests = %d, want %d", finishedStatus.FinishedRequests, requestCount)
	}
}

func TestPoolRuntimeHandlesConcurrentRequestsForMultipleServices(t *testing.T) {
	const requestsPerService = 12

	orderUpstream := newGateUpstream(t, requestsPerService)
	paymentUpstream := newGateUpstream(t, requestsPerService)

	runtime := harness.NewRuntime()
	runtime.Register(t, harness.ServiceSettings(
		"order-service",
		harness.InstanceFromURL(t, orderUpstream.URL()),
		[]internalconfig.EndpointSettings{
			{
				Path:   "/orders",
				Method: http.MethodGet,
			},
		},
		harness.WithPoolThresholds(
			internalconfig.ThresholdPoint{},
			internalconfig.ThresholdPoint{InFlight: 4},
			internalconfig.ThresholdPoint{InFlight: 100},
		),
	))
	runtime.Register(t, harness.ServiceSettings(
		"payment-service",
		harness.InstanceFromURL(t, paymentUpstream.URL()),
		[]internalconfig.EndpointSettings{
			{
				Path:   "/payments",
				Method: http.MethodGet,
			},
		},
		harness.WithPoolThresholds(
			internalconfig.ThresholdPoint{},
			internalconfig.ThresholdPoint{InFlight: 4},
			internalconfig.ThresholdPoint{InFlight: 100},
		),
	))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	orderResults := make([]<-chan receiveResult, 0, requestsPerService)
	paymentResults := make([]<-chan receiveResult, 0, requestsPerService)
	for range requestsPerService {
		orderResults = append(orderResults, receiveAsync(t, orchestrator, http.MethodGet, "/orders"))
		paymentResults = append(paymentResults, receiveAsync(t, orchestrator, http.MethodGet, "/payments"))
	}
	orderUpstream.waitStarted(t)
	paymentUpstream.waitStarted(t)

	orderStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for order-service: %v", err)
	}
	if orderStatus.InFlight != requestsPerService {
		t.Fatalf("order InFlight = %d, want %d", orderStatus.InFlight, requestsPerService)
	}
	paymentStatus, err := trafficRecorder.StatusFor("payment-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for payment-service: %v", err)
	}
	if paymentStatus.InFlight != requestsPerService {
		t.Fatalf("payment InFlight = %d, want %d", paymentStatus.InFlight, requestsPerService)
	}

	orderUpstream.release()
	paymentUpstream.release()
	assertAllResultsOK(t, orderResults, "order")
	assertAllResultsOK(t, paymentResults, "payment")

	orderStatus, err = trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for order-service after completion: %v", err)
	}
	paymentStatus, err = trafficRecorder.StatusFor("payment-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for payment-service after completion: %v", err)
	}
	if orderStatus.FinishedRequests != requestsPerService {
		t.Fatalf("order FinishedRequests = %d, want %d", orderStatus.FinishedRequests, requestsPerService)
	}
	if paymentStatus.FinishedRequests != requestsPerService {
		t.Fatalf("payment FinishedRequests = %d, want %d", paymentStatus.FinishedRequests, requestsPerService)
	}
}

type gateUpstream struct {
	server      *httptest.Server
	releaseCh   chan struct{}
	startedCh   chan struct{}
	releaseOnce sync.Once
	started     atomic.Int64
	wantStarted int64
}

func newGateUpstream(t *testing.T, requestCount int64) *gateUpstream {
	t.Helper()

	upstream := &gateUpstream{
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

func (u *gateUpstream) URL() string {
	return u.server.URL
}

func (u *gateUpstream) waitStarted(t *testing.T) {
	t.Helper()

	waitForSignal(t, u.startedCh, "gate upstream requests")
}

func (u *gateUpstream) release() {
	u.releaseOnce.Do(func() {
		close(u.releaseCh)
	})
}

func assertAllResultsOK(t *testing.T, results []<-chan receiveResult, name string) {
	t.Helper()

	for index, resultCh := range results {
		result := waitForResult(t, resultCh, fmt.Sprintf("%s gateway request %d", name, index))
		if result.err != nil {
			t.Fatalf("%s Receive returned error for request %d: %v", name, index, result.err)
		}
		if result.statusCode != http.StatusOK {
			t.Fatalf("%s status for request %d = %d, want %d", name, index, result.statusCode, http.StatusOK)
		}
	}
}
