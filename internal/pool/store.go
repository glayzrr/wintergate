package pool

import (
	"fmt"
	"sync"

	"wintergate/internal/config"
	"wintergate/internal/utils"
)

// Threshold 특정 풀 티어로 승격하기 위한 RPS/in-flight 기준입니다.
type Threshold struct {
	RPS      float64
	InFlight int64
}

// poolInfo 서비스 이름별 트래픽 분류 정책입니다.
type poolInfo struct {
	ConfigKey string
	Normal    Threshold
	Hot       Threshold
	Super     Threshold
}

// Assignment 현재 트래픽 상태와 등록 정책을 바탕으로 결정한 풀 사용 방식입니다.
type Assignment struct {
	ConfigKey string
	Tier      Tier
	Dedicated bool
	Status    Status
}

// Store 서비스 이름별 트래픽 분류 정책을 저장합니다.
type Store struct {
	policies map[string]poolInfo

	// mu는 policies map의 동시 조회와 정책 교체를 보호합니다.
	mu sync.RWMutex
}

// NewStore 빈 트래픽 정책 저장소를 생성합니다.
func NewStore() *Store {
	return &Store{
		policies: make(map[string]poolInfo),
	}
}

// Apply 전달받은 서비스 설정의 threshold를 서비스 이름별 정책으로 반영합니다.
func (s *Store) Apply(settings config.Settings) error {
	if s == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidPolicy)
	}

	normalizedServiceName := utils.NormalizeServiceName(settings.ServiceName)
	if normalizedServiceName == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidPolicy)
	}

	if settings.Threshold == nil {
		s.Delete(normalizedServiceName)
		return nil
	}

	policy := poolInfo{
		ConfigKey: normalizedServiceName,
		Normal: Threshold{
			RPS:      settings.Threshold.Normal.RPS,
			InFlight: settings.Threshold.Normal.InFlight,
		},
		Hot: Threshold{
			RPS:      settings.Threshold.Hot.RPS,
			InFlight: settings.Threshold.Hot.InFlight,
		},
		Super: Threshold{
			RPS:      settings.Threshold.Super.RPS,
			InFlight: settings.Threshold.Super.InFlight,
		},
	}

	if err := validateThreshold(policy.Normal, "normal"); err != nil {
		return err
	}
	if err := validateThreshold(policy.Hot, "hot"); err != nil {
		return err
	}
	if err := validateThreshold(policy.Super, "super"); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.policies[normalizedServiceName] = policy

	return nil
}

// Delete 지정한 서비스 이름의 정책을 제거합니다.
func (s *Store) Delete(serviceName string) {
	if s == nil {
		return
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.policies, normalizedServiceName)
}

// PolicyFor 서비스 이름별 등록 정책의 사본을 반환합니다.
func (s *Store) PolicyFor(serviceName string) (poolInfo, bool) {
	if s == nil {
		return poolInfo{}, false
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return poolInfo{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, found := s.policies[normalizedServiceName]

	return policy, found
}

// DecisionFor 등록 정책이 있으면 RPS/in-flight 기준으로 tier를 결정합니다.
func (s *Store) DecisionFor(status Status) Assignment {
	normalizedServiceName := utils.NormalizeServiceName(status.ConfigKey)

	decision := Assignment{
		ConfigKey: normalizedServiceName,
		Tier:      DefaultTier(),
		Status:    status,
	}
	if normalizedServiceName == "" || s == nil {
		return decision
	}

	// 등록된 정책이 없으면 기본 정책을 반환합니다.
	policy, found := s.PolicyFor(normalizedServiceName)
	if !found {
		return decision
	}

	decision.Tier, decision.Dedicated = decideTier(status, policy)

	return decision
}

func validateThreshold(threshold Threshold, name string) error {
	if threshold.RPS < 0 {
		return fmt.Errorf("%w: %s rps must be greater than or equal to zero", ErrInvalidPolicy, name)
	}
	if threshold.InFlight < 0 {
		return fmt.Errorf("%w: %s in-flight must be greater than or equal to zero", ErrInvalidPolicy, name)
	}

	return nil
}

func decideTier(status Status, policy poolInfo) (Tier, bool) {
	if thresholdReached(status, policy.Super) {
		return TierSuper, true
	}
	if thresholdReached(status, policy.Hot) {
		return TierHot, true
	}
	if thresholdReached(status, policy.Normal) {
		return TierNormal, true
	}

	return DefaultTier(), false
}

func thresholdReached(status Status, threshold Threshold) bool {
	if threshold.RPS > 0 && status.RPS >= threshold.RPS {
		return true
	}
	if threshold.InFlight > 0 && status.InFlight >= threshold.InFlight {
		return true
	}

	return false
}
