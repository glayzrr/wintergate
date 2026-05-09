package config

import "errors"

var ErrInvalidSettings = errors.New("invalid settings")
var ErrServiceNotFound = errors.New("service not found")
var ErrInstanceNotFound = errors.New("instance not found")
