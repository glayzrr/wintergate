package client

import "errors"

var (
	ErrInvalidKeyID = errors.New("invalid key id")
	ErrInvalidKeySet = errors.New("invalid key set")
	ErrInvalidProviderConfig = errors.New("invalid provider config")
	ErrKeyFetchFailed = errors.New("key fetch failed")
	ErrKeyNotFound = errors.New("key not found")
)
