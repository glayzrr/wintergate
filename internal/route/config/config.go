package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"sidecargo/internal/utils"
)

// EnvConfig 환경 파일에서 읽은 라우팅 설정을 보관합니다.
type EnvConfig struct {
	RouteServiceHeader          string
	RouteTableURL               string
	RouteTableRequestTimeout    time.Duration
	RouteTableRefreshInterval   time.Duration
	RouteUpstreamRequestTimeout time.Duration
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

	routeTableURL, err := utils.RequireString(envRouteTableURL, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	routeTableRequestTimeout, err := utils.RequireDuration(envRouteTableRequestTimeout, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	routeTableRefreshInterval, err := utils.RequireDuration(envRouteTableRefreshInterval, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	routeUpstreamRequestTimeout, err := utils.RequireDuration(envRouteUpstreamRequestTimeout, ErrInvalidConfig)
	if err != nil {
		return EnvConfig{}, err
	}

	return EnvConfig{
		RouteServiceHeader:          routeServiceHeader,
		RouteTableURL:               routeTableURL,
		RouteTableRequestTimeout:    routeTableRequestTimeout,
		RouteTableRefreshInterval:   routeTableRefreshInterval,
		RouteUpstreamRequestTimeout: routeUpstreamRequestTimeout,
	}, nil
}
