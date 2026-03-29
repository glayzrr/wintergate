package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"sidecargo/internal/utils"
)

// EnvConfig 환경 파일에서 읽은 라우팅 설정을 보관합니다.
type EnvConfig struct {
	RouteServiceHeader         string
	RouteUpstreamRequestTimeout time.Duration
	RouteUpstreams             map[string]string
}

// LoadEnvConfig 지정한 환경 파일에서 라우팅 설정을 읽어옵니다.
func LoadEnvConfig(path string) (EnvConfig, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultEnvPath
	}

	if err := godotenv.Load(path); err != nil {
		return EnvConfig{}, fmt.Errorf("%w: load %s: %v", ErrInvalidConfig, path, err)
	}

	routeServiceHeader, err := utils.RequireString(envRouteServiceHeader, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	routeUpstreamRequestTimeout, err := utils.RequireDuration(envRouteUpstreamRequestTimeout, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	routeUpstreams, err := requireUpstreams(envRouteUpstreams)
	if err != nil {
		return EnvConfig{}, err
	}

	return EnvConfig{
		RouteServiceHeader:          routeServiceHeader,
		RouteUpstreamRequestTimeout: routeUpstreamRequestTimeout,
		RouteUpstreams:              routeUpstreams,
	}, nil
}

func requireUpstreams(key string) (map[string]string, error) {
	rawValue, err := utils.RequireString(key, ErrInvalidConfig)
	if err != nil {
		return nil, err
	}

	var upstreams map[string]string
	if err := json.Unmarshal([]byte(rawValue), &upstreams); err != nil {
		return nil, fmt.Errorf("%w: invalid json for %s: %v", ErrInvalidConfig, key, err)
	}

	if len(upstreams) == 0 {
		return nil, fmt.Errorf("%w: %s is required", ErrInvalidConfig, key)
	}

	validatedUpstreams := make(map[string]string, len(upstreams))
	serviceNames := make([]string, 0, len(upstreams))
	for serviceName := range upstreams {
		serviceNames = append(serviceNames, serviceName)
	}

	sort.Strings(serviceNames)
	for _, serviceName := range serviceNames {
		trimmedServiceName := strings.TrimSpace(serviceName)
		if trimmedServiceName == "" {
			return nil, fmt.Errorf("%w: service name in %s must not be empty", ErrInvalidConfig, key)
		}

		rawURL := strings.TrimSpace(upstreams[serviceName])
		if rawURL == "" {
			return nil, fmt.Errorf("%w: upstream url for %q in %s is required", ErrInvalidConfig, trimmedServiceName, key)
		}

		parsedURL, err := url.Parse(rawURL)
		if err != nil {
			return nil, fmt.Errorf("%w: parse upstream url for %q in %s: %v", ErrInvalidConfig, trimmedServiceName, key, err)
		}

		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			return nil, fmt.Errorf("%w: upstream url for %q in %s must include scheme and host", ErrInvalidConfig, trimmedServiceName, key)
		}

		validatedUpstreams[trimmedServiceName] = rawURL
	}

	return validatedUpstreams, nil
}
