package auth

import "errors"

var (
	ErrConfigUnavailable          = errors.New("config unavailable")
	ErrInvalidAudience            = errors.New("invalid audience")
	ErrInvalidAuthorizationHeader = errors.New("invalid authorization header")
	ErrInvalidIssuer              = errors.New("invalid issuer")
	ErrInvalidSignature           = errors.New("invalid signature")
	ErrInvalidToken               = errors.New("invalid token")
	ErrNilRegistry                = errors.New("nil registry")
	ErrTokenExpired               = errors.New("token expired")
	ErrTokenNotYetValid           = errors.New("token not yet valid")
	ErrUnsupportedAlgorithm       = errors.New("unsupported algorithm")
)
