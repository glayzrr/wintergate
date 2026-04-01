package gateway

import (
	"context"
	"errors"
	"testing"
)

func TestNewOrchestratorReturnsErrorWhenTaskNil(t *testing.T) {
	orchestrator := NewOrchestrator(nil)

	_, err := orchestrator.Receive(context.Background(), Request{
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

	result, err := orchestrator.Receive(context.Background(), Request{
		Method: " POST ",
		Path:   " /orders ",
	})
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}

	if !result.Received {
		t.Fatal("result.Received = false, want true")
	}

	if result.Method != "POST" {
		t.Fatalf("result.Method = %q, want %q", result.Method, "POST")
	}

	if result.Path != "/orders" {
		t.Fatalf("result.Path = %q, want %q", result.Path, "/orders")
	}
}

func TestReceiveRunsTasksInOrder(t *testing.T) {
	firstRan := false
	secondRan := false

	orchestrator := NewOrchestrator(
		TaskFunc(func(_ context.Context, state *State) error {
			firstRan = true
			state.Result.Method = "PATCH"
			return nil
		}),
		TaskFunc(func(_ context.Context, state *State) error {
			if !firstRan {
				t.Fatal("first task did not run before second task")
			}

			secondRan = true
			state.Result.Path = state.Result.Path + "/forwarded"
			return nil
		}),
	)

	result, err := orchestrator.Receive(context.Background(), Request{
		Method: "POST",
		Path:   "/orders",
	})
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}

	if !secondRan {
		t.Fatal("second task did not run")
	}

	if result.Method != "PATCH" {
		t.Fatalf("result.Method = %q, want %q", result.Method, "PATCH")
	}

	if result.Path != "/orders/forwarded" {
		t.Fatalf("result.Path = %q, want %q", result.Path, "/orders/forwarded")
	}
}

func TestReceiveReturnsErrorWhenMethodMissing(t *testing.T) {
	orchestrator := NewOrchestrator()

	_, err := orchestrator.Receive(context.Background(), Request{
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

	_, err := orchestrator.Receive(context.Background(), Request{
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

	_, err := orchestrator.Receive(context.Background(), Request{
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

func (fn TaskFunc) Run(ctx context.Context, state *State) error {
	return fn(ctx, state)
}
