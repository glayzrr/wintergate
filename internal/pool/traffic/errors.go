package traffic

import "errors"

var (
	ErrInvalidConfigKey = errors.New("invalid config key")
	ErrStatusNotFound   = errors.New("traffic status not found")
)
