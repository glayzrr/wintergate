package config

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Store 서비스 설정, 라우팅 기준, 인스턴스 목록을 메모리에 저장합니다.
type Store struct {
	services map[string]serviceRecord
	routes   map[routeKey]RouteBindingSettings

	// mu는 services와 routes map의 동시 조회와 교체를 보호합니다.
	mu sync.RWMutex
}

type serviceRecord struct {
	serviceName string
	global      *GlobalSettings
	threshold   *ThresholdSettings
	endpoints   []EndpointSettings
	instances   []InstanceSettings

	// cursor는 서비스 인스턴스 라운드로빈 선택 위치를 원자적으로 증가시킵니다.
	cursor *atomic.Uint64
}

type routeKey struct {
	method string
	path   string
}

// NewStore 빈 서비스 설정 저장소를 생성합니다.
func NewStore() *Store {
	return &Store{
		services: make(map[string]serviceRecord),
		routes:   make(map[routeKey]RouteBindingSettings),
	}
}

// RegisterService 서비스 설정, 라우팅 기준, 현재 인스턴스 주소를 저장소에 등록합니다.
func (s *Store) RegisterService(settings Settings, instance InstanceSettings) error {
	if s == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidSettings)
	}

	serviceName := normalizeServiceName(settings.ServiceName)
	if serviceName == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidSettings)
	}
	if settings.Global == nil {
		return fmt.Errorf("%w: global settings is required", ErrInvalidSettings)
	}
	if len(settings.Endpoints) == 0 {
		return fmt.Errorf("%w: endpoints are required", ErrInvalidSettings)
	}

	normalizedInstance, err := normalizeInstanceSettings(instance)
	if err != nil {
		return fmt.Errorf("%w: instance address: %w", ErrInvalidSettings, err)
	}

	endpoints, bindings, err := normalizeEndpointSettings(serviceName, settings.Endpoints)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, binding := range bindings {
		key := routeKey{method: binding.Method, path: binding.Path}
		existingBinding, found := s.routes[key]
		if found && existingBinding.ServiceName != serviceName {
			return fmt.Errorf(
				"%w: route %s %s already belongs to service %q",
				ErrInvalidSettings,
				binding.Method,
				binding.Path,
				existingBinding.ServiceName,
			)
		}
	}

	record, found := s.services[serviceName]
	if !found {
		record = serviceRecord{
			serviceName: serviceName,
			cursor:      &atomic.Uint64{},
		}
	} else if record.cursor == nil {
		return fmt.Errorf("%w: service cursor is nil", ErrInvalidSettings)
	}
	record.serviceName = serviceName
	record.global = cloneGlobalSettings(settings.Global)
	record.threshold = cloneThresholdSettings(settings.Threshold)
	record.endpoints = endpoints
	record.instances = upsertInstance(record.instances, normalizedInstance)

	s.services[serviceName] = record
	s.replaceServiceRoutes(serviceName, bindings)

	return nil
}

// ServiceFor 지정한 서비스 이름에 대응하는 설정 스냅샷을 반환합니다.
func (s *Store) ServiceFor(serviceName string) (ServiceSettings, bool) {
	if s == nil {
		return ServiceSettings{}, false
	}

	normalizedServiceName := normalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return ServiceSettings{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, found := s.services[normalizedServiceName]
	if !found {
		return ServiceSettings{}, false
	}

	return cloneServiceSettings(record), true
}

// NextInstance 지정한 서비스의 다음 인스턴스를 라운드로빈 순서로 반환합니다.
func (s *Store) NextInstance(serviceName string) (InstanceSettings, error) {
	if s == nil {
		return InstanceSettings{}, fmt.Errorf("%w: store is nil", ErrInvalidSettings)
	}

	normalizedServiceName := normalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return InstanceSettings{}, fmt.Errorf("%w: service-name is required", ErrInvalidSettings)
	}

	s.mu.RLock()
	record, found := s.services[normalizedServiceName]
	if !found {
		s.mu.RUnlock()
		return InstanceSettings{}, fmt.Errorf("%w: %s", ErrServiceNotFound, normalizedServiceName)
	}
	if len(record.instances) == 0 {
		s.mu.RUnlock()
		return InstanceSettings{}, fmt.Errorf("%w: %s", ErrInstanceNotFound, normalizedServiceName)
	}
	if record.cursor == nil {
		s.mu.RUnlock()
		return InstanceSettings{}, fmt.Errorf("%w: service cursor is nil", ErrInvalidSettings)
	}
	instances := append([]InstanceSettings(nil), record.instances...)
	cursor := record.cursor
	s.mu.RUnlock()

	// 다음 instance를 위해 cursor를 +1한 후 반환될 instance 선택을 위해 -1을 합니다.
	index := cursor.Add(1) - 1

	return instances[index%uint64(len(instances))], nil
}

