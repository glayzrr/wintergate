package gateway

import internalauth "wintergate/internal/auth"

// Request 게이트웨이가 수신한 요청의 핵심 정보만 분리해 전달합니다.
type Request struct {
	Method              string
	Path                string
	AuthorizationHeader string
}

// Result 오케스트레이터가 작업 수행 후 반환하는 요청 처리 결과입니다.
type Result struct {
	Received bool
	Method   string
	Path     string
}

// State 오케스트레이터가 각 작업 사이에서 공유하는 요청 처리 상태입니다.
type State struct {
	Request Request
	Result  Result
	Claims  *internalauth.Claims
}
