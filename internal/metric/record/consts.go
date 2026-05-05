package record

const (
	namespace = "wintergate"

	labelEvent      = "event"
	labelService    = "service"
	labelTier       = "tier"
	labelPool       = "pool"
	labelRoute      = "route"
	labelMethod     = "method"
	labelStatusCode = "status_code"
	labelResult     = "result"

	resultSuccess = "success"
	resultError   = "error"

	poolShared    = "shared"
	poolDedicated = "dedicated"

	unknown = "unknown"

	connectionEventNew        = "new"
	connectionEventReused     = "reused"
	connectionEventIdleReused = "idle_reused"

	routeGateway = "gateway"
)
