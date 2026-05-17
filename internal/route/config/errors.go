package config

import "errors"

var ErrInvalidConfig = errors.New("invalid config")
var ErrConfigNotFound = errors.New("config not found")
var ErrNoHealthyInstance = errors.New("no healthy instance")
