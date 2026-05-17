package health

import (
	"context"
	"net"
	"time"

	internalconfig "wintergate/internal/config"
)

// targetKey 헬스 체크 target을 서비스와 인스턴스 주소로 식별합니다.
type targetKey struct {
	serviceName string
	scheme      string
	host        string
	port        string
	healthKey  string
}

// instance targetKey를 로그에 사용할 host:port 문자열로 변환합니다.
func (k targetKey) instance() string {
	return net.JoinHostPort(k.host, k.port)
}

// key 라우팅 hot path에서 사용하는 사전 계산된 health 상태 조회 키를 반환합니다.
func (k targetKey) key() string {
	return k.healthKey
}

// target 하나의 인스턴스 헬스 체크 루프가 사용하는 고정 설정입니다.
type target struct {
	key        targetKey
	instance   internalconfig.InstanceSettings
	settings   runtimeSettings
	jitter     time.Duration
	generation uint64
	cancel     context.CancelFunc
}

// runtimeSettings 헬스 체크 루프에서 즉시 사용할 수 있도록 파싱된 설정입니다.
type runtimeSettings struct {
	enabled          bool
	path             string
	interval         time.Duration
	timeout          time.Duration
	jitter           time.Duration
	maxBackoff       time.Duration
	failureThreshold int
	successThreshold int
}

// statusRecord 라우팅 후보 판단에 사용하는 마지막 헬스 체크 상태입니다.
type statusRecord struct {
	status               Status
	checkedAt            time.Time
	consecutiveFailures  int
	consecutiveSuccesses int
	generation           uint64
	lastError            string
}

// desiredTarget 중앙 설정 스냅샷에서 계산한 목표 헬스 체크 target입니다.
type desiredTarget struct {
	key      targetKey
	instance internalconfig.InstanceSettings
	settings runtimeSettings
	jitter   time.Duration
}
