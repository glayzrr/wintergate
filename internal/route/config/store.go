package config

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	internalconfig "wintergate/internal/config"
	"wintergate/internal/utils"
)

// Store 서비스 설정, 인스턴스 목록, 라우팅 기준을 메모리에 보관합니다.
type Store struct {
	services       map[string]serviceInfo
	routes         map[routeKey]RouteInfo
	wildcardRoutes []RouteInfo

	// mu는 services와 routes map의 동시 조회와 교체를 보호합니다.
	mu sync.RWMutex
}

type serviceInfo struct {
	serviceName string
	global      *internalconfig.GlobalSettings
	threshold   *internalconfig.ThresholdSettings
	endpoints   []internalconfig.EndpointSettings
	instances   []internalconfig.InstanceSettings

	// cursor는 서비스 인스턴스 라운드로빈 선택 위치를 원자적으로 증가시킵니다.
	cursor *atomic.Uint64
}

type routeKey struct {
	method string
	path   string
}

// NewStore 빈 라우팅 설정 Store를 생성합니다.
func NewStore() *Store {
	return &Store{
		services: make(map[string]serviceInfo),
		routes:   make(map[routeKey]RouteInfo),
	}
}

// Apply 전달받은 서비스 설정, 현재 인스턴스 주소, 라우팅 기준을 저장소에 반영합니다.
func (s *Store) Apply(settings internalconfig.Settings) error {
	if s == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidConfig)
	}

	serviceName := utils.NormalizeServiceName(settings.ServiceName)
	if serviceName == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidConfig)
	}
	if settings.Global == nil {
		return fmt.Errorf("%w: global settings is required", ErrInvalidConfig)
	}
	if len(settings.Endpoints) == 0 {
		return fmt.Errorf("%w: endpoints are required", ErrInvalidConfig)
	}

	if settings.Instance == nil {
		return fmt.Errorf("%w: instance is required", ErrInvalidConfig)
	}

	normalizedHost, normalizedPort, err := utils.NormalizeHostPort(settings.Instance.Host, settings.Instance.Port)
	if err != nil {
		return fmt.Errorf("%w: instance address: %w", ErrInvalidConfig, err)
	}
	instance := internalconfig.InstanceSettings{
		Scheme: utils.NormalizeLower(settings.Instance.Scheme),
		Host:   normalizedHost,
		Port:   normalizedPort,
	}

	endpoints, routeInfos, err := routeInfosFromEndpointSettings(serviceName, settings.Endpoints)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, routeInfo := range routeInfos {
		key := routeKey{method: routeInfo.HttpMethod, path: routeInfo.Path}
		existingRoute, found := s.routes[key]
		if found && existingRoute.ServiceName != serviceName {
			return fmt.Errorf(
				"%w: route %s %s already belongs to service %q",
				ErrInvalidConfig,
				routeInfo.HttpMethod,
				routeInfo.Path,
				existingRoute.ServiceName,
			)
		}
	}

	record, found := s.services[serviceName]
	if !found {
		record = serviceInfo{
			serviceName: serviceName,
			cursor:      &atomic.Uint64{},
		}
	} else if record.cursor == nil {
		return fmt.Errorf("%w: service cursor is nil", ErrInvalidConfig)
	}

	record.serviceName = serviceName
	record.global = cloneGlobalSettings(settings.Global)
	record.threshold = cloneThresholdSettings(settings.Threshold)
	record.endpoints = endpoints
	record.instances = upsertInstance(record.instances, instance)

	s.services[serviceName] = record
	s.replaceServiceRoutes(serviceName, routeInfos)

	return nil
}

