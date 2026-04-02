package route

import "errors"

var (
	ErrServiceNotFound = errors.New("cannot find service")
	ErrNilRegistry     = errors.New("registry is required")
)
