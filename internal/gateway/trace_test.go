package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wintergate/internal/trace"
)

func TestTraceTaskRunUsesExistingRequestID(t *testing.T) {
	task := NewTraceTask(trace.NewGenerator())
	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	request.Header.Set(trace.RequestIDHeader, " request-1 ")
	recorder := httptest.NewRecorder()
	state := &State{
		Request: Request{
			ConfigKey:      "order-service",
			HTTPRequest:    request,
			ResponseWriter: recorder,
		},
	}

	if err := task.Run(context.Background(), state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Request.ID != "request-1" {
		t.Fatalf("state.Request.ID = %q, want %q", state.Request.ID, "request-1")
	}
	if request.Header.Get(trace.RequestIDHeader) != "request-1" {
		t.Fatalf("request header = %q, want %q", request.Header.Get(trace.RequestIDHeader), "request-1")
	}
	if recorder.Header().Get(trace.RequestIDHeader) != "request-1" {
		t.Fatalf("response header = %q, want %q", recorder.Header().Get(trace.RequestIDHeader), "request-1")
	}
}

func TestTraceTaskRunGeneratesRequestIDWhenMissing(t *testing.T) {
	task := NewTraceTask(trace.NewGenerator())
	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	state := &State{
		Request: Request{
			ConfigKey:   "order-service",
			HTTPRequest: request,
		},
	}

	if err := task.Run(context.Background(), state); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Request.ID == "" {
		t.Fatal("state.Request.ID is empty")
	}
	if !strings.HasPrefix(state.Request.ID, "order-service-") {
		t.Fatalf("state.Request.ID = %q, want order-service prefix", state.Request.ID)
	}
	if request.Header.Get(trace.RequestIDHeader) != state.Request.ID {
		t.Fatalf("request header = %q, want %q", request.Header.Get(trace.RequestIDHeader), state.Request.ID)
	}
}

func TestTraceTaskRunReturnsErrorWhenGeneratorNil(t *testing.T) {
	task := NewTraceTask(nil)

	err := task.Run(context.Background(), &State{})
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if !errors.Is(err, trace.ErrNilGenerator) {
		t.Fatalf("error = %v, want ErrNilGenerator", err)
	}
}
