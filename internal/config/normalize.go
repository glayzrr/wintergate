package config

import (
	"fmt"
	"strings"
	"time"

	"wintergate/internal/utils"
)

const (
	defaultHealthPath             = "/actuator/health"
	defaultHealthInterval         = 5 * time.Second
	defaultHealthTimeout          = 2 * time.Second
	defaultHealthJitter           = 2 * time.Second
	defaultHealthMaxBackoff       = 5 * time.Minute
	defaultHealthFailureThreshold = 2
	defaultHealthSuccessThreshold = 1
)

func normalizeSettings(settings Settings) (string, Settings, error) {
	serviceName := normalizeServiceName(settings.ServiceName)
	if serviceName == "" {
		return "", Settings{}, fmt.Errorf("%w: service-name is required", ErrInvalidSettings)
	}
	if settings.Global == nil {
		return "", Settings{}, fmt.Errorf("%w: global settings is required", ErrInvalidSettings)
	}
	if len(settings.Endpoints) == 0 {
		return "", Settings{}, fmt.Errorf("%w: endpoints are required", ErrInvalidSettings)
	}
	if settings.Instance == nil {
		return "", Settings{}, fmt.Errorf("%w: instance is required", ErrInvalidSettings)
	}

	normalizedInstance, err := normalizeInstanceSettings(serviceName, *settings.Instance)
	if err != nil {
		return "", Settings{}, err
	}

	settings.ServiceName = serviceName
	settings.Instance = &normalizedInstance
	normalizedHealthSettings, err := normalizeHealthSettings(settings.Health)
	if err != nil {
		return "", Settings{}, err
	}
	settings.Health = normalizedHealthSettings

	return serviceName, settings, nil
}

func normalizeInstanceSettings(serviceName string, instance InstanceSettings) (InstanceSettings, error) {
	normalizedServiceName := normalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return InstanceSettings{}, fmt.Errorf("%w: service-name is required", ErrInvalidSettings)
	}

	normalizedScheme := strings.ToLower(strings.TrimSpace(instance.Scheme))
	if normalizedScheme != "http" && normalizedScheme != "https" {
		return InstanceSettings{}, fmt.Errorf("%w: instance scheme is required", ErrInvalidSettings)
	}

	normalizedHost, normalizedPort, err := utils.NormalizeHostPort(instance.Host, instance.Port)
	if err != nil {
		return InstanceSettings{}, fmt.Errorf("%w: config address: %w", ErrInvalidSettings, err)
	}

	return InstanceSettings{
		Scheme:    normalizedScheme,
		Host:      normalizedHost,
		Port:      normalizedPort,
		HealthKey: healthKeyFor(normalizedServiceName, normalizedScheme, normalizedHost, normalizedPort),
	}, nil
}

func healthKeyFor(serviceName, scheme, host, port string) string {
	return serviceName + "|" + scheme + "|" + host + "|" + port
}

func normalizeHealthSettings(settings *HealthSettings) (*HealthSettings, error) {
	if settings == nil {
		return DefaultHealthSettings(), nil
	}

	normalizedSettings := settings.Clone()
	if normalizedSettings.Enabled == nil {
		enabled := true
		normalizedSettings.Enabled = &enabled
	}

	path := strings.TrimSpace(normalizedSettings.Path)
	if path == "" {
		path = defaultHealthPath
	}
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("%w: health path must start with /", ErrInvalidSettings)
	}
	normalizedSettings.Path = path

	interval, err := normalizePositiveDuration(normalizedSettings.Interval, defaultHealthInterval, "health interval")
	if err != nil {
		return nil, err
	}
	timeout, err := normalizePositiveDuration(normalizedSettings.Timeout, defaultHealthTimeout, "health timeout")
	if err != nil {
		return nil, err
	}
	jitter, err := normalizeNonNegativeDuration(normalizedSettings.Jitter, defaultHealthJitter, "health jitter")
	if err != nil {
		return nil, err
	}
	maxBackoff, err := normalizePositiveDuration(normalizedSettings.MaxBackoff, defaultHealthMaxBackoff, "health max backoff")
	if err != nil {
		return nil, err
	}
	if maxBackoff < interval {
		return nil, fmt.Errorf("%w: health max backoff must be greater than or equal to interval", ErrInvalidSettings)
	}

	normalizedSettings.Interval = interval.String()
	normalizedSettings.Timeout = timeout.String()
	normalizedSettings.Jitter = jitter.String()
	normalizedSettings.MaxBackoff = maxBackoff.String()

	if normalizedSettings.FailureThreshold < 0 {
		return nil, fmt.Errorf("%w: health failure threshold must be greater than or equal to zero", ErrInvalidSettings)
	}
	if normalizedSettings.FailureThreshold == 0 {
		normalizedSettings.FailureThreshold = defaultHealthFailureThreshold
	}
	if normalizedSettings.SuccessThreshold < 0 {
		return nil, fmt.Errorf("%w: health success threshold must be greater than or equal to zero", ErrInvalidSettings)
	}
	if normalizedSettings.SuccessThreshold == 0 {
		normalizedSettings.SuccessThreshold = defaultHealthSuccessThreshold
	}

	return normalizedSettings, nil
}

