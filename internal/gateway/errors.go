package gateway

import "errors"

var (
	ErrInvalidRequest     = errors.New("invalid request")
	ErrNilTask            = errors.New("nil task")
	ErrNilTokenDecoder    = errors.New("nil token decoder")
	ErrNilTrafficRecorder = errors.New("nil traffic recorder")
)
