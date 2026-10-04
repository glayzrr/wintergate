package policy

import (
	"sync"
	"testing"
	"time"

	internalconfig "wintergate/internal/config"
	poolconfig "wintergate/internal/pool/config"
	"wintergate/internal/pool/traffic"
)

func TestAssignmentDelaysSharedReturnAndResetsWhenEitherThresholdIsReached(t *testing.T) {
	for _, metric := range []string{"rps", "in-flight"} {
		t.Run(metric, func(t *testing.T) {
			now := time.Unix(100, 0)
			store := NewStore()
			store.clock = func() time.Time { return now }
			snapshot := assignmentSnapshot("orders", &internalconfig.ThresholdSettings{
				Hot: internalconfig.ThresholdPoint{RPS: 100, InFlight: 14},
			})
			high := traffic.Status{ConfigKey: "orders"}
			if metric == "rps" {
				high.RPS = 100
			} else {
				high.InFlight = 14
			}
			low := traffic.Status{ConfigKey: "orders", RPS: 99, InFlight: 13}

			assertAssignmentTier(t, store.AssignmentFor(snapshot, low), poolconfig.TierShared)
			assertAssignmentTier(t, store.AssignmentFor(snapshot, high), poolconfig.TierHot)
			assertAssignmentTier(t, store.AssignmentFor(snapshot, low), poolconfig.TierHot)
			now = now.Add(29 * time.Second)
			assertAssignmentTier(t, store.AssignmentFor(snapshot, low), poolconfig.TierHot)

			// 임계치에 다시 도달하면 최초의 복귀 대기 시간은 버립니다.
			assertAssignmentTier(t, store.AssignmentFor(snapshot, high), poolconfig.TierHot)
			assertAssignmentTier(t, store.AssignmentFor(snapshot, low), poolconfig.TierHot)
			now = now.Add(29 * time.Second)
			assertAssignmentTier(t, store.AssignmentFor(snapshot, low), poolconfig.TierHot)
			now = now.Add(time.Second)
			assertAssignmentTier(t, store.AssignmentFor(snapshot, low), poolconfig.TierShared)
			assertAssignmentTier(t, store.AssignmentFor(snapshot, high), poolconfig.TierHot)
		})
	}
}

func TestAssignmentPromotesImmediatelyDuringSharedReturnDelay(t *testing.T) {
	now := time.Unix(100, 0)
	store := NewStore()
	store.clock = func() time.Time { return now }
	snapshot := assignmentSnapshot("orders", &internalconfig.ThresholdSettings{
		Normal: internalconfig.ThresholdPoint{InFlight: 2},
		Hot:    internalconfig.ThresholdPoint{InFlight: 14},
		Super:  internalconfig.ThresholdPoint{InFlight: 50},
	})
	status := traffic.Status{ConfigKey: "orders", InFlight: 2}
	assertAssignmentTier(t, store.AssignmentFor(snapshot, status), poolconfig.TierNormal)
	status.InFlight = 0
	assertAssignmentTier(t, store.AssignmentFor(snapshot, status), poolconfig.TierNormal)
	status.InFlight = 50
	assertAssignmentTier(t, store.AssignmentFor(snapshot, status), poolconfig.TierSuper)
}

func TestSharedReturnTimersAreIndependentForEachService(t *testing.T) {
	now := time.Unix(100, 0)
	store := NewStore()
	store.clock = func() time.Time { return now }
	snapshot := assignmentSnapshot("orders", &internalconfig.ThresholdSettings{
		Hot: internalconfig.ThresholdPoint{InFlight: 2},
	})
	snapshot.Services["payments"] = snapshot.Services["orders"]
	for _, service := range []string{"orders", "payments"} {
		status := traffic.Status{ConfigKey: service, InFlight: 2}
		assertAssignmentTier(t, store.AssignmentFor(snapshot, status), poolconfig.TierHot)
	}
	orders := traffic.Status{ConfigKey: "orders"}
	payments := traffic.Status{ConfigKey: "payments"}
	assertAssignmentTier(t, store.AssignmentFor(snapshot, orders), poolconfig.TierHot)
	now = now.Add(20 * time.Second)
	assertAssignmentTier(t, store.AssignmentFor(snapshot, payments), poolconfig.TierHot)
	now = now.Add(10 * time.Second)
	assertAssignmentTier(t, store.AssignmentFor(snapshot, orders), poolconfig.TierShared)
	assertAssignmentTier(t, store.AssignmentFor(snapshot, payments), poolconfig.TierHot)
	now = now.Add(20 * time.Second)
	assertAssignmentTier(t, store.AssignmentFor(snapshot, payments), poolconfig.TierShared)
}

