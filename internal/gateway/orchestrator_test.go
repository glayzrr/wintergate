package gateway

import (
	"context"
	"errors"
	"testing"
)

func TestNewOrchestratorReturnsErrorWhenTaskNil(t *testing.T) {
	orchestrator := NewOrchestrator(nil)

	err := orchestrator.Receive(context.Background(), Request{
		Method: "POST",
		Path:   "/orders",
	})
	if err == nil {
		t.Fatal("Receive returned nil error")
	}

	if !errors.Is(err, ErrNilTask) {
		t.Fatalf("error = %v, want ErrNilTask", err)
	}
}

func TestReceiveReturnsRequestMetadataWhenNoTaskRegistered(t *testing.T) {
	orchestrator := NewOrchestrator()

	err := orchestrator.Receive(context.Background(), Request{
		Method: " POST ",
		Path:   " /orders ",
	})
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
}

func TestReceiveRunsTasksInOrder(t *testing.T) {
	firstRan := false
	secondRan := false

	orchestrator := NewOrchestrator(
		TaskFunc(func(_ context.Context, state *State) error {
			firstRan = true
			state.Request.Method = "PATCH"
			return nil
		}),
		TaskFunc(func(_ context.Context, state *State) error {
			if !firstRan {
				t.Fatal("first task did not run before second task")
			}

			secondRan = true
			state.Request.Path = state.Request.Path + "/forwarded"
			return nil
		}),
	)

	err := orchestrator.Receive(context.Background(), Request{
		Method: "POST",
		Path:   "/orders",
	})
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}

	if !secondRan {
		t.Fatal("second task did not run")
	}
}

func TestReceivePreservesAuthorizationHeader(t *testing.T) {
	orchestrator := NewOrchestrator(
		TaskFunc(func(_ context.Context, state *State) error {
			if state.Request.Service != "order-service" {
				t.Fatalf("state.Request.Service = %q, want %q", state.Request.Service, "order-service")
			}

			if state.Request.AuthorizationHeader != "Bearer token-value" {
				t.Fatalf("state.Request.AuthorizationHeader = %q, want %q", state.Request.AuthorizationHeader, "Bearer token-value")
			}

			return nil
		}),
	)

	err := orchestrator.Receive(context.Background(), Request{
		Service:             " order-service ",
		Method:              "POST",
		Path:                "/orders",
		AuthorizationHeader: "Bearer token-value",
	})
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
}

func TestReceiveReturnsErrorWhenMethodMissing(t *testing.T) {
	orchestrator := NewOrchestrator()

	err := orchestrator.Receive(context.Background(), Request{
		Path: "/orders",
	})
	if err == nil {
		t.Fatal("Receive returned nil error")
	}

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestReceiveReturnsErrorWhenPathMissing(t *testing.T) {
	orchestrator := NewOrchestrator()

	err := orchestrator.Receive(context.Background(), Request{
		Method: "POST",
	})
	if err == nil {
		t.Fatal("Receive returned nil error")
	}

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestReceiveReturnsWrappedTaskError(t *testing.T) {
	taskErr := errors.New("task failed")

	orchestrator := NewOrchestrator(
		TaskFunc(func(_ context.Context, _ *State) error {
			return taskErr
		}),
	)

	err := orchestrator.Receive(context.Background(), Request{
		Method: "POST",
		Path:   "/orders",
	})
	if err == nil {
		t.Fatal("Receive returned nil error")
	}

	if !errors.Is(err, taskErr) {
		t.Fatalf("error = %v, want wrapped task error", err)
	}
}

type TaskFunc func(ctx context.Context, state *State) error

// Run 테스트용 gateway task 함수를 실행합니다.
func (fn TaskFunc) Run(ctx context.Context, state *State) error {
	// 테스트에서 주입한 함수로 Task 실행 흐름을 대체합니다.
	return fn(ctx, state)
}
