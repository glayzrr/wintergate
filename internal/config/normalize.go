package config

import (
	"fmt"
	"net"
	"strings"

	"wintergate/internal/utils"
)

func normalizeServiceName(serviceName string) string {
	return strings.ToLower(strings.TrimSpace(serviceName))
}

func normalizeInstanceSettings(instance InstanceSettings) (InstanceSettings, error) {
	instanceKey, err := utils.ConfigKey(instance.Host, instance.Port)
	if err != nil {
		return InstanceSettings{}, err
	}

	host, port, err := net.SplitHostPort(instanceKey)
	if err != nil {
		return InstanceSettings{}, fmt.Errorf("split instance address: %w", err)
	}

	return InstanceSettings{
		Host: host,
		Port: port,
	}, nil
}

func normalizeEndpointSettings(serviceName string, endpoints []EndpointSettings) ([]EndpointSettings, []RouteBindingSettings, error) {
	normalizedEndpoints := make([]EndpointSettings, 0, len(endpoints))
	bindings := make([]RouteBindingSettings, 0, len(endpoints))
	seen := make(map[routeKey]struct{}, len(endpoints))

	for _, endpoint := range endpoints {
		path := normalizeHTTPPath(endpoint.Path)
		if path == "" {
			return nil, nil, fmt.Errorf("%w: path is required", ErrInvalidSettings)
		}

		method := normalizeHTTPMethod(endpoint.Method)
		if method == "" {
			return nil, nil, fmt.Errorf("%w: http method is required", ErrInvalidSettings)
		}

		roles, ok := normalizeRoles(endpoint.Roles)
		if !ok {
			return nil, nil, fmt.Errorf("%w: role is required", ErrInvalidSettings)
		}

		key := routeKey{method: method, path: path}
		if _, found := seen[key]; found {
			return nil, nil, fmt.Errorf("%w: duplicate route %s %s", ErrInvalidSettings, method, path)
		}
		seen[key] = struct{}{}

		normalizedEndpoint := EndpointSettings{
			Path:   path,
			Method: method,
			Roles:  roles,
		}
		normalizedEndpoints = append(normalizedEndpoints, normalizedEndpoint)
		bindings = append(bindings, RouteBindingSettings{
			ServiceName: serviceName,
			Path:        path,
			Method:      method,
			Roles:       append([]string(nil), roles...),
		})
	}

	return normalizedEndpoints, bindings, nil
}

func normalizeHTTPMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

func normalizeHTTPPath(path string) string {
	return strings.TrimSpace(path)
}

func normalizeRoles(roles []string) ([]string, bool) {
	if len(roles) == 0 {
		return nil, true
	}

	normalizedRoles := make([]string, 0, len(roles))
	for _, role := range roles {
		trimmedRole := strings.TrimSpace(role)
		if trimmedRole == "" {
			return nil, false
		}

		normalizedRoles = append(normalizedRoles, trimmedRole)
	}

	return normalizedRoles, true
}
