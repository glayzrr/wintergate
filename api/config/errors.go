package configapi

import "errors"

var (
	ErrInvalidSnapshot = errors.New("invalid snapshot")
	ErrNilRegisterer   = errors.New("nil registerer")
)
