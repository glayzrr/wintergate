package policy

import (
	"fmt"
	"sync"
	"time"

	internalconfig "wintergate/internal/config"
	poolconfig "wintergate/internal/pool/config"
	"wintergate/internal/pool/traffic"
	"wintergate/internal/utils"
)

const sharedReturnDelay = 30 * time.Second

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
	ServiceName string
	Tier        poolconfig.Tier
	Dedicated   bool
	Status      traffic.Status
}

// Store snapshot의 threshold 설정으로 pool assignment를 계산하고 shared 복귀를 지연합니다.
type Store struct {
	clock  func() time.Time
	states map[string]assignmentState
	mu     sync.Mutex
}

type assignmentState struct {
	revision   uint64
	policy     poolInfo
	tier       poolconfig.Tier
	belowSince time.Time
}

// NewStore 빈 트래픽 정책 저장소를 생성합니다.
func NewStore() *Store {
	return &Store{
		clock:  time.Now,
		states: make(map[string]assignmentState),
	}
}

// Validate 후보 스냅샷의 전체 풀 정책이 반영 가능한지 검증합니다.
func (s *Store) Validate(candidate internalconfig.Snapshot) error {
	if s == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidPolicy)
	}

	for _, service := range candidate.Services {
		if service.Threshold == nil {
			continue
		}

		policy := poolInfo{
			ConfigKey: utils.NormalizeServiceName(service.ServiceName),
			Normal: Threshold{
				RPS:      service.Threshold.Normal.RPS,
				InFlight: service.Threshold.Normal.InFlight,
			},
			Hot: Threshold{
				RPS:      service.Threshold.Hot.RPS,
				InFlight: service.Threshold.Hot.InFlight,
			},
			Super: Threshold{
				RPS:      service.Threshold.Super.RPS,
				InFlight: service.Threshold.Super.InFlight,
			},
		}

		if policy.ConfigKey == "" {
			return fmt.Errorf("%w: service-name is required", ErrInvalidPolicy)
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
	}

	return nil
}

// Apply 중앙 snapshot 전환 이후 threshold를 내부 저장소에 복제하지 않습니다.
func (s *Store) Apply(settings internalconfig.Settings) error {
	if s == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidPolicy)
	}

	normalizedServiceName := utils.NormalizeServiceName(settings.ServiceName)
	if normalizedServiceName == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidPolicy)
	}

	if settings.Threshold == nil {
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

	return nil
}

// Delete 지정한 서비스 이름의 정책을 제거합니다.
func (s *Store) Delete(serviceName string) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, utils.NormalizeServiceName(serviceName))
}

func (s *Store) policyFor(snapshot *internalconfig.Snapshot, serviceName string) (poolInfo, bool) {
	if s == nil || snapshot == nil {
		return poolInfo{}, false
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	if normalizedServiceName == "" {
		return poolInfo{}, false
	}

	service, found := snapshot.Services[normalizedServiceName]
	if !found || service.Threshold == nil {
		return poolInfo{}, false
	}

	return poolInfo{
		ConfigKey: normalizedServiceName,
		Normal: Threshold{
			RPS:      service.Threshold.Normal.RPS,
			InFlight: service.Threshold.Normal.InFlight,
		},
		Hot: Threshold{
			RPS:      service.Threshold.Hot.RPS,
			InFlight: service.Threshold.Hot.InFlight,
		},
		Super: Threshold{
			RPS:      service.Threshold.Super.RPS,
			InFlight: service.Threshold.Super.InFlight,
		},
	}, true
}

// AssignmentFor 등록 정책이 있으면 RPS/in-flight 기준으로 tier를 결정합니다.
func (s *Store) AssignmentFor(snapshot *internalconfig.Snapshot, status traffic.Status) Assignment {
	normalizedServiceName := utils.NormalizeServiceName(status.ConfigKey)

	decision := Assignment{
		ServiceName: normalizedServiceName,
		Tier:        poolconfig.TierShared,
		Status:      status,
	}
	if normalizedServiceName == "" || s == nil || snapshot == nil {
		return decision
	}

	// 설정 변경은 즉시 반영하고, 동일한 정책 안에서 트래픽 감소에 따른 shared 복귀만 지연합니다.
	policy, found := s.policyFor(snapshot, normalizedServiceName)
	if found {
		decision.Tier = decideTier(status, policy)
	}

	decision.Tier = s.delaySharedReturn(normalizedServiceName, snapshot.Revision, policy, decision.Tier)
	decision.Dedicated = decision.Tier != poolconfig.TierShared

	return decision
}

func (s *Store) delaySharedReturn(serviceName string, revision uint64, policy poolInfo, tier poolconfig.Tier) poolconfig.Tier {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.states == nil {
		s.states = make(map[string]assignmentState)
	}
	state, found := s.states[serviceName]

	// 이전 snapshot을 캡처한 요청은 해당 정책을 사용하되 최신 복귀 타이머를 변경하지 않습니다.
	if found && revision < state.revision {
		if state.policy == policy && tier == poolconfig.TierShared {
			return state.tier
		}
		return tier
	}

	if !found || state.policy != policy {
		state = assignmentState{policy: policy, tier: tier}
	} else if tier != poolconfig.TierShared {
		// 어떤 전용 tier 조건이든 다시 충족하면 복귀 대기를 취소합니다.
		state.tier = tier
		state.belowSince = time.Time{}
	} else if state.tier != poolconfig.TierShared {
		now := time.Now()
		if s.clock != nil {
			now = s.clock()
		}
		if state.belowSince.IsZero() {
			state.belowSince = now
		}

		// 첫 임계치 미만 관측부터 30초 뒤, 다음 요청별 평가에서 shared로 복귀합니다.
		if now.Sub(state.belowSince) >= sharedReturnDelay {
			state.tier = poolconfig.TierShared
			state.belowSince = time.Time{}
		}
	}

	state.revision = revision
	s.states[serviceName] = state
	return state.tier
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

func decideTier(status traffic.Status, policy poolInfo) poolconfig.Tier {
	if thresholdReached(status, policy.Super) {
		return poolconfig.TierSuper
	}
	if thresholdReached(status, policy.Hot) {
		return poolconfig.TierHot
	}
	if thresholdReached(status, policy.Normal) {
		return poolconfig.TierNormal
	}

	return poolconfig.TierShared
}

func thresholdReached(status traffic.Status, threshold Threshold) bool {
	if threshold.RPS > 0 && status.RPS >= threshold.RPS {
		return true
	}
	if threshold.InFlight > 0 && status.InFlight >= threshold.InFlight {
		return true
	}

	return false
}
