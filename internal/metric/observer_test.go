package metric

import (
	"errors"
	"testing"
)

func TestBuildRequestObserverReturnsErrorWhenRecorderNil(t *testing.T) {
	_, err := BuildRequestObserver(nil)
	if err == nil {
		t.Fatal("BuildRequestObserver returned nil error")
	}

	if !errors.Is(err, ErrNilRecorder) {
		t.Fatalf("error = %v, want ErrNilRecorder", err)
	}
}
