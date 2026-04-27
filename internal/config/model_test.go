package config

import (
	"encoding/json"
	"testing"
)

func TestSettingsUnmarshalParsesRoutesAndRateLimit(t *testing.T) {
	var settings Settings

	err := json.Unmarshal([]byte(`{"auth":{"jwt_algorithm":"HS256","jwt_audience":"wintergate","jwt_clock_skew":"1m","jwt_issuer":"auth-service","jwt_secret":"secret"},"routes":{"protected":[{"path":"/api/order","method":"POST","service":"order-service","roles":["ADMIN","OPS"],"time_window":{"start":"09:00","end":"18:00","timezone":"Asia/Seoul"}},{"path":"/v3/api-docs/**","method":"GET","service":"order-service","roles":["ADMIN"]}]},"rate_limit":[{"path":"/api/order","method":"POST","service":"order-service","roles":["anyone"],"duration":"1m","limit":10}]}`), &settings)
	if err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if settings.Auth == nil {
		t.Fatal("settings.Auth is nil")
	}

	if settings.Routes == nil {
		t.Fatal("settings.Routes is nil")
	}

	if len(settings.Routes.Protected) != 2 {
		t.Fatalf("len(settings.Routes.Protected) = %d, want %d", len(settings.Routes.Protected), 2)
	}

	if settings.Routes.Protected[0].AccessWindow == nil {
		t.Fatal("settings.Routes.Protected[0].AccessWindow is nil")
	}

	if settings.Routes.Protected[0].AccessWindow.Timezone != "Asia/Seoul" {
		t.Fatalf(
			"settings.Routes.Protected[0].AccessWindow.Timezone = %q, want %q",
			settings.Routes.Protected[0].AccessWindow.Timezone,
			"Asia/Seoul",
		)
	}

	if len(settings.RateLimit) != 1 {
		t.Fatalf("len(settings.RateLimit) = %d, want %d", len(settings.RateLimit), 1)
	}

	if settings.RateLimit[0].Limit != 10 {
		t.Fatalf("settings.RateLimit[0].Limit = %d, want %d", settings.RateLimit[0].Limit, 10)
	}

	if settings.RateLimit[0].Duration != "1m" {
		t.Fatalf("settings.RateLimit[0].Duration = %q, want %q", settings.RateLimit[0].Duration, "1m")
	}
}
