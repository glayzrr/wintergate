package config

import (
	"fmt"
	"strings"

	"wintergate/internal/utils"
)

// Applier 설정 변경을 자신의 런타임 상태에 반영합니다.
type Applier interface {
	Apply(settings Settings) error
}

// Manager 설정 정보를 등록된 Applier들에게 전달합니다.
type Manager struct {
	appliers []Applier
}

// NewManager 빈 설정 Manager를 생성합니다.
func NewManager() *Manager {
	return &Manager{
		appliers: make([]Applier, 0),
	}
}

// AddApplier 설정 변경을 수신할 Applier를 추가합니다.
func (m *Manager) AddApplier(applier Applier) {
	m.appliers = append(m.appliers, applier)
}

// Register 설정 정보를 검증한 뒤 등록된 Applier들에게 전달합니다.
func (m *Manager) Register(settings Settings) error {
	if settings.Global == nil {
		return fmt.Errorf("%w: global settings is required", ErrInvalidSettings)
	}

	if utils.NormalizeServiceName(settings.ServiceName) == "" {
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

	return nil
}
