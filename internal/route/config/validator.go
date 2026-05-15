package config

import (
	"fmt"

	internalconfig "wintergate/internal/config"
	"wintergate/internal/utils"
)

// Validator Validator는 후보 snapshot의 route index가 서비스 설정과 일관되는지 검증합니다.
type Validator struct{}

// NewValidator route snapshot validator를 생성합니다.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate 후보 스냅샷의 전체 라우팅 설정이 반영 가능한지 검증합니다.
func (v *Validator) Validate(candidate internalconfig.Snapshot) error {
	if v == nil {
		return fmt.Errorf("%w: route validator is nil", ErrInvalidConfig)
	}

	// 서비스 원본 설정이 비어 있으면 route index가 있어도 안전하게 해석할 수 없습니다.
	for serviceName, service := range candidate.Services {
		if utils.NormalizeServiceName(serviceName) == "" || service.ServiceName == "" {
			return fmt.Errorf("%w: service-name is required", ErrInvalidConfig)
		}
		if len(service.Endpoints) == 0 {
			return fmt.Errorf("%w: endpoints are required", ErrInvalidConfig)
		}
	}

	// exact route index는 반드시 존재하는 서비스만 가리켜야 commit 후 라우팅 장애가 나지 않습니다.
	for key, route := range candidate.Routes {
		if key.Method == "" || key.Path == "" || route.Method == "" || route.Path == "" {
			return fmt.Errorf("%w: route method and path are required", ErrInvalidConfig)
		}
		if _, found := candidate.Services[route.ServiceName]; !found {
			return fmt.Errorf("%w: route %s %s references unknown service %q", ErrInvalidConfig, route.Method, route.Path, route.ServiceName)
		}
	}

	// wildcard index는 exact route와 별도로 순회되므로 wildcard 형태와 서비스 참조를 다시 확인합니다.
	for _, route := range candidate.WildcardRoutes {
		if !isWildcardPath(route.Path) {
			return fmt.Errorf("%w: wildcard route %s %s is not wildcard", ErrInvalidConfig, route.Method, route.Path)
		}
		if _, found := candidate.Services[route.ServiceName]; !found {
			return fmt.Errorf("%w: wildcard route %s %s references unknown service %q", ErrInvalidConfig, route.Method, route.Path, route.ServiceName)
		}
	}

	return nil
}
