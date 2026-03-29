package config

import "errors"

var (
	ErrInvalidConfig     = errors.New("invalid config")
	ErrInvalidKeyID      = errors.New("invalid key id")
	ErrInvalidKeySet     = errors.New("invalid key set")
	ErrKeyNotFound       = errors.New("key not found")
	ErrKeySetUnavailable = errors.New("key set unavailable")
)
