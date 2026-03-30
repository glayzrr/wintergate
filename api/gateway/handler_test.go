package gatewayapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	configapi "wintergate/api/config"

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

	body := recorder.Body.String()
	if !strings.Contains(body, `"received":true`) {
		t.Fatalf("body = %q, want received flag in response", body)
	}

	if !strings.Contains(body, `"method":"POST"`) {
		t.Fatalf("body = %q, want method in response", body)
	}

	if !strings.Contains(body, `"path":"/orders"`) {
		t.Fatalf("body = %q, want path in response", body)
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
}
