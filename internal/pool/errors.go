package pool

import "errors"

var (
	ErrInvalidConfig  = errors.New("invalid pool config")
	ErrInvalidPolicy  = errors.New("invalid traffic policy")
	ErrInvalidService = errors.New("invalid service")
	ErrStatusNotFound = errors.New("traffic status not found")
)
