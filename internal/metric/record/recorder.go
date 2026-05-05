package record

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Recorder HTTP와 pool 메트릭을 함께 기록합니다.
type Recorder struct {
	http *RequestRecorder
	pool *PoolRecorder
}

// NewRecorder Prometheus 레지스트리에 HTTP와 pool 메트릭 recorder를 등록합니다.
func NewRecorder(registry *prometheus.Registry) *Recorder {
	return &Recorder{
		http: newRequestRecorder(registry),
		pool: newPoolRecorder(registry),
	}
}

// RecordHTTP HTTP 요청 시작을 기록하고 종료 기록 함수를 반환합니다.
func (r *Recorder) RecordHTTP() RequestDoneFunc {
	return r.http.recordRequest()
}

// RecordPool 커넥션 풀 사용 시작을 기록하고 종료 기록 함수를 반환합니다.
func (r *Recorder) RecordPool(observation PoolObservation) PoolDoneFunc {
	return r.pool.recordPool(observation)
}

// RecordConnection 업스트림 커넥션 획득 결과를 기록합니다.
func (r *Recorder) RecordConnection(observation PoolObservation, connection ConnectionObservation) {
	if r == nil {
		return
	}

	r.pool.recordConnection(observation, connection)
}
