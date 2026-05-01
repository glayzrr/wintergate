package pool

// Tier 트래픽 규모별 커넥션 풀 설정 단계를 표현합니다.
type Tier string

const (
	// TierNormal 일반 트래픽용 기본 풀 설정입니다.
	TierNormal Tier = "normal"
	// TierHot 트래픽이 몰리는 서비스용 풀 설정입니다.
	TierHot Tier = "hot"
	// TierSuper 매우 높은 트래픽을 받는 서비스용 풀 설정입니다.
	TierSuper Tier = "super"
)
