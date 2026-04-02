package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func TestNewRouterRegistersGatewayIngressRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, true)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("response.Data type = %T, want map[string]any", response.Data)
	}

	received, ok := data["received"].(bool)
	if !ok || !received {
		t.Fatalf("data[received] = %#v, want true", data["received"])
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
