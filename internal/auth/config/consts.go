package config

const (
	defaultEnvPath                  = ".env"
	supportedJWTAlgorithm           = "RS256"
	envAuthPublicKeyURL             = "AUTH_PUBLIC_KEY_URL"
	envAuthPublicKeyRequestTimeout  = "AUTH_PUBLIC_KEY_REQUEST_TIMEOUT"
	envAuthPublicKeyRefreshInterval = "AUTH_PUBLIC_KEY_REFRESH_INTERVAL"
	envJWTAlgorithm                 = "JWT_ALGORITHM"
	envJWTAudience                  = "JWT_AUDIENCE"
	envJWTClockSkew                 = "JWT_CLOCK_SKEW"
	envJWTIssuer                    = "JWT_ISSUER"
)
