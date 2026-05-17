package benchmark

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type loadBalancerBenchmarkOptions struct {
	instances      int
	unhealthyEvery int
	sampleEvery    int
	flapIntervals  []time.Duration
}

// loadBalancerBenchmarkOptionsFromEnv 벤치마크 조건을 환경 변수에서
// 읽어 실험 조건을 쉽게 바꿀 수 있게 합니다.
func loadBalancerBenchmarkOptionsFromEnv(b *testing.B) loadBalancerBenchmarkOptions {
	b.Helper()

	options := loadBalancerBenchmarkOptions{
		instances:      benchmarkIntEnv(b, benchmarkEnvInstances, benchmarkDefaultInstances),
		unhealthyEvery: benchmarkIntEnv(b, benchmarkEnvUnhealthyEvery, benchmarkDefaultUnhealthyEvery),
		sampleEvery:    benchmarkIntEnv(b, benchmarkEnvSampleEvery, benchmarkDefaultSampleEvery),
		flapIntervals:  benchmarkFlapIntervalsEnv(b),
	}
	if options.unhealthyEvery > options.instances {
		b.Fatalf("%s must be less than or equal to %s", benchmarkEnvUnhealthyEvery, benchmarkEnvInstances)
	}
	if options.unhealthyEvery < 2 {
		b.Fatalf("%s must be greater than or equal to 2", benchmarkEnvUnhealthyEvery)
	}

	return options
}

// benchmarkIntEnv 양의 정수 환경 변수를 읽고 비어 있으면 기본값을
// 사용합니다.
func benchmarkIntEnv(b *testing.B, name string, fallback int) int {
	b.Helper()

	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		b.Fatalf("parse %s: %v", name, err)
	}
	if value <= 0 {
		b.Fatalf("%s must be positive", name)
	}

	return value
}

// benchmarkFlapIntervalsEnv 쉼표로 구분한 health flapping 주기 목록을
// 읽습니다.
func benchmarkFlapIntervalsEnv(b *testing.B) []time.Duration {
	b.Helper()

	raw := strings.TrimSpace(os.Getenv(benchmarkEnvFlapIntervals))
	if raw == "" {
		raw = benchmarkDefaultFlapIntervalList
	}

	parts := strings.Split(raw, ",")
	intervals := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || trimmed == "0" {
			intervals = append(intervals, 0)
			continue
		}

		interval, err := time.ParseDuration(trimmed)
		if err != nil {
			b.Fatalf("parse %s: %v", benchmarkEnvFlapIntervals, err)
		}
		if interval < 0 {
			b.Fatalf("%s must not contain a negative duration", benchmarkEnvFlapIntervals)
		}
		intervals = append(intervals, interval)
	}

	return intervals
}

// benchmarkName provider 이름과 flapping 주기를 조합해 go test 출력용
// benchmark 이름을 만듭니다.
func benchmarkName(providerName string, flapInterval time.Duration) string {
	if flapInterval <= 0 {
		return providerName + "/read_only"
	}

	return providerName + "/flapping_" + strings.ReplaceAll(flapInterval.String(), "/", "_")
}
