package config

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// Validator 후보 스냅샷 전체가 런타임에 반영 가능한지 검증합니다.
type Validator interface {
	Validate(candidate Snapshot) error
}

// SnapshotListener 활성 설정 스냅샷 commit 이후 알림을 받습니다.
type SnapshotListener interface {
	OnSnapshotCommitted(snapshot *Snapshot)
}

// SettingsProvider 현재 활성 설정 스냅샷을 제공합니다.
type SettingsProvider interface {
	Settings() *Snapshot
}

// Manager 중앙 설정 snapshot을 생성, 검증, commit합니다.
type Manager struct {
	validators []Validator
	listeners  []SnapshotListener
	settings   atomic.Pointer[Snapshot]
	mu         sync.Mutex
}

// NewManager 빈 설정 Manager를 생성합니다.
func NewManager() *Manager {
	manager := &Manager{}
	manager.settings.Store(&Snapshot{
		Services: make(map[string]ServiceSettings),
		Routes:   make(map[RouteKey]RouteEntry),
	})

	return manager
}

// AddValidator 설정 후보 스냅샷을 검증할 Validator를 추가합니다.
func (m *Manager) AddValidator(validator Validator) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.validators = append(m.validators, validator)
}

// AddSnapshotListener 설정 commit 이후 활성 스냅샷을 받을 listener를 추가합니다.
func (m *Manager) AddSnapshotListener(listener SnapshotListener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listeners = append(m.listeners, listener)
}

// Register 설정 정보를 후보 snapshot으로 만든 뒤 검증을 통과한 경우에만 commit합니다.
func (m *Manager) Register(settings Settings) error {
	// 요청 payload를 서비스 키와 런타임 저장소에서 공통으로 사용할 정규화된 값으로 맞춥니다.
	serviceName, normalizedSettings, err := normalizeSettings(settings)
	if err != nil {
		return err
	}

	// 설정 등록은 current snapshot 기준 candidate 생성부터 commit까지 직렬화합니다.
	m.mu.Lock()
	candidate, err := buildCandidate(m.settings.Load(), serviceName, normalizedSettings)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	listeners, err := m.commitCandidate(candidate)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	notifySnapshotListeners(listeners, candidate)

	return nil
}

// DeregisterInstance 지정한 서비스의 인스턴스를 후보 snapshot에서 제거한 뒤 commit합니다.
func (m *Manager) DeregisterInstance(serviceName string, instance InstanceSettings) error {
	normalizedServiceName := normalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidSettings)
	}

	normalizedInstance, err := normalizeInstanceSettings(normalizedServiceName, instance)
	if err != nil {
		return err
	}

	m.mu.Lock()
	candidate, err := buildDeregisterCandidate(m.settings.Load(), normalizedServiceName, normalizedInstance)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	listeners, err := m.commitCandidate(candidate)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	notifySnapshotListeners(listeners, candidate)

	return nil
}

func (m *Manager) commitCandidate(candidate *Snapshot) ([]SnapshotListener, error) {
	// 전체 후보 snapshot을 모든 validator가 승인해야 런타임 상태에 반영할 수 있습니다.
	for _, validator := range m.validators {
		if validator == nil {
			return nil, fmt.Errorf("%w: validator is required", ErrInvalidSettings)
		}

		if err := validator.Validate(*candidate); err != nil {
			return nil, fmt.Errorf("validate config with component %T: %w", validator, err)
		}
	}

	// 모든 검증이 끝난 후보만 활성 snapshot으로 공개합니다.
	m.commit(candidate)

	return append([]SnapshotListener(nil), m.listeners...), nil
}

func (m *Manager) commit(candidate *Snapshot) {
	m.settings.Store(candidate)
}

func notifySnapshotListeners(listeners []SnapshotListener, snapshot *Snapshot) {
	for _, listener := range listeners {
		if listener == nil {
			continue
		}

		listener.OnSnapshotCommitted(snapshot)
	}
}

// Settings 현재 활성 설정 스냅샷을 반환합니다. 반환된 스냅샷은 수정하지 않아야 합니다.
func (m *Manager) Settings() *Snapshot {
	if m == nil {
		return nil
	}

	return m.settings.Load()
}

// ConfigFor 지정한 서비스 이름으로 등록된 설정 정보의 사본을 반환합니다.
func (m *Manager) ConfigFor(serviceName string) (ServiceSettings, bool) {
	snapshot := m.Settings()
	if snapshot == nil {
		return ServiceSettings{}, false
	}

	normalizedServiceName := normalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return ServiceSettings{}, false
	}

	settings, found := snapshot.Services[normalizedServiceName]
	if !found {
		return ServiceSettings{}, false
	}

	return settings.Clone(), true
}
