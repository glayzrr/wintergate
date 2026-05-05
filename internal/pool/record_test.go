package pool

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRecorderStartStoresInFlightRequest(t *testing.T) {
	now := time.Unix(100, 0)
	recorder := newRecorder(func() time.Time { return now }, time.Minute)

	done := recorder.Start(" order-service ")

	snapshot, err := recorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if snapshot.Service != "order-service" {
		t.Fatalf("snapshot.Service = %q, want %q", snapshot.Service, "order-service")
	}
	if snapshot.InFlight != 1 {
		t.Fatalf("snapshot.InFlight = %d, want %d", snapshot.InFlight, 1)
	}
	if snapshot.StartedRequests != 1 {
		t.Fatalf("snapshot.StartedRequests = %d, want %d", snapshot.StartedRequests, 1)
	}
	if snapshot.FinishedRequests != 0 {
		t.Fatalf("snapshot.FinishedRequests = %d, want %d", snapshot.FinishedRequests, 0)
	}
	if snapshot.RequestsInWindow != 1 {
		t.Fatalf("snapshot.RequestsInWindow = %d, want %d", snapshot.RequestsInWindow, 1)
	}

	done()
}

func TestRecorderDoneFinishesRequestOnce(t *testing.T) {
	now := time.Unix(100, 0)
	recorder := newRecorder(func() time.Time { return now }, time.Minute)

	done := recorder.Start("order-service")
	now = now.Add(150 * time.Millisecond)
	done()
	done()

	snapshot, err := recorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if snapshot.InFlight != 0 {
		t.Fatalf("snapshot.InFlight = %d, want %d", snapshot.InFlight, 0)
	}
	if snapshot.FinishedRequests != 1 {
		t.Fatalf("snapshot.FinishedRequests = %d, want %d", snapshot.FinishedRequests, 1)
	}
	if snapshot.AverageLatency != 150*time.Millisecond {
		t.Fatalf("snapshot.AverageLatency = %s, want %s", snapshot.AverageLatency, 150*time.Millisecond)
	}
}

func TestRecorderTracksServicesIndependently(t *testing.T) {
	now := time.Unix(100, 0)
	recorder := newRecorder(func() time.Time { return now }, time.Minute)

	orderDone := recorder.Start("order-service")
	_ = recorder.Start("payment-service")
	orderDone()

	orderSnapshot, err := recorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("Status returned error for order-service: %v", err)
	}
	if orderSnapshot.InFlight != 0 {
		t.Fatalf("orderSnapshot.InFlight = %d, want %d", orderSnapshot.InFlight, 0)
	}

	paymentSnapshot, err := recorder.StatusFor("payment-service")
	if err != nil {
		t.Fatalf("Status returned error for payment-service: %v", err)
	}
	if paymentSnapshot.InFlight != 1 {
		t.Fatalf("paymentSnapshot.InFlight = %d, want %d", paymentSnapshot.InFlight, 1)
	}
}

func TestRecorderCalculatesRPSFromWindow(t *testing.T) {
	now := time.Unix(100, 0)
	recorder := newRecorder(func() time.Time { return now }, 10*time.Second)

	for range 5 {
		recorder.Start("order-service")
	}

	snapshot, err := recorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if snapshot.RequestsInWindow != 5 {
		t.Fatalf("snapshot.RequestsInWindow = %d, want %d", snapshot.RequestsInWindow, 5)
	}
	if snapshot.RPS != 0.5 {
		t.Fatalf("snapshot.RPS = %f, want %f", snapshot.RPS, 0.5)
	}

	now = now.Add(11 * time.Second)
	snapshot, err = recorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if snapshot.RequestsInWindow != 0 {
		t.Fatalf("snapshot.RequestsInWindow = %d, want %d", snapshot.RequestsInWindow, 0)
	}
	if snapshot.RPS != 0 {
		t.Fatalf("snapshot.RPS = %f, want %f", snapshot.RPS, 0.0)
	}
}

func TestRecorderStartIgnoresBlankService(t *testing.T) {
	recorder := NewRecorder()

	done := recorder.Start(" ")
	done()

	_, err := recorder.StatusFor(" ")
	if err == nil {
		t.Fatal("Status returned nil error")
	}
	if !errors.Is(err, ErrInvalidService) {
		t.Fatalf("error = %v, want ErrInvalidService", err)
	}
}

func TestRecorderStatusReturnsErrorWhenServiceMissing(t *testing.T) {
	recorder := NewRecorder()

	_, err := recorder.StatusFor("missing-service")
	if err == nil {
		t.Fatal("Status returned nil error")
	}
	if !errors.Is(err, ErrStatusNotFound) {
		t.Fatalf("error = %v, want ErrStatusNotFound", err)
	}
}

func TestRecorderIsSafeForConcurrentRequests(t *testing.T) {
	recorder := NewRecorder()

	var waitGroup sync.WaitGroup
	for range 100 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			done := recorder.Start("order-service")
			done()
		}()
	}
	waitGroup.Wait()

	snapshot, err := recorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if snapshot.InFlight != 0 {
		t.Fatalf("snapshot.InFlight = %d, want %d", snapshot.InFlight, 0)
	}
	if snapshot.StartedRequests != 100 {
		t.Fatalf("snapshot.StartedRequests = %d, want %d", snapshot.StartedRequests, 100)
	}
	if snapshot.FinishedRequests != 100 {
		t.Fatalf("snapshot.FinishedRequests = %d, want %d", snapshot.FinishedRequests, 100)
	}
}
