package harness

import (
	"net"
	"net/url"
	"testing"

	authconfig "wintergate/internal/auth/config"
	internalconfig "wintergate/internal/config"
	"wintergate/internal/pool"
	routeconfig "wintergate/internal/route/config"
)

// Runtime 통합 테스트에서 설정 등록 결과를 공유하는 런타임 저장소 묶음입니다.
type Runtime struct {
	Manager      *internalconfig.Manager
	AuthStore    *authconfig.Store
	Router       *routeconfig.Router
	LoadBalancer *routeconfig.LoadBalancer
	PoolStore    *pool.Store
}

// ServiceOption 테스트 서비스 설정을 조정합니다.
type ServiceOption func(*internalconfig.Settings)

// NewRuntime 설정 Manager와 주요 런타임 저장소를 함께 생성합니다.
func NewRuntime() *Runtime {
	manager := internalconfig.NewManager()
	authStore := authconfig.NewStore()
	router := routeconfig.NewRouter()
	loadBalancer := routeconfig.NewLoadBalancer()
	poolStore := pool.NewStore()

	manager.AddValidator(routeconfig.NewValidator())
	manager.AddValidator(authStore)
	manager.AddValidator(poolStore)

	return &Runtime{
		Manager:      manager,
		AuthStore:    authStore,
		Router:       router,
		LoadBalancer: loadBalancer,
		PoolStore:    poolStore,
	}
}

// Register 설정을 Manager에 등록하고 실패 시 테스트를 중단합니다.
func (r *Runtime) Register(t testing.TB, settings internalconfig.Settings) {
	t.Helper()

	if r == nil || r.Manager == nil {
		t.Fatal("runtime manager is nil")
	}

	if err := r.Manager.Register(settings); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
}

// ServiceSettings 통합 테스트용 서비스 설정을 생성합니다.
func ServiceSettings(serviceName string, instance internalconfig.InstanceSettings, endpoints []internalconfig.EndpointSettings, opts ...ServiceOption) internalconfig.Settings {
	settings := internalconfig.Settings{
		ServiceName: serviceName,
		Global: &internalconfig.GlobalSettings{
			Auth: &internalconfig.AuthSettings{
				JWTAlgorithm: "HS256",
				JWTAudience:  "wintergate",
				JWTClockSkew: "1m",
				JWTIssuer:    "auth-service",
				JWTSecret:    "shared-secret",
			},
		},
		Instance:  &instance,
		Endpoints: append([]internalconfig.EndpointSettings(nil), endpoints...),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&settings)
		}
	}

	return settings
}

// InstanceFromURL httptest 서버 URL에서 서비스 인스턴스 설정을 생성합니다.
func InstanceFromURL(t testing.TB, rawURL string) internalconfig.InstanceSettings {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	host, port, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		t.Fatalf("SplitHostPort returned error: %v", err)
	}

	return internalconfig.InstanceSettings{
		Scheme: parsedURL.Scheme,
		Host:   host,
		Port:   port,
	}
}

// WithAuthSettings 인증 설정을 테스트 서비스 설정에 반영합니다.
func WithAuthSettings(auth *internalconfig.AuthSettings) ServiceOption {
	return func(settings *internalconfig.Settings) {
		settings.Global = &internalconfig.GlobalSettings{
			Auth: auth,
		}
	}
}

// WithPoolThresholds pool 티어 승격 기준을 테스트 서비스 설정에 반영합니다.
func WithPoolThresholds(normal, hot, super internalconfig.ThresholdPoint) ServiceOption {
	return func(settings *internalconfig.Settings) {
		settings.Threshold = &internalconfig.ThresholdSettings{
			Normal: normal,
			Hot:    hot,
			Super:  super,
		}
	}
}