// RouteFor 지정한 HTTP method와 path에 연결된 라우팅 정보를 반환합니다.
func (s *Store) RouteFor(method, path string) (RouteInfo, bool) {
	if s == nil {
		return RouteInfo{}, false
	}

	key := routeKey{
		method: utils.NormalizeHTTPMethod(method),
		path:   utils.NormalizeHTTPPath(path),
	}
	if key.method == "" || key.path == "" {
		return RouteInfo{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if routeInfo, found := s.routes[key]; found && !isWildcardPath(routeInfo.Path) {
		return cloneRouteInfo(routeInfo), true
	}
	if routeInfo, found := s.routes[routeKey{method: "ALL", path: key.path}]; found && !isWildcardPath(routeInfo.Path) {
		return cloneRouteInfo(routeInfo), true
	}
	for _, routeInfo := range s.wildcardRoutes {
		if matchRoute(routeInfo, key.method, key.path) {
			return cloneRouteInfo(routeInfo), true
		}
	}

	return RouteInfo{}, false
}

// ServiceFor 지정한 서비스 이름에 대응하는 설정 스냅샷을 반환합니다.
func (s *Store) ServiceFor(serviceName string) (internalconfig.ServiceSettings, bool) {
	if s == nil {
		return internalconfig.ServiceSettings{}, false
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return internalconfig.ServiceSettings{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, found := s.services[normalizedServiceName]
	if !found {
		return internalconfig.ServiceSettings{}, false
	}

	return cloneServiceSettings(record), true
}

// NextInstance 지정한 서비스의 다음 인스턴스를 라운드로빈 순서로 반환합니다.
func (s *Store) NextInstance(serviceName string) (internalconfig.InstanceSettings, error) {
	if s == nil {
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: store is nil", ErrInvalidConfig)
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: service-name is required", ErrInvalidConfig)
	}

	s.mu.RLock()
	record, found := s.services[normalizedServiceName]
	if !found {
		s.mu.RUnlock()
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: %s", ErrConfigNotFound, normalizedServiceName)
	}
	if len(record.instances) == 0 {
		s.mu.RUnlock()
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: service %q has no instances", ErrInvalidConfig, normalizedServiceName)
	}
	if record.cursor == nil {
		s.mu.RUnlock()
		return internalconfig.InstanceSettings{}, fmt.Errorf("%w: service cursor is nil", ErrInvalidConfig)
	}
	instances := append([]internalconfig.InstanceSettings(nil), record.instances...)
	cursor := record.cursor
	s.mu.RUnlock()

	index := cursor.Add(1) - 1

	return instances[index%uint64(len(instances))], nil
}

func (s *Store) replaceServiceRoutes(serviceName string, routeInfos []RouteInfo) {
	for key, routeInfo := range s.routes {
		if routeInfo.ServiceName == serviceName {
			delete(s.routes, key)
		}
	}

	filteredWildcardRoutes := s.wildcardRoutes[:0]
	for _, routeInfo := range s.wildcardRoutes {
		if routeInfo.ServiceName != serviceName {
			filteredWildcardRoutes = append(filteredWildcardRoutes, routeInfo)
		}
	}
	s.wildcardRoutes = filteredWildcardRoutes

	for _, routeInfo := range routeInfos {
		s.routes[routeKey{method: routeInfo.HttpMethod, path: routeInfo.Path}] = routeInfo
		if isWildcardPath(routeInfo.Path) {
			s.wildcardRoutes = append(s.wildcardRoutes, routeInfo)
		}
	}
}

func routeInfosFromEndpointSettings(serviceName string, endpoints []internalconfig.EndpointSettings) ([]internalconfig.EndpointSettings, []RouteInfo, error) {
	normalizedEndpoints := make([]internalconfig.EndpointSettings, 0, len(endpoints))
	routeInfos := make([]RouteInfo, 0, len(endpoints))
	seen := make(map[routeKey]struct{}, len(endpoints))

	for _, endpoint := range endpoints {
		path := utils.NormalizeHTTPPath(endpoint.Path)
		if path == "" {
			return nil, nil, fmt.Errorf("%w: path is required", ErrInvalidConfig)
		}

		method := utils.NormalizeHTTPMethod(endpoint.Method)
		if method == "" {
			return nil, nil, fmt.Errorf("%w: http method is required", ErrInvalidConfig)
		}

		roles, ok := utils.NormalizeRoles(endpoint.Roles)
		if !ok {
			return nil, nil, fmt.Errorf("%w: role is required", ErrInvalidConfig)
		}

		key := routeKey{method: method, path: path}
		if _, found := seen[key]; found {
			return nil, nil, fmt.Errorf("%w: duplicate route %s %s", ErrInvalidConfig, method, path)
		}
		seen[key] = struct{}{}

		normalizedEndpoints = append(normalizedEndpoints, internalconfig.EndpointSettings{
			Path:   path,
			Method: method,
			Roles:  roles,
		})
		routeInfos = append(routeInfos, RouteInfo{
			ServiceName: serviceName,
			Path:        path,
			HttpMethod:  method,
			Roles:       append([]string(nil), roles...),
		})
	}

	return normalizedEndpoints, routeInfos, nil
}

func upsertInstance(instances []internalconfig.InstanceSettings, next internalconfig.InstanceSettings) []internalconfig.InstanceSettings {
	for index, instance := range instances {
		if instance.Host == next.Host && instance.Port == next.Port {
			instances[index] = next
			return instances
		}
	}

	return append(instances, next)
}

func matchRoute(routeInfo RouteInfo, method, path string) bool {
	if routeInfo.HttpMethod != "ALL" && routeInfo.HttpMethod != method {
		return false
	}

	routePath := strings.TrimSpace(routeInfo.Path)
	if isWildcardPath(routePath) {
		return strings.HasPrefix(path, strings.TrimSuffix(routePath, "/**"))
	}

	return routePath == path
}

func isWildcardPath(path string) bool {
	return strings.HasSuffix(strings.TrimSpace(path), "/**")
}

func cloneRouteInfo(routeInfo RouteInfo) RouteInfo {
	return RouteInfo{
		ServiceName: routeInfo.ServiceName,
		Path:        routeInfo.Path,
		HttpMethod:  routeInfo.HttpMethod,
		Roles:       append([]string(nil), routeInfo.Roles...),
	}
}

func cloneServiceSettings(record serviceInfo) internalconfig.ServiceSettings {
	return internalconfig.ServiceSettings{
		ServiceName: record.serviceName,
		Global:      cloneGlobalSettings(record.global),
		Threshold:   cloneThresholdSettings(record.threshold),
		Endpoints:   cloneEndpointSettings(record.endpoints),
		Instances:   append([]internalconfig.InstanceSettings(nil), record.instances...),
	}
}

func cloneGlobalSettings(settings *internalconfig.GlobalSettings) *internalconfig.GlobalSettings {
	if settings == nil {
		return nil
	}

	return &internalconfig.GlobalSettings{
		Auth: cloneAuthSettings(settings.Auth),
	}
}

func cloneAuthSettings(settings *internalconfig.AuthSettings) *internalconfig.AuthSettings {
	if settings == nil {
		return nil
	}

	return &internalconfig.AuthSettings{
		JWTAlgorithm: settings.JWTAlgorithm,
		JWTAudience:  settings.JWTAudience,
		JWTClockSkew: settings.JWTClockSkew,
		JWTIssuer:    settings.JWTIssuer,
		JWTSecret:    settings.JWTSecret,
		JWKS:         append([]byte(nil), settings.JWKS...),
	}
}

func cloneThresholdSettings(settings *internalconfig.ThresholdSettings) *internalconfig.ThresholdSettings {
	if settings == nil {
		return nil
	}

	return &internalconfig.ThresholdSettings{
		Hot:   settings.Hot,
		Super: settings.Super,
	}
}

func cloneEndpointSettings(endpoints []internalconfig.EndpointSettings) []internalconfig.EndpointSettings {
	if len(endpoints) == 0 {
		return nil
	}

	clonedEndpoints := make([]internalconfig.EndpointSettings, 0, len(endpoints))
	for _, endpoint := range endpoints {
		clonedEndpoints = append(clonedEndpoints, internalconfig.EndpointSettings{
			Path:   endpoint.Path,
			Method: endpoint.Method,
			Roles:  append([]string(nil), endpoint.Roles...),
		})
	}

	return clonedEndpoints
}
