package pool_test

import (
	"net/http"
	"testing"

	internalconfig "wintergate/internal/config"
	internalpool "wintergate/internal/pool"
	"wintergate/test/harness"
)

func TestRegisteredThresholdsMoveClientBetweenSharedAndDedicatedPools(t *testing.T) {
	runtime := harness.NewRuntime()
	runtime.Register(t, harness.ServiceSettings(
		"order-service",
		internalconfig.InstanceSettings{
			Scheme: "http",
			Host:   "127.0.0.1",
			Port:   "8080",
		},
		[]internalconfig.EndpointSettings{
			{
				Path:   "/orders",
				Method: http.MethodGet,
			},
		},
		harness.WithPoolThresholds(
			internalconfig.ThresholdPoint{},
			internalconfig.ThresholdPoint{InFlight: 2},
			internalconfig.ThresholdPoint{InFlight: 100},
		),
	))

	coordinator := internalpool.NewCoordinator()
	trafficRecorder := internalpool.NewRecorder()

	firstDone := trafficRecorder.Start("order-service")
	firstStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for first request: %v", err)
	}
	firstAssignment := runtime.PoolStore.AssignmentFor(firstStatus)
	if firstAssignment.Dedicated {
		t.Fatal("first assignment is dedicated, want shared below threshold")
	}

	firstLease, err := coordinator.Acquire(firstAssignment)
	if err != nil {
		t.Fatalf("Acquire returned error for first assignment: %v", err)
	}
	firstLease.Finish()

	secondDone := trafficRecorder.Start("order-service")
	secondStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error for second request: %v", err)
	}
	secondAssignment := runtime.PoolStore.AssignmentFor(secondStatus)
	if !secondAssignment.Dedicated {
		t.Fatal("second assignment is shared, want dedicated at threshold")
	}
	if secondAssignment.Tier != internalpool.TierHot {
		t.Fatalf("second assignment tier = %q, want %q", secondAssignment.Tier, internalpool.TierHot)
	}

	secondLease, err := coordinator.Acquire(secondAssignment)
	if err != nil {
		t.Fatalf("Acquire returned error for second assignment: %v", err)
	}
	if secondLease.Client == firstLease.Client {
		t.Fatal("dedicated assignment reused shared client")
	}
	secondLease.Finish()

	secondDone()
	firstDone()
	afterDoneStatus, err := trafficRecorder.StatusFor("order-service")
	if err != nil {
		t.Fatalf("StatusFor returned error after completion: %v", err)
	}
	afterDoneAssignment := runtime.PoolStore.AssignmentFor(afterDoneStatus)
	if afterDoneAssignment.Dedicated {
		t.Fatal("assignment is dedicated after in-flight dropped below threshold, want shared")
	}

	afterDoneLease, err := coordinator.Acquire(afterDoneAssignment)
	if err != nil {
		t.Fatalf("Acquire returned error after completion: %v", err)
	}
	if afterDoneLease.Client != firstLease.Client {
		t.Fatal("shared assignment did not return to the original shared client")
	}
	afterDoneLease.Finish()
}
