package config

import (
	"fmt"
	"strings"
	"sync"

	"wintergate/internal/utils"
)

// Applier 설정 변경을 자신의 런타임 상태에 반영합니다.
type Applier interface {
	Apply(settings Settings) error
}

// Manager 설정 정보를 등록된 Applier들에게 전달합니다.
type Manager struct {
	appliers []Applier
	configs  map[string]ServiceSettings
	mu       sync.RWMutex
}

// NewManager 빈 설정 Manager를 생성합니다.
func NewManager() *Manager {
	return &Manager{
		appliers: make([]Applier, 0),
		configs:  make(map[string]ServiceSettings),
	}
}

// AddApplier 설정 변경을 수신할 Applier를 추가합니다.
func (m *Manager) AddApplier(applier Applier) {
	m.appliers = append(m.appliers, applier)
}

// Register 설정 정보를 검증한 뒤 등록된 Applier들에게 전달합니다.
func (m *Manager) Register(settings Settings) error {
	serviceName := utils.NormalizeServiceName(settings.ServiceName)
	if settings.Global == nil {
		return fmt.Errorf("%w: global settings is required", ErrInvalidSettings)
	}

	if serviceName == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidSettings)
	}
	if len(settings.Endpoints) == 0 {
		return fmt.Errorf("%w: endpoints are required", ErrInvalidSettings)
	}
	if settings.Instance == nil {
		return fmt.Errorf("%w: instance is required", ErrInvalidSettings)
	}
	if normalizedScheme := strings.ToLower(strings.TrimSpace(settings.Instance.Scheme)); normalizedScheme != "http" && normalizedScheme != "https" {
		return fmt.Errorf("%w: instance scheme is required", ErrInvalidSettings)
	}

	if _, err := utils.ConfigKey(settings.Instance.Host, settings.Instance.Port); err != nil {
		return fmt.Errorf("%w: config address: %w", ErrInvalidSettings, err)
	}

	for _, applier := range m.appliers {
		if applier == nil {
			return fmt.Errorf("%w: applier is required", ErrInvalidSettings)
		}

		if err := applier.Apply(settings); err != nil {
			return fmt.Errorf("apply config to component %T: %w", applier, err)
		}
	}

	return m.addSettings(serviceName, settings)
}

// ConfigFor 지정한 서비스 이름으로 등록된 설정 정보의 사본을 반환합니다.
func (m *Manager) ConfigFor(serviceName string) (ServiceSettings, bool) {
	if m == nil {
		return ServiceSettings{}, false
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return ServiceSettings{}, false
	}

	m.mu.RLock()
	settings, found := m.configs[normalizedServiceName]
	m.mu.RUnlock()
	if !found {
		return ServiceSettings{}, false
	}

	return cloneServiceSettings(settings), true
}

func (m *Manager) addSettings(serviceName string, settings Settings) error {
	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidSettings)
	}
	if settings.Instance == nil {
		return fmt.Errorf("%w: instance is required", ErrInvalidSettings)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	serviceSettings, found := m.configs[normalizedServiceName]
	if !found {
		m.configs[normalizedServiceName] = convertSettings(normalizedServiceName, settings)
		return nil
	}

	serviceSettings.Instances = append(serviceSettings.Instances, *cloneInstanceSettings(settings.Instance))
	m.configs[normalizedServiceName] = serviceSettings

	return nil
}

func convertSettings(serviceName string, settings Settings) ServiceSettings {
	return ServiceSettings{
		ServiceName: serviceName,
		Global:      cloneGlobalSettings(settings.Global),
		Threshold:   cloneThresholdSettings(settings.Threshold),
		Endpoints:   cloneEndpointSettings(settings.Endpoints),
		Instances:   []InstanceSettings{*cloneInstanceSettings(settings.Instance)},
	}
}

func cloneServiceSettings(settings ServiceSettings) ServiceSettings {
	return ServiceSettings{
		ServiceName: settings.ServiceName,
		Global:      cloneGlobalSettings(settings.Global),
		Threshold:   cloneThresholdSettings(settings.Threshold),
		Endpoints:   cloneEndpointSettings(settings.Endpoints),
		Instances:   append([]InstanceSettings(nil), settings.Instances...),
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

func cloneInstanceSettings(settings *InstanceSettings) *InstanceSettings {
	if settings == nil {
		return nil
	}

	return &InstanceSettings{
		Scheme: settings.Scheme,
		Host:   settings.Host,
		Port:   settings.Port,
	}
}

func cloneThresholdSettings(settings *ThresholdSettings) *ThresholdSettings {
	if settings == nil {
		return nil
	}

	return &ThresholdSettings{
		Normal: settings.Normal,
		Hot:    settings.Hot,
		Super:  settings.Super,
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
