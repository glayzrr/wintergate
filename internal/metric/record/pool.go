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

// ConnectionObservation 업스트림 커넥션 획득 결과입니다.
type ConnectionObservation struct {
	Reused       bool
	WasIdle      bool
	WaitDuration time.Duration
}

// PoolDoneFunc 커넥션 풀 사용 종료 시 메트릭을 기록합니다.
type PoolDoneFunc func(PoolResult)

// PoolRecorder 커넥션 풀과 업스트림 요청 메트릭을 기록합니다.
type PoolRecorder struct {
	selections *prometheus.CounterVec
	requests   *prometheus.CounterVec
	duration   *prometheus.HistogramVec
	inFlight   *prometheus.GaugeVec

	connectionEvents *prometheus.CounterVec
	connectionWait   *prometheus.HistogramVec
}

func newPoolRecorder(registry *prometheus.Registry) *PoolRecorder {
	recorder := &PoolRecorder{
		// 서비스가 어떤 tier와 pool 종류를 선택했는지 누적합니다.
		selections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "pool",
				Name:      "selections_total",
				Help:      "Total number of connection pool selections.",
			},
			[]string{labelService, labelTier, labelPool},
		),
		// 업스트림 요청의 최종 상태 코드를 pool 선택 결과와 함께 누적합니다.
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "upstream",
				Name:      "requests_total",
				Help:      "Total number of upstream requests.",
			},
			[]string{labelService, labelTier, labelPool, labelStatusCode, labelResult},
		),
		// 업스트림 요청이 시작된 뒤 응답 헤더를 받은 시점까지의 시간을 기록합니다.
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
		// 현재 특정 service/tier/pool 조합을 사용 중인 요청 수를 기록합니다.
		inFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "pool",
				Name:      "requests_in_flight",
				Help:      "Current number of requests using a connection pool.",
			},
			[]string{labelService, labelTier, labelPool},
		),
		// httptrace가 알려주는 신규 연결과 재사용 연결 이벤트를 누적합니다.
		connectionEvents: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "upstream_connection",
				Name:      "events_total",
				Help:      "Total number of upstream connection events.",
			},
			[]string{labelService, labelTier, labelPool, labelEvent},
		),
		// Transport에서 커넥션을 얻기까지 걸린 시간을 기록합니다.
		connectionWait: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "upstream_connection",
				Name:      "wait_duration_seconds",
				Help:      "Time spent waiting for an upstream connection.",
				Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
			},
			[]string{labelService, labelTier, labelPool},
		),
	}

	if registry != nil {
		registry.MustRegister(
			recorder.selections,
			recorder.requests,
			recorder.duration,
			recorder.inFlight,
			recorder.connectionEvents,
			recorder.connectionWait,
		)
	}

	return recorder
}

func (r *PoolRecorder) recordPool(observation PoolObservation) PoolDoneFunc {
	startedAt := time.Now()
	service := normalizeMetricValue(observation.Service)
	tier := normalizeMetricValue(observation.Tier)
	pool := poolFor(observation.Dedicated)

	// pool 선택은 요청 시작 시점에 한 번만 기록합니다.
	r.selections.WithLabelValues(service, tier, pool).Inc()
	r.inFlight.WithLabelValues(service, tier, pool).Inc()

	var once sync.Once
	return func(result PoolResult) {
		once.Do(func() {
			// 요청 종료 시 pool in-flight를 감소시키고 업스트림 결과를 기록합니다.
			r.inFlight.WithLabelValues(service, tier, pool).Dec()

			statusCode := normalizeStatusCode(result.StatusCode)
			status := strconv.Itoa(statusCode)
			metricResult := resultFor(statusCode)

			r.requests.WithLabelValues(service, tier, pool, status, metricResult).Inc()
			r.duration.WithLabelValues(service, tier, pool, status, metricResult).Observe(time.Since(startedAt).Seconds())
		})
	}
}

func (r *PoolRecorder) recordConnection(observation PoolObservation, connection ConnectionObservation) {
	service := normalizeMetricValue(observation.Service)
	tier := normalizeMetricValue(observation.Tier)
	pool := poolFor(observation.Dedicated)

	event := connectionEventFor(connection)
	r.connectionEvents.WithLabelValues(service, tier, pool, event).Inc()

	waitDuration := connection.WaitDuration
	if waitDuration < 0 {
		waitDuration = 0
	}
	r.connectionWait.WithLabelValues(service, tier, pool).Observe(waitDuration.Seconds())
}

func connectionEventFor(connection ConnectionObservation) string {
	switch {
	case connection.Reused && connection.WasIdle:
		return connectionEventIdleReused
	case connection.Reused:
		return connectionEventReused
	default:
		return connectionEventNew
	}
}

func normalizeMetricValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return unknown
	}

	return value
}

func poolFor(dedicated bool) string {
	if dedicated {
		return poolDedicated
	}

	return poolShared
}
