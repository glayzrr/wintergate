package record

import "github.com/prometheus/client_golang/prometheus"

// Recorder HTTP와 pool 메트릭을 함께 기록합니다.
type Recorder struct {
	http *HTTPRecorder
	pool *PoolRecorder
}

// NewRecorder Prometheus 레지스트리에 HTTP와 pool 메트릭 recorder를 등록합니다.
func NewRecorder(registry *prometheus.Registry) *Recorder {
	return &Recorder{
		http: newHTTPRecorder(registry),
		pool: newPoolRecorder(registry),
	}
}

// RecordHTTP HTTP 요청 시작을 기록하고 종료 기록 함수를 반환합니다.
func (r *Recorder) RecordHTTP() HTTPDoneFunc {
	return r.http.Start()
}

// RecordPool 커넥션 풀 사용 시작을 기록하고 종료 기록 함수를 반환합니다.
func (r *Recorder) RecordPool(observation PoolObservation) PoolDoneFunc {
	return r.pool.Start(observation)
}
