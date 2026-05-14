package pool_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	internalconfig "wintergate/internal/config"
	internalpool "wintergate/internal/pool"
	"wintergate/test/harness"
)

func TestDedicatedPoolReplacementWaitsForInFlightRequestBeforeClosingOldConnection(t *testing.T) {
	upstream := newTrackedUpstream(t)

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
			internalconfig.ThresholdPoint{InFlight: 1},
			internalconfig.ThresholdPoint{InFlight: 2},
		),
	))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	firstResult := receiveAsync(t, orchestrator, http.MethodGet, "/orders")
	firstRemoteAddress := waitForString(t, upstream.firstStarted, "first upstream request")

	second := receive(t, orchestrator, http.MethodGet, "/orders")
	if second.err != nil {
		t.Fatalf("second Receive returned error: %v", second.err)
	}
	if second.statusCode != http.StatusAccepted {
		t.Fatalf("second status = %d, want %d", second.statusCode, http.StatusAccepted)
	}
	if second.body != "second response" {
		t.Fatalf("second body = %q, want %q", second.body, "second response")
	}
	assertRemoteAddressStillOpen(t, upstream.closed, firstRemoteAddress, 20*time.Millisecond)

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

	waitForClosedRemoteAddress(t, upstream.closed, firstRemoteAddress)
}

func TestDedicatedPoolReplacementWaitsForContextTimeoutBeforeClosingOldConnection(t *testing.T) {
	upstream := newTimeoutTrackedUpstream(t)

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
			internalconfig.ThresholdPoint{InFlight: 1},
			internalconfig.ThresholdPoint{InFlight: 2},
		),
	))

	trafficRecorder := internalpool.NewRecorder()
	orchestrator := newPoolOrchestrator(runtime, trafficRecorder)

	firstRequest := httptest.NewRequest(http.MethodGet, "/orders", nil)
	firstContext, cancel := context.WithTimeout(firstRequest.Context(), 200*time.Millisecond)
	defer cancel()
	firstRequest = firstRequest.WithContext(firstContext)
	firstResult := receiveRequestAsync(t, orchestrator, firstRequest)
	firstRemoteAddress := waitForString(t, upstream.firstStarted, "first upstream request")

	second := receive(t, orchestrator, http.MethodGet, "/orders")
	if second.err != nil {
		t.Fatalf("second Receive returned error: %v", second.err)
	}
	if second.statusCode != http.StatusAccepted {
		t.Fatalf("second status = %d, want %d", second.statusCode, http.StatusAccepted)
	}
	assertRemoteAddressStillOpen(t, upstream.closed, firstRemoteAddress, 30*time.Millisecond)

	first := waitForResult(t, firstResult, "timed out gateway request")
	if first.err == nil {
		t.Fatal("first Receive returned nil error")
	}
	if !errors.Is(first.err, context.DeadlineExceeded) {
		t.Fatalf("first error = %v, want context deadline exceeded", first.err)
	}

	waitForClosedRemoteAddress(t, upstream.closed, firstRemoteAddress)
}
