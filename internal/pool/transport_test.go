package pool

import (
	"net/http"
	"testing"
	"time"
)

func TestNewTransportAppliesTierConfig(t *testing.T) {
	transport, err := NewTransport(TierSuper)
	if err != nil {
		t.Fatalf("NewTransport returned error: %v", err)
	}

	if transport.MaxIdleConns != 3000 {
		t.Fatalf("transport.MaxIdleConns = %d, want %d", transport.MaxIdleConns, 3000)
	}
	if transport.MaxIdleConnsPerHost != 300 {
		t.Fatalf("transport.MaxIdleConnsPerHost = %d, want %d", transport.MaxIdleConnsPerHost, 300)
	}
	if transport.MaxConnsPerHost != 800 {
		t.Fatalf("transport.MaxConnsPerHost = %d, want %d", transport.MaxConnsPerHost, 800)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Fatalf("transport.IdleConnTimeout = %s, want %s", transport.IdleConnTimeout, 90*time.Second)
	}
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("transport.ResponseHeaderTimeout = %s, want %s", transport.ResponseHeaderTimeout, 30*time.Second)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Fatalf("transport.TLSHandshakeTimeout = %s, want %s", transport.TLSHandshakeTimeout, 10*time.Second)
	}
	if transport.ExpectContinueTimeout != time.Second {
		t.Fatalf("transport.ExpectContinueTimeout = %s, want %s", transport.ExpectContinueTimeout, time.Second)
	}
}

func TestNewTransportClonesDefaultTransport(t *testing.T) {
	transport, err := NewTransport("")
	if err != nil {
		t.Fatalf("NewTransport returned error: %v", err)
	}

	defaultTransport := http.DefaultTransport.(*http.Transport)
	if transport == defaultTransport {
		t.Fatal("NewTransport returned http.DefaultTransport")
	}
	if transport.Proxy == nil {
		t.Fatal("transport.Proxy is nil, want cloned default proxy function")
	}
	if transport.DialContext == nil {
		t.Fatal("transport.DialContext is nil, want cloned default dialer")
	}
}
