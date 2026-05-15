package gateway

import (
	"net/http"

	internalauth "wintergate/internal/auth"
	internalconfig "wintergate/internal/config"
	routeconfig "wintergate/internal/route/config"
)

// Request 게이트웨이가 수신한 요청의 핵심 정보만 분리해 전달합니다.
type Request struct {
	ID                  string
	ServiceName         string
	Method              string
	Path                string
	AuthorizationHeader string
	ResponseWriter      http.ResponseWriter
	HTTPRequest         *http.Request
}

// State 오케스트레이터가 각 작업 사이에서 공유하는 요청 처리 상태입니다.
type State struct {
	Settings *internalconfig.Snapshot
	Request  Request
	Route    *routeconfig.RouteInfo
	Claims   *internalauth.Claims
}
