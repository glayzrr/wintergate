package config

import (
	"encoding/json"
	"testing"
)

func TestSettingsUnmarshalParsesGlobalThresholdAndEndpoints(t *testing.T) {
	var settings Settings

	err := json.Unmarshal([]byte(`{"global":{"auth":{"jwt_algorithm":"HS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwt_secret":"secret"}},"threshold":{"hot":{"rps":100,"in-flight":14},"super":{"rps":150,"in-flight":50}},"endpoints":[{"path":"/api/order","method":"POST","roles":["ADMIN","OPS"]},{"path":"/v3/api-docs/**","method":"GET","roles":[]}]}`), &settings)
	if err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if settings.Global == nil {
		t.Fatal("settings.Global is nil")
	}

	if settings.Global.Auth == nil {
		t.Fatal("settings.Global.Auth is nil")
	}

	if settings.Threshold == nil {
		t.Fatal("settings.Threshold is nil")
	}

	if settings.Threshold.Hot.InFlight != 14 {
		t.Fatalf("settings.Threshold.Hot.InFlight = %d, want %d", settings.Threshold.Hot.InFlight, 14)
	}

	if len(settings.Endpoints) != 2 {
		t.Fatalf("len(settings.Endpoints) = %d, want %d", len(settings.Endpoints), 2)
	}

	if len(settings.Endpoints[1].Roles) != 0 {
		t.Fatalf("len(settings.Endpoints[1].Roles) = %d, want %d", len(settings.Endpoints[1].Roles), 0)
	}
}
