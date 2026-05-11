package gateway

import (
	"context"
	"errors"
	"testing"

	internalauth "wintergate/internal/auth"
)

func TestNewAuthenticateTaskReturnsTaskWhenDecoderNil(t *testing.T) {
	task := NewAuthenticateTask(nil)
	if task == nil {
		t.Fatal("NewAuthenticateTask returned nil task")
	}
}

func TestAuthenticateTaskRunStoresClaims(t *testing.T) {
	task := NewAuthenticateTask(stubTokenDecoder{
		claims: internalauth.Claims{
			Subject: "user-1",
		},
	})

	state := &State{
		Request: Request{
			AuthorizationHeader: "Bearer token-value",
		},
	}

	err := task.Run(context.Background(), state)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Claims == nil {
		t.Fatal("state.Claims is nil")
	}

	if state.Claims.Subject != "user-1" {
		t.Fatalf("state.Claims.Subject = %q, want %q", state.Claims.Subject, "user-1")
	}
}

func TestAuthenticateTaskRunSkipsWhenAuthorizationHeaderMissing(t *testing.T) {
	task := NewAuthenticateTask(stubTokenDecoder{})

	state := &State{}

	err := task.Run(context.Background(), state)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if state.Claims != nil {
		t.Fatal("state.Claims is not nil")
	}
}

func TestAuthenticateTaskRunReturnsWrappedBearerError(t *testing.T) {
	task := NewAuthenticateTask(stubTokenDecoder{})

	state := &State{
		Request: Request{
			AuthorizationHeader: "Basic token-value",
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, internalauth.ErrInvalidAuthorizationHeader) {
		t.Fatalf("error = %v, want ErrInvalidAuthorizationHeader", err)
	}
}

func TestAuthenticateTaskRunReturnsErrorWhenDecoderNil(t *testing.T) {
	task := NewAuthenticateTask(nil)

	state := &State{
		Request: Request{
			AuthorizationHeader: "Bearer token-value",
		},
	}

	err := task.Run(context.Background(), state)
	if err == nil {
		t.Fatal("Run returned nil error")
	}

	if !errors.Is(err, ErrNilTokenDecoder) {
		t.Fatalf("error = %v, want ErrNilTokenDecoder", err)
	}
}

type stubTokenDecoder struct {
	claims internalauth.Claims
	err    error
}

func (d stubTokenDecoder) DecodeFor(_, _ string) (internalauth.Claims, error) {
	if d.err != nil {
		return internalauth.Claims{}, d.err
	}

	return d.claims, nil
}
