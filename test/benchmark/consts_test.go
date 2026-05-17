package benchmark

const (
	benchmarkServiceName             = "bench-service"
	benchmarkDefaultInstances        = 64
	benchmarkDefaultUnhealthyEvery   = 4
	benchmarkDefaultSampleEvery      = 128
	benchmarkDefaultFlapIntervalList = "0,10ms,1ms"
)

const (
	benchmarkEnvInstances      = "WINTERGATE_BENCH_INSTANCES"
	benchmarkEnvUnhealthyEvery = "WINTERGATE_BENCH_UNHEALTHY_EVERY"
	benchmarkEnvSampleEvery    = "WINTERGATE_BENCH_SAMPLE_EVERY"
	benchmarkEnvFlapIntervals  = "WINTERGATE_BENCH_FLAP_INTERVALS"
)
