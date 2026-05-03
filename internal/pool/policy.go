package pool

import (
	"fmt"
	"strings"
	"sync"
)

// Threshold 특정 풀 티어로 승격하기 위한 RPS/in-flight 기준입니다.
type Threshold struct {
	RPS      float64
	InFlight int64
}

// Policy 서비스별 트래픽 분류 정책입니다.
type Policy struct {
	Service       string
	Hot           Threshold
	Super         Threshold
	DedicatedFrom Tier
}

// Decision 현재 트래픽 상태와 등록 정책을 바탕으로 결정한 풀 사용 방식입니다.
type Decision struct {
	Service    string
	Registered bool
	Tier       Tier
	Dedicated  bool
}

// PolicyRegistry 서비스별 트래픽 분류 정책을 저장합니다.
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

// RegisterPolicies 기본 정책 저장소에 서비스별 정책을 등록합니다.
func RegisterPolicies(policies []Policy) error {
	return DefaultPolicyRegistry().Register(policies)
}

// DecidePolicy 기본 정책 저장소에서 현재 상태에 대한 풀 사용 방식을 결정합니다.
func DecidePolicy(status Status) Decision {
	return DefaultPolicyRegistry().Decide(status)
}

// Register 전달받은 정책 목록으로 현재 정책을 교체합니다.
func (r *PolicyRegistry) Register(policies []Policy) error {
	if r == nil {
		return fmt.Errorf("%w: registry is nil", ErrInvalidPolicy)
	}

	registeredPolicies := make(map[string]Policy, len(policies))
	for _, policy := range policies {
		normalizedPolicy, err := normalizePolicy(policy)
		if err != nil {
			return err
		}

		if _, exists := registeredPolicies[normalizedPolicy.Service]; exists {
			return fmt.Errorf("%w: duplicate service %q", ErrInvalidPolicy, normalizedPolicy.Service)
		}

		registeredPolicies[normalizedPolicy.Service] = normalizedPolicy
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.policies = registeredPolicies

	return nil
}

// Policy 서비스별 등록 정책의 사본을 반환합니다.
func (r *PolicyRegistry) Policy(service string) (Policy, bool) {
	if r == nil {
		return Policy{}, false
	}

	normalizedService := normalizeService(service)
	if normalizedService == "" {
		return Policy{}, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	policy, found := r.policies[normalizedService]

	return policy, found
}

// Decide 등록 정책이 있으면 RPS/in-flight 기준으로 tier와 전용 풀 여부를 결정합니다.
func (r *PolicyRegistry) Decide(status Status) Decision {
	normalizedService := normalizeService(status.Service)

	// TODO 서버 기본 설정에 따라 티어 설정하기
	decision := Decision{
		Service: normalizedService,
		Tier:    TierNormal,
	}
	if normalizedService == "" || r == nil {
		return decision
	}

	// 등록된 정책이 없으면 기본 정책을 반환합니다.
	policy, found := r.Policy(normalizedService)
	if !found {
		return decision
	}

	decision.Registered = true
	decision.Tier = decideTier(status, policy)
	decision.Dedicated = isDedicatedTier(decision.Tier, policy.DedicatedFrom)

	return decision
}

func normalizePolicy(policy Policy) (Policy, error) {
	service := normalizeService(policy.Service)
	if service == "" {
		return Policy{}, fmt.Errorf("%w: service is required", ErrInvalidPolicy)
	}

	normalizedDedicatedFrom, err := normalizeDedicatedFrom(policy.DedicatedFrom)
	if err != nil {
		return Policy{}, err
	}

	if err := validateThreshold(policy.Hot, "hot"); err != nil {
		return Policy{}, err
	}
	if err := validateThreshold(policy.Super, "super"); err != nil {
		return Policy{}, err
	}

	policy.Service = service
	policy.DedicatedFrom = normalizedDedicatedFrom

	return policy, nil
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

func normalizeDedicatedFrom(tier Tier) (Tier, error) {
	trimmedTier := strings.ToLower(strings.TrimSpace(string(tier)))
	if trimmedTier == "" {
		return "", nil
	}

	switch Tier(trimmedTier) {
	case TierNormal, TierHot, TierSuper:
		return Tier(trimmedTier), nil
	default:
		return "", fmt.Errorf("%w: unsupported dedicated_from tier %q", ErrInvalidPolicy, tier)
	}
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
	if threshold.RPS > 0 && status.RPS >= threshold.RPS {
		return true
	}
	if threshold.InFlight > 0 && status.InFlight >= threshold.InFlight {
		return true
	}

	return false
}

func isDedicatedTier(tier Tier, dedicatedFrom Tier) bool {
	switch dedicatedFrom {
	case TierNormal:
		return tier == TierNormal || tier == TierHot || tier == TierSuper
	case TierHot:
		return tier == TierHot || tier == TierSuper
	case TierSuper:
		return tier == TierSuper
	default:
		return false
	}
}
