package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetcherFetchReturnsErrorWhenStatusNotOK(t *testing.T) {
	service := newTestKeyService(t, `{"error":"failed"}`)
	service.setResponse(http.StatusInternalServerError, `{"error":"failed"}`, 0)
	server := httptest.NewServer(service)
	defer server.Close()

	clientKeys, err := (fetcher{
		client: &http.Client{Timeout: time.Second},
		url:    server.URL,
	}).fetch(context.Background())
	if err == nil {
		t.Fatal("fetch returned nil error")
	}

	if clientKeys != nil {
		t.Fatalf("clientKeys = %#v, want nil", clientKeys)
	}

	if !errors.Is(err, ErrKeyFetchFailed) {
		t.Fatalf("error = %v, want ErrKeyFetchFailed", err)
	}
}

func TestFetcherFetchReturnsErrorWhenPayloadInvalidJSON(t *testing.T) {
	service := newTestKeyService(t, `{`)
	server := httptest.NewServer(service)
	defer server.Close()

	clientKeys, err := (fetcher{
		client: &http.Client{Timeout: time.Second},
		url:    server.URL,
	}).fetch(context.Background())
	if err == nil {
		t.Fatal("fetch returned nil error")
	}

	if clientKeys != nil {
		t.Fatalf("clientKeys = %#v, want nil", clientKeys)
	}

	if !errors.Is(err, ErrInvalidKeySet) {
		t.Fatalf("error = %v, want ErrInvalidKeySet", err)
	}
}

func TestFetcherFetchReturnsKeys(t *testing.T) {
	privateKey := generateRSAKey(t)
	service := newTestKeyService(t, mustMarshalDocument(t, newRSAKey("key-1", &privateKey.PublicKey)))
	server := httptest.NewServer(service)
	defer server.Close()

	clientKeys, err := (fetcher{
		client: &http.Client{Timeout: time.Second},
		url:    server.URL,
	}).fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch returned error: %v", err)
	}

	if !equalPublicKeys(clientKeys["key-1"], &privateKey.PublicKey) {
		t.Fatal("clientKeys[key-1] does not match the served key")
	}
}
