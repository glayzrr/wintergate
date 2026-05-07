package utils

import "testing"

func TestConfigKeyNormalizesHostAndPort(t *testing.T) {
	key, err := ConfigKey(" LOCALHOST ", "08080")
	if err != nil {
		t.Fatalf("ConfigKey returned error: %v", err)
	}

	if key != "localhost:8080" {
		t.Fatalf("key = %q, want %q", key, "localhost:8080")
	}
}

func TestConfigKeyReturnsErrorWhenInvalid(t *testing.T) {
	tests := []struct {
		name string
		host string
		port string
	}{
		{name: "host missing", port: "8080"},
		{name: "port missing", host: "localhost"},
		{name: "port invalid", host: "localhost", port: "0"},
		{name: "host includes scheme", host: "http://localhost", port: "8080"},
		{name: "host includes path", host: "localhost/api", port: "8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ConfigKey(tt.host, tt.port); err == nil {
				t.Fatal("ConfigKey returned nil error")
			}
		})
	}
}
