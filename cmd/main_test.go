package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	responseapi "wintergate/api/response"

	"github.com/gin-gonic/gin"
)

func TestMainReturnsWhenRunSucceeds(t *testing.T) {
	restoreRunMain := runMain
	restoreLogError := logError
	restoreExitProcess := exitProcess
	t.Cleanup(func() {
		runMain = restoreRunMain
		logError = restoreLogError
		exitProcess = restoreExitProcess
	})

	logged := false
	exited := false

	runMain = func() error {
		return nil
	}
	logError = func(string, ...any) {
		logged = true
	}
	exitProcess = func(int) {
		exited = true
	}

	main()

	if logged {
		t.Fatal("main logged error on success")
	}

	if exited {
		t.Fatal("main exited on success")
	}
}

func TestMainLogsAndExitsWhenRunFails(t *testing.T) {
	restoreRunMain := runMain
	restoreLogError := logError
	restoreExitProcess := exitProcess
	t.Cleanup(func() {
		runMain = restoreRunMain
		logError = restoreLogError
		exitProcess = restoreExitProcess
	})

	runErr := errors.New("boom")
	logged := false
	exitCode := 0

	runMain = func() error {
		return runErr
	}
	logError = func(msg string, args ...any) {
		logged = msg == logRunFailed
	}
	exitProcess = func(code int) {
		exitCode = code
	}

	main()

	if !logged {
		t.Fatal("main did not log error")
	}

	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want %d", exitCode, 1)
	}
}

func TestRunReturnsErrorWhenBuildRouterFails(t *testing.T) {
	restoreBuildRouter := buildRouter
	t.Cleanup(func() {
		buildRouter = restoreBuildRouter
	})

	buildErr := errors.New("router error")
	buildRouter = func() (*gin.Engine, error) {
		return nil, buildErr
	}

	err := run()
	if err == nil {
		t.Fatal("run returned nil error")
	}

	if !strings.Contains(err.Error(), "build router") {
		t.Fatalf("error = %q, want build router context", err.Error())
	}
}

func TestRunReturnsErrorWhenServerFails(t *testing.T) {
	restoreBuildRouter := buildRouter
	restoreRunServer := runServer
	t.Cleanup(func() {
		buildRouter = restoreBuildRouter
		runServer = restoreRunServer
	})

	buildRouter = newRouter
	runServer = func(*gin.Engine, string) error {
		return errors.New("server error")
	}

	err := run()
	if err == nil {
		t.Fatal("run returned nil error")
	}

	if !strings.Contains(err.Error(), "run gin server") {
		t.Fatalf("error = %q, want run gin server context", err.Error())
	}
}

func TestRunReturnsNilWhenServerStarts(t *testing.T) {
	restoreBuildRouter := buildRouter
	restoreRunServer := runServer
	t.Cleanup(func() {
		buildRouter = restoreBuildRouter
		runServer = restoreRunServer
	})

	called := false
	buildRouter = newRouter
	runServer = func(_ *gin.Engine, addr string) error {
		called = addr == defaultListenAddress
		return nil
	}

	t.Setenv("PORT", "")

	err := run()
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if !called {
		t.Fatal("runServer was not called with default listen address")
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
