package config

import (
	"encoding/json"
	"testing"
)

func TestSettingsUnmarshalParsesGlobalAndRoutes(t *testing.T) {
	var settings Settings

	err := json.Unmarshal([]byte(`{"global":{"auth":{"jwt_algorithm":"HS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwt_secret":"secret"}},"routes":[{"name":"order-service","host":"localhost","port":8080,"threshold":{"hot":{"rps":100,"in-flight":14},"super":{"rps":150,"in-flight":50}},"endpoints":[{"path":"/api/order","method":"POST","roles":["ADMIN","OPS"]},{"path":"/v3/api-docs/**","method":"GET","roles":[]}]}]}`), &settings)
	if err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if settings.Global == nil {
		t.Fatal("settings.Global is nil")
	}

	if settings.Global.Auth == nil {
		t.Fatal("settings.Global.Auth is nil")
	}

	if len(settings.Routes) != 1 {
		t.Fatalf("len(settings.Routes) = %d, want %d", len(settings.Routes), 1)
	}

	if settings.Routes[0].Name != "order-service" {
		t.Fatalf("settings.Routes[0].Name = %q, want %q", settings.Routes[0].Name, "order-service")
	}

	if settings.Routes[0].Threshold == nil {
		t.Fatal("settings.Routes[0].Threshold is nil")
	}

	if settings.Routes[0].Threshold.Hot.InFlight != 14 {
		t.Fatalf("settings.Routes[0].Threshold.Hot.InFlight = %d, want %d", settings.Routes[0].Threshold.Hot.InFlight, 14)
	}

	if len(settings.Routes[0].Endpoints) != 2 {
		t.Fatalf("len(settings.Routes[0].Endpoints) = %d, want %d", len(settings.Routes[0].Endpoints), 2)
	}

	if len(settings.Routes[0].Endpoints[1].Roles) != 0 {
		t.Fatalf("len(settings.Routes[0].Endpoints[1].Roles) = %d, want %d", len(settings.Routes[0].Endpoints[1].Roles), 0)
	}
}
