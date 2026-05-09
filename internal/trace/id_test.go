package trace

import (
	"errors"
	"testing"

	"wintergate/internal/utils"

	"github.com/google/uuid"
)

func TestGeneratorGenerateIncludesService(t *testing.T) {
	generator := &Generator{
		newID: func() (uuid.UUID, error) {
			return uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"), nil
		},
	}

	requestID, err := generator.Generate(" order-service ")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	want := "order-service-123e4567-e89b-12d3-a456-426614174000"
	if requestID != want {
		t.Fatalf("requestID = %q, want %q", requestID, want)
	}
}

func TestGeneratorGenerateFallsBackToUUIDWhenServiceInvalid(t *testing.T) {
	generator := &Generator{
		newID: func() (uuid.UUID, error) {
			return uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"), nil
		},
	}

	requestID, err := generator.Generate("order-service\nbad")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	want := "123e4567-e89b-12d3-a456-426614174000"
	if requestID != want {
		t.Fatalf("requestID = %q, want %q", requestID, want)
	}
}

func TestGeneratorGenerateReturnsErrorWhenUUIDFails(t *testing.T) {
	generateErr := errors.New("uuid failed")
	generator := &Generator{
		newID: func() (uuid.UUID, error) {
			return uuid.Nil, generateErr
		},
	}

	_, err := generator.Generate("order-service")
	if err == nil {
		t.Fatal("Generate returned nil error")
	}
	if !errors.Is(err, generateErr) {
		t.Fatalf("error = %v, want %v", err, generateErr)
	}
}

func TestNormalizeRequestID(t *testing.T) {
	requestID, ok := utils.NormalizeRequestID(" request-1 ", MaxRequestIDLength)
	if !ok {
		t.Fatal("NormalizeRequestID returned false")
	}
	if requestID != "request-1" {
		t.Fatalf("requestID = %q, want %q", requestID, "request-1")
	}

	if _, ok := utils.NormalizeRequestID("request\n1", MaxRequestIDLength); ok {
		t.Fatal("NormalizeRequestID returned true for newline")
	}
}
