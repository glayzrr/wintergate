package pool

import (
	"fmt"
	"sync"
)

// Threshold 특정 풀 티어로 승격하기 위한 RPS/in-flight 기준입니다.
type Threshold struct {
	RPS      float64
	InFlight int64
}

// Policy 설정 키별 트래픽 분류 정책입니다.
type Policy struct {
	ConfigKey string
	Hot       Threshold
	Super     Threshold
}

// Decision 현재 트래픽 상태와 등록 정책을 바탕으로 결정한 풀 사용 방식입니다.
type Decision struct {
	ConfigKey string
	Tier      Tier
	Dedicated bool
	Status    Status
}

// PolicyRegistry 설정 키별 트래픽 분류 정책을 저장합니다.
type PolicyRegistry struct {
	policies map[string]Policy

	// mu는 policies map의 동시 조회와 정책 교체를 보호합니다.
	mu sync.RWMutex
}

var defaultPolicyRegistry = NewPolicyRegistry()

// NewPolicyRegistry 빈 트래픽 정책 저장소를 생성합니다.
func NewPolicyRegistry() *PolicyRegistry {
	return &PolicyRegistry{
		policies: make(map[string]Policy),
	}
}

// DefaultPolicyRegistry 패키지 기본 트래픽 정책 저장소를 반환합니다.
func DefaultPolicyRegistry() *PolicyRegistry {
	return defaultPolicyRegistry
}

// RegisterPolicies 기본 정책 저장소에 설정 키별 정책을 등록합니다.
func RegisterPolicies(policies []Policy) error {
	return DefaultPolicyRegistry().Register(policies)
}

// UnregisterPolicy 기본 정책 저장소에서 설정 키별 정책을 제거합니다.
func UnregisterPolicy(configKey string) {
	DefaultPolicyRegistry().Delete(configKey)
}

// DecidePolicy 기본 정책 저장소에서 현재 상태에 대한 풀 사용 방식을 결정합니다.
func DecidePolicy(status Status) Decision {
	return DefaultPolicyRegistry().Decide(status)
}

// Register 전달받은 정책 목록을 등록하거나 기존 정책을 교체합니다.
func (r *PolicyRegistry) Register(policies []Policy) error {
	if r == nil {
		return fmt.Errorf("%w: registry is nil", ErrInvalidPolicy)
	}

	registeredPolicies := make(map[string]Policy, len(policies))
	for _, policy := range policies {
		normalizedPolicy := policy
		normalizedPolicy.ConfigKey = normalizeConfigKey(policy.ConfigKey)
		if normalizedPolicy.ConfigKey == "" {
			return fmt.Errorf("%w: config key is required", ErrInvalidPolicy)
		}

		if err := validateThreshold(normalizedPolicy.Hot, "hot"); err != nil {
			return err
		}
		if err := validateThreshold(normalizedPolicy.Super, "super"); err != nil {
			return err
		}

		registeredPolicies[normalizedPolicy.ConfigKey] = normalizedPolicy
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for configKey, policy := range registeredPolicies {
		r.policies[configKey] = policy
	}

	return nil
}

// Delete 지정한 설정 키의 정책을 제거합니다.
func (r *PolicyRegistry) Delete(configKey string) {
	if r == nil {
		return
	}

	normalizedConfigKey := normalizeConfigKey(configKey)
	if normalizedConfigKey == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.policies, normalizedConfigKey)
}

// PolicyFor 설정 키별 등록 정책의 사본을 반환합니다.
func (r *PolicyRegistry) PolicyFor(configKey string) (Policy, bool) {
	if r == nil {
		return Policy{}, false
	}

	normalizedConfigKey := normalizeConfigKey(configKey)
	if normalizedConfigKey == "" {
		return Policy{}, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	policy, found := r.policies[normalizedConfigKey]

	return policy, found
}

// Decide 등록 정책이 있으면 RPS/in-flight 기준으로 tier를 결정합니다.
func (r *PolicyRegistry) Decide(status Status) Decision {
	normalizedConfigKey := normalizeConfigKey(status.ConfigKey)

	decision := Decision{
		ConfigKey: normalizedConfigKey,
		Tier:      DefaultTier(),
		Status:    status,
	}
	if normalizedConfigKey == "" || r == nil {
		return decision
	}

	// 등록된 정책이 없으면 기본 정책을 반환합니다.
	policy, found := r.PolicyFor(normalizedConfigKey)
	if !found {
		return decision
	}

	decision.Tier = decideTier(status, policy)
	decision.Dedicated = true

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

func decideTier(status Status, policy Policy) Tier {
	if thresholdReached(status, policy.Super) {
		return TierSuper
	}
	if thresholdReached(status, policy.Hot) {
		return TierHot
	}

	return TierNormal
}

func thresholdReached(status Status, threshold Threshold) bool {
	if threshold.RPS > 0 || status.RPS >= threshold.RPS {
		return true
	}
	if threshold.InFlight > 0 || status.InFlight >= threshold.InFlight {
		return true
	}

	return false
}
