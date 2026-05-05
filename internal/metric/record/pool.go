package record

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// PoolObservation 커넥션 풀 사용 시작 정보입니다.
type PoolObservation struct {
	Service   string
	Tier      string
	Dedicated bool
}

// PoolResult 커넥션 풀을 사용한 업스트림 요청 결과입니다.
type PoolResult struct {
	StatusCode int
}

// PoolDoneFunc 커넥션 풀 사용 종료 시 메트릭을 기록합니다.
type PoolDoneFunc func(PoolResult)

// PoolRecorder 커넥션 풀과 업스트림 요청 메트릭을 기록합니다.
type PoolRecorder struct {
	selections *prometheus.CounterVec
	requests   *prometheus.CounterVec
	duration   *prometheus.HistogramVec
}

func newPoolRecorder(registry *prometheus.Registry) *PoolRecorder {
	recorder := &PoolRecorder{
		selections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "pool",
				Name:      "selections_total",
				Help:      "Total number of connection pool selections.",
			},
			[]string{labelService, labelTier, labelPool},
		),
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "upstream",
				Name:      "requests_total",
				Help:      "Total number of upstream requests.",
			},
			[]string{labelService, labelTier, labelPool, labelStatusCode, labelResult},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "upstream",
				Name:      "request_duration_seconds",
				Help:      "Upstream request duration in seconds.",
				Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{labelService, labelTier, labelPool, labelStatusCode, labelResult},
		),
	}

	if registry != nil {
		registry.MustRegister(recorder.selections, recorder.requests, recorder.duration)
	}

	return recorder
}

// Start 커넥션 풀 선택을 기록하고 업스트림 요청 종료 기록 함수를 반환합니다.
func (r *PoolRecorder) Start(observation PoolObservation) PoolDoneFunc {
	startedAt := time.Now()
	service := normalizeService(observation.Service)
	tier := normalizeMetricValue(observation.Tier)
	pool := poolFor(observation.Dedicated)

	r.selections.WithLabelValues(service, tier, pool).Inc()

	var once sync.Once
	return func(result PoolResult) {
		once.Do(func() {
			statusCode := normalizeStatusCode(result.StatusCode)
			status := strconv.Itoa(statusCode)
			metricResult := resultFor(statusCode)

			r.requests.WithLabelValues(service, tier, pool, status, metricResult).Inc()
			r.duration.WithLabelValues(service, tier, pool, status, metricResult).Observe(time.Since(startedAt).Seconds())
		})
	}
}

func normalizeService(service string) string {
	service = strings.TrimSpace(service)
	if service == "" {
		return serviceUnknown
	}

	return service
}

func normalizeMetricValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return valueUnknown
	}

	return value
}

func poolFor(dedicated bool) string {
	if dedicated {
		return poolDedicated
	}

	return poolShared
}
