package health

const (
	logHealthStatusChanged       = "health status changed"
	logHealthCheckConfigSkipped  = "health check config skipped"
	logHealthResponseBodyDiscard = "health response body discard failed"

	logAttrServiceName         = "service_name"
	logAttrInstance            = "instance"
	logAttrStatus              = "status"
	logAttrPreviousStatus      = "previous_status"
	logAttrError               = "error"
	logAttrConsecutiveFailures = "consecutive_failures"
	logAttrConsecutiveSuccess  = "consecutive_successes"
)
