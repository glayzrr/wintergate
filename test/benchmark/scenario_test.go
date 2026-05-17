package benchmark

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	internalconfig "wintergate/internal/config"
)

// benchmarkSnapshot 네트워크를 타지 않고 LoadBalancer만 측정할 수 있는
// 더미 서비스 snapshot을 만듭니다.
func benchmarkSnapshot(instanceCount int) (*internalconfig.Snapshot, []string) {
	instances := make([]internalconfig.InstanceSettings, 0, instanceCount)
	healthKeys := make([]string, 0, instanceCount)
	for index := 0; index < instanceCount; index++ {
		healthKey := fmt.Sprintf("%s|http|127.0.0.1|%d", benchmarkServiceName, 10_000+index)
		instances = append(instances, internalconfig.InstanceSettings{
			Scheme:    "http",
			Host:      "127.0.0.1",
			Port:      strconv.Itoa(10_000 + index),
			HealthKey: healthKey,
		})
		healthKeys = append(healthKeys, healthKey)
	}

	return &internalconfig.Snapshot{
		Services: map[string]internalconfig.ServiceSettings{
			benchmarkServiceName: {
				ServiceName: benchmarkServiceName,
				Instances:   instances,
			},
		},
		Routes: make(map[internalconfig.RouteKey]internalconfig.RouteEntry),
	}, healthKeys
}

// startBenchmarkHealthFlapping 주어진 주기마다 일부 인스턴스의
// routable 상태를 뒤집는 writer 부하를 만듭니다.
func startBenchmarkHealthFlapping(provider benchmarkHealthProvider, healthKeys []string, interval time.Duration, unhealthyEvery int) func() uint64 {
	if interval <= 0 {
		// interval이 0이면 read-only 기준선으로 사용합니다.
		return func() uint64 {
			return 0
		}
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	var flaps atomic.Uint64
	go func() {
		defer close(done)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var shift int
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				shift++
				for index, healthKey := range healthKeys {
					// shift를 증가시켜 매 tick마다 unhealthy 대상이 이동하게
					// 만들고, 항상 일부 healthy 인스턴스는 남깁니다.
					routable := (index+shift)%unhealthyEvery != 0
					provider.setRoutable(healthKey, routable)
				}
				flaps.Add(1)
			}
		}
	}()

	return func() uint64 {
		close(stop)
		<-done

		return flaps.Load()
	}
}
