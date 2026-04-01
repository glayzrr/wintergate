package gateway

import "errors"

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrNilTask        = errors.New("nil task")
)
