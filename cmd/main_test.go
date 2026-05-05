package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	responseapi "wintergate/api/response"

	"github.com/gin-gonic/gin"
)

func TestMainLogsAndExitsWhenRunFails(t *testing.T) {
	if os.Getenv("WG_TEST_RUN_MAIN") == "1" {
		main()
		t.Fatal("main returned unexpectedly")
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainLogsAndExitsWhenRunFails$")
	cmd.Env = append(os.Environ(), "WG_TEST_RUN_MAIN=1", "PORT=65536")
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	if err == nil {
		t.Fatal("main subprocess succeeded unexpectedly")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run returned error type %T, want *exec.ExitError", err)
	}

	if exitErr.ExitCode() != 1 {
		t.Fatalf("exitCode = %d, want %d", exitErr.ExitCode(), 1)
	}

	if !strings.Contains(output.String(), logRunFailed) {
		t.Fatalf("output = %q, want log message %q", output.String(), logRunFailed)
	}
}

func TestRunReturnsErrorWhenServerFails(t *testing.T) {
	t.Setenv("PORT", "65536")

	err := run()
	if err == nil {
		t.Fatal("run returned nil error")
	}

	if !strings.Contains(err.Error(), "run gin server") {
		t.Fatalf("error = %q, want run gin server context", err.Error())
	}
}

func TestNewRouterRegistersConfigRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{`))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}
}

func TestNewRouterRegistersMetricsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/metric", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), "go_goroutines") {
		t.Fatal("metric response does not include go_goroutines")
	}
}

func TestNewRouterRecordsHTTPMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter returned error: %v", err)
	}

	configRequest := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{`))
	configRequest.Header.Set("Content-Type", "application/json")
	configRecorder := httptest.NewRecorder()
	router.ServeHTTP(configRecorder, configRequest)

	if configRecorder.Code != http.StatusBadRequest {
		t.Fatalf("config status = %d, want %d", configRecorder.Code, http.StatusBadRequest)
	}

	metricRequest := httptest.NewRequest(http.MethodGet, "/metric", nil)
	metricRecorder := httptest.NewRecorder()
	router.ServeHTTP(metricRecorder, metricRequest)

	body := metricRecorder.Body.String()
	for _, metricName := range []string{
		"wintergate_http_requests_total",
		"wintergate_http_request_duration_seconds",
		"wintergate_http_requests_in_flight",
		"wintergate_http_request_failures_total",
	} {
		if !strings.Contains(body, metricName) {
			t.Fatalf("metric response does not include %q", metricName)
		}
	}
}

func TestNewRouterRegistersGatewayIngressRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter returned error: %v", err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders" {
			t.Errorf("upstream path = %q, want %q", r.URL.Path, "/orders")
		}

		w.WriteHeader(http.StatusAccepted)
		if _, err := w.Write([]byte("upstream ok")); err != nil {
			t.Errorf("Write returned error: %v", err)
		}
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	serviceHost, servicePort, err := net.SplitHostPort(upstreamURL.Host)
	if err != nil {
		t.Fatalf("SplitHostPort returned error: %v", err)
	}

	servicePortNumber, err := strconv.Atoi(servicePort)
	if err != nil {
		t.Fatalf("Atoi returned error: %v", err)
	}

	configBody, err := json.Marshal(map[string]any{
		"global": map[string]any{
			"auth": map[string]any{
				"jwt_algorithm":  "HS256",
				"jwt_audience":   "wintergate",
				"jwt_clock_skew": "1m",
				"jwt_issuer":     "auth-service",
				"jwt_secret":     "shared-secret",
			},
		},
		"routes": []map[string]any{
			{
				"name": "order-service",
				"host": serviceHost,
				"port": servicePortNumber,
				"threshold": map[string]any{
					"hot": map[string]any{
						"rps":       100,
						"in-flight": 14,
					},
					"super": map[string]any{
						"rps":       150,
						"in-flight": 50,
					},
				},
				"endpoints": []map[string]any{
					{
						"path":   "/orders",
						"method": http.MethodGet,
						"roles":  []string{},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	configRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/config",
		strings.NewReader(string(configBody)),
	)
	configRequest.Header.Set("Content-Type", "application/json")
	configRequest.RemoteAddr = "192.0.2.10:43123"

	configRecorder := httptest.NewRecorder()
	router.ServeHTTP(configRecorder, configRequest)

	if configRecorder.Code != http.StatusOK {
		t.Fatalf("config status = %d, want %d", configRecorder.Code, http.StatusOK)
	}

	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	request.Header.Set("X-Service-Scheme", upstreamURL.Scheme)
	request.Header.Set("X-Service-Host", serviceHost)
	request.Header.Set("X-Service-Port", servicePort)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}

	if recorder.Body.String() != "upstream ok" {
		t.Fatalf("body = %q, want %q", recorder.Body.String(), "upstream ok")
	}

	metricRequest := httptest.NewRequest(http.MethodGet, "/metric", nil)
	metricRecorder := httptest.NewRecorder()
	router.ServeHTTP(metricRecorder, metricRequest)

	metricBody := metricRecorder.Body.String()
	for _, metricName := range []string{
		"wintergate_pool_selections_total",
		"wintergate_upstream_requests_total",
		"wintergate_upstream_request_duration_seconds",
	} {
		if !strings.Contains(metricBody, metricName) {
			t.Fatalf("metric response does not include %q", metricName)
		}
	}
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) responseapi.APIResponse {
	t.Helper()

	var response responseapi.APIResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	return response
}

func TestListenAddressReturnsPortWhenSet(t *testing.T) {
	t.Setenv("PORT", "9090")

	address := listenAddress()
	if address != ":9090" {
		t.Fatalf("address = %q, want %q", address, ":9090")
	}
}

func TestListenAddressReturnsDefaultWhenPortMissing(t *testing.T) {
	t.Setenv("PORT", "")

	address := listenAddress()
	if address != defaultListenAddress {
		t.Fatalf("address = %q, want %q", address, defaultListenAddress)
	}
}
