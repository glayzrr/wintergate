package client

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
)

type fetcher struct {
	client *http.Client
	url    string
}

func (f fetcher) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request: %w", ErrKeyFetchFailed, err)
	}

	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: request %s: %w", ErrKeyFetchFailed, f.url, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", ErrKeyFetchFailed, response.StatusCode)
	}

	var keyDocument document
	if err := json.NewDecoder(response.Body).Decode(&keyDocument); err != nil {
		return nil, fmt.Errorf("%w: decode response body: %w", ErrInvalidKeySet, err)
	}

	keys, err := keyDocument.publicKeys()
	if err != nil {
		return nil, err
	}

	return keys, nil
}
