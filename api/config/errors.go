package configapi

import "errors"

var (
	ErrInvalidSnapshot  = errors.New("invalid snapshot")
	ErrNilAuthRegistry  = errors.New("nil auth registry")
	ErrNilRegisterer    = errors.New("nil registerer")
	ErrNilRoutingRegistry = errors.New("nil routing registry")
)
