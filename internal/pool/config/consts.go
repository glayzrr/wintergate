package config

type Tier string

const (
	TierShared Tier = "shared"
	TierNormal Tier = "normal"
	TierHot    Tier = "hot"
	TierSuper  Tier = "super"
)
