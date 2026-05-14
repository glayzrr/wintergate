package pool_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internalconfig "wintergate/internal/config"
	internalpool "wintergate/internal/pool"
	"wintergate/test/harness"
)

func TestRegisteredPoolRuntimeForwardsRequestsAndChangesAssignment(t *testing.T) {
	upstream := newBlockingPairUpstream(t)

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
			internalconfig.ThresholdPoint{InFlight: 2},
			internalconfig.ThresholdPoint{InFlight: 100},
		),
	))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	firstResult := receiveAsync(t, orchestrator, http.MethodGet, "/orders")
	waitForSignal(t, upstream.firstStarted, "first upstream request")

	firstStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for first request: %v", err)
	}
	firstAssignment := runtime.PoolStore.AssignmentFor(firstStatus)
	if firstStatus.InFlight != 1 {
		t.Fatalf("first status InFlight = %d, want 1", firstStatus.InFlight)
	}
	if firstAssignment.Dedicated {
		t.Fatal("first assignment is dedicated, want shared below threshold")
	}

	secondResult := receiveAsync(t, orchestrator, http.MethodGet, "/orders")
	waitForSignal(t, upstream.secondStarted, "second upstream request")

	secondStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for second request: %v", err)
	}
	secondAssignment := runtime.PoolStore.AssignmentFor(secondStatus)
	if secondStatus.InFlight != 2 {
		t.Fatalf("second status InFlight = %d, want 2", secondStatus.InFlight)
	}
	if !secondAssignment.Dedicated {
		t.Fatal("second assignment is shared, want dedicated at threshold")
	}
	if secondAssignment.Tier != internalpool.TierHot {
		t.Fatalf("second assignment tier = %q, want %q", secondAssignment.Tier, internalpool.TierHot)
	}

	upstream.releaseSecond()
	second := waitForResult(t, secondResult, "second gateway request")
	if second.err != nil {
		t.Fatalf("second Receive returned error: %v", second.err)
	}
	if second.statusCode != http.StatusAccepted {
		t.Fatalf("second status = %d, want %d", second.statusCode, http.StatusAccepted)
	}
	if second.body != "second response" {
		t.Fatalf("second body = %q, want %q", second.body, "second response")
	}

	upstream.releaseFirst()
	first := waitForResult(t, firstResult, "first gateway request")
	if first.err != nil {
		t.Fatalf("first Receive returned error: %v", first.err)
	}
	if first.statusCode != http.StatusOK {
		t.Fatalf("first status = %d, want %d", first.statusCode, http.StatusOK)
	}
	if first.body != "first response" {
		t.Fatalf("first body = %q, want %q", first.body, "first response")
	}

	afterDoneStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error after completion: %v", err)
	}
	afterDoneAssignment := runtime.PoolStore.AssignmentFor(afterDoneStatus)
	if afterDoneStatus.InFlight != 0 {
		t.Fatalf("after done InFlight = %d, want 0", afterDoneStatus.InFlight)
	}
	if afterDoneAssignment.Dedicated {
		t.Fatal("assignment is dedicated after in-flight dropped below threshold, want shared")
	}
}

func TestRegisteredPoolRuntimeForwardsRequestThroughConfiguredInstance(t *testing.T) {
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
		if _, err := fmt.Fprint(w, "created"); err != nil {
			t.Errorf("Write returned error: %v", err)
		}
	}))
	defer upstream.Close()

	runtime := harness.NewRuntime()
	runtime.Register(t, harness.ServiceSettings(
		"order-service",
		harness.InstanceFromURL(t, upstream.URL),
		[]internalconfig.EndpointSettings{
			{
				Path:   "/orders",
				Method: http.MethodPost,
			},
		},
	))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)
	request := httptest.NewRequest(http.MethodPost, "/orders?page=1", strings.NewReader(`{"id":1}`))
	request.Header.Set("X-Request-ID", "request-1")

	result := receiveRequest(t, orchestrator, request)
	if result.err != nil {
		t.Fatalf("Receive returned error: %v", result.err)
	}
	if result.statusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", result.statusCode, http.StatusCreated)
	}
	if result.body != "created" {
		t.Fatalf("body = %q, want %q", result.body, "created")
	}
	if result.header.Get("X-Upstream") != "order" {
		t.Fatalf("X-Upstream = %q, want %q", result.header.Get("X-Upstream"), "order")
	}

	status, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error: %v", err)
	}
	if status.InFlight != 0 {
		t.Fatalf("status.InFlight = %d, want 0", status.InFlight)
	}
	if status.StartedRequests != 1 {
		t.Fatalf("status.StartedRequests = %d, want 1", status.StartedRequests)
	}
	if status.FinishedRequests != 1 {
		t.Fatalf("status.FinishedRequests = %d, want 1", status.FinishedRequests)
	}
}
