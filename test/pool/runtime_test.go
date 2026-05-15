package pool_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	internalgateway "wintergate/internal/gateway"
	internalmetric "wintergate/internal/metric"
	metricrecord "wintergate/internal/metric/record"
	internalpool "wintergate/internal/pool"
	"wintergate/test/harness"
)

type receiveResult struct {
	statusCode int
	header     http.Header
	body       string
	err        error
}

func newPoolOrchestrator(runtime *harness.Runtime, trafficRecorder *internalpool.Recorder) *internalgateway.Orchestrator {
	coordinator := internalpool.NewCoordinator()
	metricRecorder := metricrecord.NewRecorder(internalmetric.NewRegistry())
	forwarder := internalpool.NewForwarder(coordinator, metricRecorder)

	return internalgateway.NewOrchestrator(
		internalgateway.NewRouteTask(runtime.Manager, runtime.Router, runtime.LoadBalancer),
		internalgateway.NewTransferTask(runtime.PoolStore, forwarder, trafficRecorder),
	)
}

func receiveAsync(t *testing.T, orchestrator *internalgateway.Orchestrator, method, path string) <-chan receiveResult {
	t.Helper()

	resultCh := make(chan receiveResult, 1)
	go func() {
		resultCh <- receive(t, orchestrator, method, path)
	}()

	return resultCh
}

func receiveRequestAsync(t *testing.T, orchestrator *internalgateway.Orchestrator, request *http.Request) <-chan receiveResult {
	t.Helper()

	resultCh := make(chan receiveResult, 1)
	go func() {
		resultCh <- receiveRequest(t, orchestrator, request)
	}()

	return resultCh
}

func receive(t *testing.T, orchestrator *internalgateway.Orchestrator, method, path string) receiveResult {
	t.Helper()

	request := httptest.NewRequest(method, path, nil)
	return receiveRequest(t, orchestrator, request)
}

func receiveRequest(t *testing.T, orchestrator *internalgateway.Orchestrator, request *http.Request) receiveResult {
	t.Helper()

	recorder := httptest.NewRecorder()
	err := orchestrator.Receive(request.Context(), internalgateway.Request{
		Method:         request.Method,
		Path:           request.URL.Path,
		ResponseWriter: recorder,
		HTTPRequest:    request,
	})

	return receiveResult{
		statusCode: recorder.Code,
		header:     recorder.Header().Clone(),
		body:       recorder.Body.String(),
		err:        err,
	}
}
