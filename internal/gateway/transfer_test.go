package gateway

import (
	"errors"
	"testing"
)

func TestUpstreamHostBuildsURLFromSchemeHostAndPort(t *testing.T) {
	upstream, err := upstreamHost("https", "localhost", "8443")
	if err != nil {
		t.Fatalf("upstreamHost returned error: %v", err)
	}

	if upstream != "https://localhost:8443" {
		t.Fatalf("upstream = %q, want %q", upstream, "https://localhost:8443")
	}
}

func TestUpstreamHostReturnsErrorWhenSchemeMissing(t *testing.T) {
	_, err := upstreamHost("", "localhost", "8080")
	if err == nil {
		t.Fatal("upstreamHost returned nil error")
	}

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}

func TestUpstreamHostReturnsErrorWhenHostIncludesScheme(t *testing.T) {
	_, err := upstreamHost("https", "https://localhost", "8080")
	if err == nil {
		t.Fatal("upstreamHost returned nil error")
	}

	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v, want ErrInvalidRequest", err)
	}
}
