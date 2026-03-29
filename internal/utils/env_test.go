package utils

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

var errInvalidConfig = errors.New("invalid config")

func TestRequireString(t *testing.T) {
	resetEnv(t, "TEST_STRING")
	t.Setenv("TEST_STRING", " value ")

	value, err := RequireString("TEST_STRING", errInvalidConfig)
	if err != nil {
		t.Fatalf("RequireString returned error: %v", err)
	}

	if value != "value" {
		t.Fatalf("value = %q, want %q", value, "value")
	}
}

func TestRequireStringReturnsErrorWhenMissing(t *testing.T) {
	resetEnv(t, "TEST_STRING")

	_, err := RequireString("TEST_STRING", errInvalidConfig)
	if err == nil {
		t.Fatal("RequireString returned nil error")
	}

	if !errors.Is(err, errInvalidConfig) {
		t.Fatalf("error = %v, want errInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), "TEST_STRING") {
		t.Fatalf("error = %q, want missing key %q in message", err.Error(), "TEST_STRING")
	}
}

func TestRequireDuration(t *testing.T) {
	resetEnv(t, "TEST_DURATION")
	t.Setenv("TEST_DURATION", "5s")

	duration, err := RequireDuration("TEST_DURATION", errInvalidConfig)
	if err != nil {
		t.Fatalf("RequireDuration returned error: %v", err)
	}

	if duration != 5*time.Second {
		t.Fatalf("duration = %s, want %s", duration, 5*time.Second)
	}
}

func TestRequireDurationReturnsErrorWhenMissing(t *testing.T) {
	resetEnv(t, "TEST_DURATION")

	_, err := RequireDuration("TEST_DURATION", errInvalidConfig)
	if err == nil {
		t.Fatal("RequireDuration returned nil error")
	}

	if !errors.Is(err, errInvalidConfig) {
		t.Fatalf("error = %v, want errInvalidConfig", err)
	}
}

func TestRequireDurationReturnsErrorWhenInvalid(t *testing.T) {
	resetEnv(t, "TEST_DURATION")
	t.Setenv("TEST_DURATION", "not-a-duration")

	_, err := RequireDuration("TEST_DURATION", errInvalidConfig)
	if err == nil {
		t.Fatal("RequireDuration returned nil error")
	}

	if !errors.Is(err, errInvalidConfig) {
		t.Fatalf("error = %v, want errInvalidConfig", err)
	}

	if !strings.Contains(err.Error(), "TEST_DURATION") {
		t.Fatalf("error = %q, want invalid key %q in message", err.Error(), "TEST_DURATION")
	}
}

func resetEnv(t *testing.T, key string) {
	t.Helper()

	value, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset env %s: %v", key, err)
	}

	t.Cleanup(func() {
		if !ok {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("cleanup unset env %s: %v", key, err)
			}

			return
		}

		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("cleanup restore env %s: %v", key, err)
		}
	})
}
