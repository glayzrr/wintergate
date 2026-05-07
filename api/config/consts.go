package configapi

const (
	// ConfigRoute 설정 정보를 수신하는 기본 API 경로입니다.
	ConfigRoute = "/api/config"

	requestHeaderHost = "X-Service-Host"
	requestHeaderPort = "X-Service-Port"

	responseRegisterSuccess = "config registered"
	responseBindFailed      = "invalid config payload"
	responseRegisterFailed  = "failed to register config"
)
