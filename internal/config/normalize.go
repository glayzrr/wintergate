package config

import (
	"fmt"
	"strings"

	"wintergate/internal/utils"
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

	normalizedScheme := strings.ToLower(strings.TrimSpace(settings.Instance.Scheme))
	if normalizedScheme != "http" && normalizedScheme != "https" {
		return "", Settings{}, fmt.Errorf("%w: instance scheme is required", ErrInvalidSettings)
	}

	normalizedHost, normalizedPort, err := utils.NormalizeHostPort(settings.Instance.Host, settings.Instance.Port)
	if err != nil {
		return "", Settings{}, fmt.Errorf("%w: config address: %w", ErrInvalidSettings, err)
	}

	settings.ServiceName = serviceName
	settings.Instance = &InstanceSettings{
		Scheme: normalizedScheme,
		Host:   normalizedHost,
		Port:   normalizedPort,
	}

	return serviceName, settings, nil
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
