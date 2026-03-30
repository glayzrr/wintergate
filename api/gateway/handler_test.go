package gatewayapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	configapi "wintergate/api/config"
	responseapi "wintergate/api/response"

	"github.com/gin-gonic/gin"
)

func TestHandlerReceiveReturnsGatewayIngressResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler()

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"hello":"world"}`))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, true)
	}

	if response.Message != responseReceiveSuccess {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseReceiveSuccess)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("response.Data type = %T, want map[string]any", response.Data)
	}

	received, ok := data["received"].(bool)
	if !ok || !received {
		t.Fatalf("data[received] = %#v, want true", data["received"])
	}

	method, ok := data["method"].(string)
	if !ok || method != http.MethodPost {
		t.Fatalf("data[method] = %#v, want %q", data["method"], http.MethodPost)
	}

	path, ok := data["path"].(string)
	if !ok || path != "/orders" {
		t.Fatalf("data[path] = %#v, want %q", data["path"], "/orders")
	}
}

func TestHandlerReceiveLeavesConfigRouteUnclaimed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler()

	router := gin.New()
	handler.RegisterRoutes(router)

	request := httptest.NewRequest(http.MethodGet, configapi.DefaultRoute, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("response.Success = %v, want %v", response.Success, false)
	}

	if response.Message != responseNotFound {
		t.Fatalf("response.Message = %q, want %q", response.Message, responseNotFound)
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
