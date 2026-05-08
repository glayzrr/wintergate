package pool

import "errors"

var (
	ErrInvalidConfig    = errors.New("invalid pool config")
	ErrInvalidPolicy    = errors.New("invalid traffic policy")
	ErrInvalidConfigKey = errors.New("invalid config key")
	ErrStatusNotFound   = errors.New("traffic status not found")
)