// RouteFor 지정한 HTTP method와 path에 연결된 서비스 라우팅 정보를 반환합니다.
func (s *Store) RouteFor(method, path string) (RouteBindingSettings, bool) {
	if s == nil {
		return RouteBindingSettings{}, false
	}

	key := routeKey{
		method: normalizeHTTPMethod(method),
		path:   normalizeHTTPPath(path),
	}
	if key.method == "" || key.path == "" {
		return RouteBindingSettings{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	binding, found := s.routes[key]
	if !found {
		return RouteBindingSettings{}, false
	}

	return cloneRouteBindingSettings(binding), true
}

func (s *Store) replaceServiceRoutes(serviceName string, bindings []RouteBindingSettings) {
	for key, binding := range s.routes {
		if binding.ServiceName == serviceName {
			delete(s.routes, key)
		}
	}

	for _, binding := range bindings {
		s.routes[routeKey{method: binding.Method, path: binding.Path}] = binding
	}
}

func cloneServiceSettings(record serviceRecord) ServiceSettings {
	return ServiceSettings{
		ServiceName: record.serviceName,
		Global:      cloneGlobalSettings(record.global),
		Threshold:   cloneThresholdSettings(record.threshold),
		Endpoints:   cloneEndpointSettings(record.endpoints),
		Instances:   append([]InstanceSettings(nil), record.instances...),
	}
}

func upsertInstance(instances []InstanceSettings, next InstanceSettings) []InstanceSettings {
	for index, instance := range instances {
		if instance.Host == next.Host && instance.Port == next.Port {
			instances[index] = next
			return instances
		}
	}

	return append(instances, next)
}

func cloneRouteBindingSettings(binding RouteBindingSettings) RouteBindingSettings {
	return RouteBindingSettings{
		ServiceName: binding.ServiceName,
		Path:        binding.Path,
		Method:      binding.Method,
		Roles:       append([]string(nil), binding.Roles...),
	}
}

func cloneGlobalSettings(settings *GlobalSettings) *GlobalSettings {
	if settings == nil {
		return nil
	}

	return &GlobalSettings{
		Auth: cloneAuthSettings(settings.Auth),
	}
}

func cloneAuthSettings(settings *AuthSettings) *AuthSettings {
	if settings == nil {
		return nil
	}

	return &AuthSettings{
		JWTAlgorithm: settings.JWTAlgorithm,
		JWTAudience:  settings.JWTAudience,
		JWTClockSkew: settings.JWTClockSkew,
		JWTIssuer:    settings.JWTIssuer,
		JWTSecret:    settings.JWTSecret,
		JWKS:         append([]byte(nil), settings.JWKS...),
	}
}

func cloneThresholdSettings(settings *ThresholdSettings) *ThresholdSettings {
	if settings == nil {
		return nil
	}

	return &ThresholdSettings{
		Hot:   settings.Hot,
		Super: settings.Super,
	}
}

func cloneEndpointSettings(endpoints []EndpointSettings) []EndpointSettings {
	if len(endpoints) == 0 {
		return nil
	}

	clonedEndpoints := make([]EndpointSettings, 0, len(endpoints))
	for _, endpoint := range endpoints {
		clonedEndpoints = append(clonedEndpoints, EndpointSettings{
			Path:   endpoint.Path,
			Method: endpoint.Method,
			Roles:  append([]string(nil), endpoint.Roles...),
		})
	}

	return clonedEndpoints
}
