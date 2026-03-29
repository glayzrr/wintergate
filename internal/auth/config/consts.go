package config

const (
	defaultEnvPath                = ".env"
	supportedJWTAlgorithm         = "RS256"
	envAuthJWKSURL                = "AUTH_JWKS_URL"
	envAuthJWKSRequestTimeout     = "AUTH_JWKS_REQUEST_TIMEOUT"
	envAuthJWKSRefreshInterval    = "AUTH_JWKS_REFRESH_INTERVAL"
	envJWTAlgorithm               = "JWT_ALGORITHM"
	envJWTAudience                = "JWT_AUDIENCE"
	envJWTClockSkew               = "JWT_CLOCK_SKEW"
	envJWTIssuer                  = "JWT_ISSUER"
)
