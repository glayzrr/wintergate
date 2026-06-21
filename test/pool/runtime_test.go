package pool_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	internalgateway "wintergate/internal/gateway"
	internalmetric "wintergate/internal/metric"
	metricrecord "wintergate/internal/metric/record"
	internalpool "wintergate/internal/pool"
	poolconfig "wintergate/internal/pool/config"
	"wintergate/internal/pool/traffic"
	"wintergate/test/harness"
)

type receiveResult struct {
	statusCode int
	header     http.Header
	body       string
	err        error
}

func newPoolOrchestrator(t *testing.T, runtime *harness.Runtime, trafficRecorder *traffic.Recorder) *internalgateway.Orchestrator {
	t.Helper()
	configurePoolRuntime(t)

	coordinator := internalpool.NewCoordinator()
	metricRecorder := metricrecord.NewRecorder(internalmetric.NewRegistry())
	forwarder := internalpool.NewForwarder(coordinator, metricRecorder)

	return internalgateway.NewOrchestrator(
		internalgateway.NewRouteTask(runtime.Manager, runtime.Router, runtime.LoadBalancer),
		internalgateway.NewTransferTask(runtime.PoolStore, forwarder, trafficRecorder),
	)
}

func configurePoolRuntime(t *testing.T) {
	t.Helper()

	err := poolconfig.Configure(poolconfig.Config{
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 256,
		MaxConnsPerHost:     512,
		IdleConnTimeout:     45 * time.Second,
	}, map[poolconfig.Tier]poolconfig.Config{
		poolconfig.TierNormal: {
			MaxIdleConns:        1024,
			MaxIdleConnsPerHost: 512,
			MaxConnsPerHost:     1024,
			IdleConnTimeout:     90 * time.Second,
		},
		poolconfig.TierHot: {
			MaxIdleConns:        2048,
			MaxIdleConnsPerHost: 1024,
			MaxConnsPerHost:     2048,
			IdleConnTimeout:     180 * time.Second,
		},
		poolconfig.TierSuper: {
			MaxIdleConns:        4096,
			MaxIdleConnsPerHost: 2048,
			MaxConnsPerHost:     4096,
			IdleConnTimeout:     360 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
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
