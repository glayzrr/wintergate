package configapi

import (
	"encoding/json"
	"testing"
)

func TestSnapshotUnmarshalParsesRoutesAndRateLimit(t *testing.T) {
	var snapshot Snapshot

	err := json.Unmarshal([]byte(`{"auth":{"jwt_algorithm":"HS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwt_secret":"secret"},"routes":{"protected":[{"path":"/api/order","method":"POST","service":"order-service","roles":["ADMIN","OPS"],"time_window":{"start":"09:00","end":"18:00","timezone":"Asia/Seoul"}},{"path":"/v3/api-docs/**","method":"GET","service":"order-service","roles":["ADMIN"]}]},"rate_limit":[{"path":"/api/order","method":"POST","service":"order-service","roles":["anyone"],"duration":"1m","limit":10}]}`), &snapshot)
	if err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if snapshot.Auth == nil {
		t.Fatal("snapshot.Auth is nil")
	}

	if snapshot.Routes == nil {
		t.Fatal("snapshot.Routes is nil")
	}

	if len(snapshot.Routes.Protected) != 2 {
		t.Fatalf("len(snapshot.Routes.Protected) = %d, want %d", len(snapshot.Routes.Protected), 2)
	}

	if snapshot.Routes.Protected[0].TimeWindow == nil {
		t.Fatal("snapshot.Routes.Protected[0].TimeWindow is nil")
	}

	if snapshot.Routes.Protected[0].TimeWindow.Timezone != "Asia/Seoul" {
		t.Fatalf(
			"snapshot.Routes.Protected[0].TimeWindow.Timezone = %q, want %q",
			snapshot.Routes.Protected[0].TimeWindow.Timezone,
			"Asia/Seoul",
		)
	}

	if len(snapshot.RateLimit) != 1 {
		t.Fatalf("len(snapshot.RateLimit) = %d, want %d", len(snapshot.RateLimit), 1)
	}

	if snapshot.RateLimit[0].Limit != 10 {
		t.Fatalf("snapshot.RateLimit[0].Limit = %d, want %d", snapshot.RateLimit[0].Limit, 10)
	}

	if snapshot.RateLimit[0].Duration != "1m" {
		t.Fatalf("snapshot.RateLimit[0].Duration = %q, want %q", snapshot.RateLimit[0].Duration, "1m")
	}
}
