package config

import (
	"fmt"
	"sync"
	"sync/atomic"

	internalconfig "wintergate/internal/config"
	"wintergate/internal/utils"
)

// LoadBalancer LoadBalancer는 서비스별 인스턴스 선택 cursor만 관리합니다.
type LoadBalancer struct {
	cursors sync.Map
	health  HealthStatusProvider
}

// HealthStatusProvider 인스턴스가 라우팅 후보로 사용할 수 있는 상태인지 알려줍니다.
type HealthStatusProvider interface {
	IsRoutableKey(healthKey string) bool
}

// NewLoadBalancer 인스턴스 선택용 load balancer를 생성합니다.
func NewLoadBalancer(health ...HealthStatusProvider) *LoadBalancer {
	loadBalancer := &LoadBalancer{}
	if len(health) > 0 {
		loadBalancer.health = health[0]
	}

	return loadBalancer
}

// NextInstance 지정한 서비스의 다음 인스턴스를 라운드로빈 순서로 반환합니다.
func (lb *LoadBalancer) NextInstance(snapshot *internalconfig.Snapshot, serviceName string) (internalconfig.InstanceSettings, error) {
	// LoadBalancer는 설정을 보관하지 않으므로 호출자가 고정한 snapshot이 반드시 필요합니다.
	if lb == nil {
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: load balancer is nil", ErrInvalidConfig)
	}
	if snapshot == nil {
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: settings snapshot is required", ErrConfigNotFound)
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: service-name is required", ErrInvalidConfig)
	}

	// 인스턴스 목록은 중앙 snapshot에서만 읽고, 여기서는 선택 위치만 관리합니다.
	service, found := snapshot.Services[normalizedServiceName]
	if !found {
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: %s", ErrConfigNotFound, normalizedServiceName)
	}
	if len(service.Instances) == 0 {
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: service %q has no instances", ErrNoHealthyInstance, normalizedServiceName)
	}
	// cursor는 요청 경로의 mutable runtime state라 snapshot에 넣지 않고 서비스별로 독립 관리합니다.
	value, _ := lb.cursors.LoadOrStore(normalizedServiceName, &atomic.Uint64{})
	cursor, ok := value.(*atomic.Uint64)
	if !ok || cursor == nil {
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: service cursor is invalid", ErrInvalidConfig)
	}

	index := cursor.Add(1) - 1

	// snapshot의 인스턴스 slice는 불변으로 다루고, health 상태는 사전 계산된 key로 lock 없이 조회합니다.
	for offset := uint64(0); offset < uint64(len(service.Instances)); offset++ {
		instance := service.Instances[(index+offset)%uint64(len(service.Instances))]
		if lb.routableInstance(instance) {
			return instance, nil
		}
	}

	return internalconfig.InstanceSettings{}, fmt.Errorf("%w: service %q", ErrNoHealthyInstance, normalizedServiceName)
}

func (lb *LoadBalancer) routableInstance(instance internalconfig.InstanceSettings) bool {
	return lb.health == nil || lb.health.IsRoutableKey(instance.HealthKey)
}
