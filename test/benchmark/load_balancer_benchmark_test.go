package benchmark

import (
	"testing"
	"time"

	internalconfig "wintergate/internal/config"
	"wintergate/internal/route/config"
)

type loadBalancerBenchmarkCase struct {
	name         string
	provider     benchmarkProviderFactory
	flapInterval time.Duration
}

// BenchmarkLoadBalancerHealthProvider health 상태 조회 방식만 바꿔
// 라우팅 hot path 처리량을 비교합니다.
func BenchmarkLoadBalancerHealthProvider(b *testing.B) {
	// 환경 변수로 인스턴스 수, unhealthy 비율, 상태 변경 주기를 바꿔
	// 같은 harness를 여러 조건에 재사용합니다.
	options := loadBalancerBenchmarkOptionsFromEnv(b)

	// 모든 비교군은 동일한 서비스 snapshot과 health key 목록을
	// 사용해야 결과를 직접 비교할 수 있습니다.
	snapshot, healthKeys := benchmarkSnapshot(options.instances)

	for _, benchmarkCase := range loadBalancerBenchmarkCases(options) {
		b.Run(benchmarkCase.name, func(b *testing.B) {
			runLoadBalancerBenchmarkCase(b, benchmarkCase, snapshot, healthKeys, options)
		})
	}
}

// loadBalancerBenchmarkCases provider와 flapping 주기를 조합해
// go test가 실행할 sub-benchmark 목록을 만듭니다.
func loadBalancerBenchmarkCases(options loadBalancerBenchmarkOptions) []loadBalancerBenchmarkCase {
	providers := benchmarkProviderFactories()
	cases := make([]loadBalancerBenchmarkCase, 0, len(options.flapIntervals)*len(providers))
	for _, flapInterval := range options.flapIntervals {
		for _, provider := range providers {
			cases = append(cases, loadBalancerBenchmarkCase{
				name:         benchmarkName(provider.name, flapInterval),
				provider:     provider,
				flapInterval: flapInterval,
			})
		}
	}

	return cases
}

// runLoadBalancerBenchmarkCase 하나의 provider와 flapping 조건을 실행하고
// 측정 결과를 go test metric으로 보고합니다.
func runLoadBalancerBenchmarkCase(b *testing.B, benchmarkCase loadBalancerBenchmarkCase, snapshot *internalconfig.Snapshot, healthKeys []string, options loadBalancerBenchmarkOptions) {
	b.Helper()

	provider := benchmarkCase.provider.newProvider(healthKeys)
	loadBalancer := config.NewLoadBalancer(provider)

	// 별도 goroutine에서 health 상태를 계속 바꿔 read/write 경합이
	// 있는 운영 상황을 시뮬레이션합니다.
	stopFlapping := startBenchmarkHealthFlapping(provider, healthKeys, benchmarkCase.flapInterval, options.unhealthyEvery)

	b.ReportAllocs()
	result := measureLoadBalancerBenchmark(b, loadBalancer, snapshot, options.sampleEvery)

	// benchmark 종료 후 flapping goroutine을 멈추고 RPS, p95, p99
	// 보조 지표를 출력합니다.
	result.flaps = stopFlapping()
	reportLoadBalancerBenchmarkMetrics(b, result)
}

// measureLoadBalancerBenchmark 측정 구간을 NextInstance 호출에만 맞춥니다.
func measureLoadBalancerBenchmark(b *testing.B, loadBalancer *config.LoadBalancer, snapshot *internalconfig.Snapshot, sampleEvery int) loadBalancerBenchmarkResult {
	b.Helper()

	b.ResetTimer()
	startedAt := time.Now()
	samples := runLoadBalancerParallelBenchmark(b, loadBalancer, snapshot, sampleEvery)
	elapsed := time.Since(startedAt)
	b.StopTimer()

	return loadBalancerBenchmarkResult{
		samples: samples,
		elapsed: elapsed,
	}
}
