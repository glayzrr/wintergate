package configapi

const (
	// ConfigApplyRoute 설정 정보를 수신하는 기본 API 경로입니다.
	ConfigApplyRoute = "/api/config"
	// ConfigForRoute 서비스 이름으로 등록된 설정 정보를 조회하는 API 경로입니다.
	ConfigForRoute = ConfigApplyRoute + "/:serviceName"

	responseRegisterSuccess = "config registered"
	responseBindFailed      = "invalid config payload"
	responseRegisterFailed  = "failed to register config"
	responseConfigFound     = "config found"
	responseConfigNotFound  = "config not found"
)
