package record

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// RequestObservation HTTP 요청 처리 결과입니다.
type RequestObservation struct {
	Method     string
	Path       string
	StatusCode int
}

// RequestDoneFunc HTTP 요청 종료 시 메트릭을 기록합니다.
type RequestDoneFunc func(RequestObservation)

// RequestRecorder HTTP 요청 메트릭을 기록합니다.
type RequestRecorder struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
	failures *prometheus.CounterVec
}

func newRequestRecorder(registry *prometheus.Registry) *RequestRecorder {
	recorder := &RequestRecorder{
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests handled by Wintergate.",
			},
			[]string{labelRoute, labelMethod, labelStatusCode, labelResult},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{labelRoute, labelMethod, labelStatusCode, labelResult},
		),
		inFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: "http",
				Name:      "requests_in_flight",
				Help:      "Current number of HTTP requests being handled by Wintergate.",
			},
		),
		failures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: "http",
				Name:      "request_failures_total",
				Help:      "Total number of failed HTTP requests handled by Wintergate.",
			},
			[]string{labelRoute, labelMethod, labelStatusCode},
		),
	}

	if registry != nil {
		registry.MustRegister(recorder.requests, recorder.duration, recorder.inFlight, recorder.failures)
	}

	return recorder
}

func (r *RequestRecorder) recordRequest() RequestDoneFunc {
	startedAt := time.Now()
	r.inFlight.Inc()

	var once sync.Once
	return func(observation RequestObservation) {
		once.Do(func() {
			// 요청 완료 시점에 현재 처리 중인 HTTP 요청 수를 되돌립니다.
			r.inFlight.Dec()

			path := normalizePath(observation.Path)
			method := normalizeMethod(observation.Method)
			statusCode := normalizeStatusCode(observation.StatusCode)
			status := strconv.Itoa(statusCode)
			result := resultFor(statusCode)

			// HTTP 요청 수와 처리 시간을 route/method/status/result 단위로 기록합니다.
			r.requests.WithLabelValues(path, method, status, result).Inc()
			r.duration.WithLabelValues(path, method, status, result).Observe(time.Since(startedAt).Seconds())

			if result == resultError {
				// 실패 카운터는 4xx/5xx 응답만 별도로 집계합니다.
				r.failures.WithLabelValues(path, method, status).Inc()
			}
		})
	}
}

func normalizeMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodGet
	}

	return method
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return routeGateway
	}

	return path
}

func normalizeStatusCode(statusCode int) int {
	if statusCode == 0 {
		return http.StatusOK
	}

	return statusCode
}

func resultFor(statusCode int) string {
	if statusCode >= http.StatusBadRequest {
		return resultError
	}

	return resultSuccess
}