// DefaultHealthSettings 기본 헬스 체크 설정을 반환합니다.
func DefaultHealthSettings() *HealthSettings {
	enabled := true

	return &HealthSettings{
		Enabled:          &enabled,
		Path:             defaultHealthPath,
		Interval:         defaultHealthInterval.String(),
		Timeout:          defaultHealthTimeout.String(),
		Jitter:           defaultHealthJitter.String(),
		MaxBackoff:        defaultHealthMaxBackoff.String(),
		FailureThreshold: defaultHealthFailureThreshold,
		SuccessThreshold: defaultHealthSuccessThreshold,
	}
}

func normalizePositiveDuration(value string, defaultValue time.Duration, fieldName string) (time.Duration, error) {
	duration, err := normalizeNonNegativeDuration(value, defaultValue, fieldName)
	if err != nil {
		return 0, err
	}
	if duration == 0 {
		return 0, fmt.Errorf("%w: %s must be greater than zero", ErrInvalidSettings, fieldName)
	}

	return duration, nil
}

func normalizeNonNegativeDuration(value string, defaultValue time.Duration, fieldName string) (time.Duration, error) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return defaultValue, nil
	}

	duration, err := time.ParseDuration(trimmedValue)
	if err != nil {
		return 0, fmt.Errorf("%w: parse %s: %w", ErrInvalidSettings, fieldName, err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("%w: %s must be greater than or equal to zero", ErrInvalidSettings, fieldName)
	}

	return duration, nil
}

func routeEntriesFromEndpointSettings(serviceName string, endpoints []EndpointSettings) ([]EndpointSettings, []RouteEntry, error) {
	normalizedEndpoints := make([]EndpointSettings, 0, len(endpoints))
	routeEntries := make([]RouteEntry, 0, len(endpoints))
	seen := make(map[RouteKey]struct{}, len(endpoints))

	for _, endpoint := range endpoints {
		path := utils.NormalizeHTTPPath(endpoint.Path)
		if path == "" {
			return nil, nil, fmt.Errorf("%w: path is required", ErrInvalidSettings)
		}

		method := utils.NormalizeHTTPMethod(endpoint.Method)
		if method == "" {
			return nil, nil, fmt.Errorf("%w: http method is required", ErrInvalidSettings)
		}

		roles, ok := utils.NormalizeRoles(endpoint.Roles)
		if !ok {
			return nil, nil, fmt.Errorf("%w: role is required", ErrInvalidSettings)
		}

		key := RouteKey{Method: method, Path: path}
		if _, found := seen[key]; found {
			return nil, nil, fmt.Errorf("%w: duplicate route %s %s", ErrInvalidSettings, method, path)
		}
		seen[key] = struct{}{}

		normalizedEndpoints = append(normalizedEndpoints, EndpointSettings{
			Path:   path,
			Method: method,
			Roles:  roles,
		})
		routeEntries = append(routeEntries, RouteEntry{
			ServiceName: serviceName,
			Path:        path,
			Method:      method,
			Roles:       append([]string(nil), roles...),
		})
	}

	return normalizedEndpoints, routeEntries, nil
}

func normalizeServiceName(serviceName string) string {
	return utils.NormalizeServiceName(serviceName)
}

func isWildcardPath(path string) bool {
	return strings.HasSuffix(strings.TrimSpace(path), "/**")
}
