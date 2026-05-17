package benchmark

import (
	"sort"
	"sync"
	"testing"
	"time"

	internalconfig "wintergate/internal/config"
	"wintergate/internal/route/config"
)

type loadBalancerBenchmarkResult struct {
	samples []time.Duration
	elapsed time.Duration
	flaps   uint64
}

// runLoadBalancerParallelBenchmark 여러 goroutine에서 동시에
// NextInstance를 호출해 요청 hot path를 압박합니다.
func runLoadBalancerParallelBenchmark(b *testing.B, loadBalancer *config.LoadBalancer, snapshot *internalconfig.Snapshot, sampleEvery int) []time.Duration {
	b.Helper()

	var samplesMu sync.Mutex
	samples := make([]time.Duration, 0)
	b.RunParallel(func(pb *testing.PB) {
		// 모든 호출 시간을 재면 측정 오버헤드가 커지므로 일부 요청만
		// 샘플링해 p95, p99를 계산합니다.
		localSamples := make([]time.Duration, 0, 1024)
		operations := 0
		for pb.Next() {
			operations++
			if operations%sampleEvery == 0 {
				startedAt := time.Now()
				if !selectNextBenchmarkInstance(b, loadBalancer, snapshot) {
					return
				}
				localSamples = append(localSamples, time.Since(startedAt))
				continue
			}

			if !selectNextBenchmarkInstance(b, loadBalancer, snapshot) {
				return
			}
		}

		samplesMu.Lock()
		samples = append(samples, localSamples...)
		samplesMu.Unlock()
	})

	return samples
}

// selectNextBenchmarkInstance benchmark 대상인 NextInstance 호출과 에러
// 처리를 한 곳에 모읍니다.
func selectNextBenchmarkInstance(b *testing.B, loadBalancer *config.LoadBalancer, snapshot *internalconfig.Snapshot) bool {
	b.Helper()

	if _, err := loadBalancer.NextInstance(snapshot, benchmarkServiceName); err != nil {
		b.Errorf("select next instance: %v", err)
		return false
	}

	return true
}

// reportLoadBalancerBenchmarkMetrics go test 기본 ns/op 외에
// 포트폴리오에 쓰기 좋은 RPS와 tail latency를 출력합니다.
func reportLoadBalancerBenchmarkMetrics(b *testing.B, result loadBalancerBenchmarkResult) {
	b.Helper()

	if result.elapsed > 0 {
		b.ReportMetric(float64(b.N)/result.elapsed.Seconds(), "rps")
	}
	b.ReportMetric(float64(result.flaps), "flaps")
	if len(result.samples) == 0 {
		return
	}

	sort.Slice(result.samples, func(i, j int) bool {
		return result.samples[i] < result.samples[j]
	})
	b.ReportMetric(float64(percentileDuration(result.samples, 0.95).Nanoseconds()), "sampled-p95-ns")
	b.ReportMetric(float64(percentileDuration(result.samples, 0.99).Nanoseconds()), "sampled-p99-ns")
}

// percentileDuration 샘플링한 요청 지연 시간에서 지정한 분위 값을
// 반환합니다.
func percentileDuration(samples []time.Duration, percentile float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}

	index := int(float64(len(samples))*percentile + 0.5)
	if index <= 0 {
		index = 1
	}
	if index > len(samples) {
		index = len(samples)
	}

	return samples[index-1]
}