func TestAssignmentAppliesPolicyChangesWithoutWaitingForSharedReturn(t *testing.T) {
	for _, removePolicy := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed", true: "removed"}[removePolicy], func(t *testing.T) {
			store := NewStore()
			oldSnapshot := assignmentSnapshot("orders", &internalconfig.ThresholdSettings{
				Hot: internalconfig.ThresholdPoint{InFlight: 2},
			})
			status := traffic.Status{ConfigKey: "orders", InFlight: 2}
			assertAssignmentTier(t, store.AssignmentFor(oldSnapshot, status), poolconfig.TierHot)

			threshold := &internalconfig.ThresholdSettings{
				Hot: internalconfig.ThresholdPoint{InFlight: 100},
			}
			if removePolicy {
				threshold = nil
			}
			newSnapshot := assignmentSnapshot("orders", threshold)
			newSnapshot.Revision = 2
			assertAssignmentTier(t, store.AssignmentFor(newSnapshot, status), poolconfig.TierShared)
			// 오래된 요청이 도착해도 새 정책의 복귀 상태가 오염되지 않아야 합니다.
			assertAssignmentTier(t, store.AssignmentFor(oldSnapshot, status), poolconfig.TierHot)
			assertAssignmentTier(t, store.AssignmentFor(newSnapshot, status), poolconfig.TierShared)
		})
	}
}

func TestUnrelatedSnapshotRevisionDoesNotResetSharedReturnDelay(t *testing.T) {
	now := time.Unix(100, 0)
	store := NewStore()
	store.clock = func() time.Time { return now }
	oldSnapshot := assignmentSnapshot("orders", &internalconfig.ThresholdSettings{
		Hot: internalconfig.ThresholdPoint{InFlight: 2},
	})
	status := traffic.Status{ConfigKey: "orders", InFlight: 2}
	assertAssignmentTier(t, store.AssignmentFor(oldSnapshot, status), poolconfig.TierHot)
	status.InFlight = 0
	assertAssignmentTier(t, store.AssignmentFor(oldSnapshot, status), poolconfig.TierHot)
	now = now.Add(20 * time.Second)
	newSnapshot := *oldSnapshot
	newSnapshot.Revision = 2
	assertAssignmentTier(t, store.AssignmentFor(&newSnapshot, status), poolconfig.TierHot)
	assertAssignmentTier(t, store.AssignmentFor(oldSnapshot, status), poolconfig.TierHot)
	now = now.Add(10 * time.Second)
	assertAssignmentTier(t, store.AssignmentFor(&newSnapshot, status), poolconfig.TierShared)
}

func TestAssignmentSupportsConcurrentRequests(t *testing.T) {
	store := NewStore()
	snapshot := assignmentSnapshot("orders", &internalconfig.ThresholdSettings{
		Hot: internalconfig.ThresholdPoint{InFlight: 2},
	})
	status := traffic.Status{ConfigKey: "orders", InFlight: 2}
	assertAssignmentTier(t, store.AssignmentFor(snapshot, status), poolconfig.TierHot)
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 100 {
				low := traffic.Status{ConfigKey: "orders"}
				assertAssignmentTier(t, store.AssignmentFor(snapshot, low), poolconfig.TierHot)
				assertAssignmentTier(t, store.AssignmentFor(snapshot, status), poolconfig.TierHot)
			}
		})
	}
	wg.Wait()
}

func TestDeleteClearsSharedReturnState(t *testing.T) {
	store := NewStore()
	snapshot := assignmentSnapshot("orders", &internalconfig.ThresholdSettings{
		Hot: internalconfig.ThresholdPoint{InFlight: 2},
	})
	status := traffic.Status{ConfigKey: "orders", InFlight: 2}
	assertAssignmentTier(t, store.AssignmentFor(snapshot, status), poolconfig.TierHot)
	store.Delete(" orders ")
	status.InFlight = 0
	assertAssignmentTier(t, store.AssignmentFor(snapshot, status), poolconfig.TierShared)
}

func assignmentSnapshot(service string, threshold *internalconfig.ThresholdSettings) *internalconfig.Snapshot {
	return &internalconfig.Snapshot{
		Revision: 1,
		Services: map[string]internalconfig.ServiceSettings{
			service: {ServiceName: service, Threshold: threshold},
		},
	}
}

func assertAssignmentTier(t *testing.T, assignment Assignment, tier poolconfig.Tier) {
	t.Helper()
	if assignment.Tier != tier || assignment.Dedicated != (tier != poolconfig.TierShared) {
		t.Errorf("assignment = (tier=%q, dedicated=%t), want (tier=%q, dedicated=%t)",
			assignment.Tier, assignment.Dedicated, tier, tier != poolconfig.TierShared)
	}
}
