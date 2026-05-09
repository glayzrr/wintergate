package gateway

import (
	"context"
	"fmt"
	"strings"

	internalauth "wintergate/internal/auth"
)

// TokenDecoder Bearer 토큰을 검증하고 claims를 반환하는 계약입니다.
type TokenDecoder interface {
	Decode(token string) (internalauth.Claims, error)
}

type serviceNameTokenDecoder interface {
	DecodeFor(serviceName, token string) (internalauth.Claims, error)
}

// AuthenticateTask 게이트웨이 요청의 Bearer JWT를 검증합니다.
type AuthenticateTask struct {
	decoder TokenDecoder
}

// NewAuthenticateTask Bearer JWT 검증용 AuthenticateTask를 생성합니다.
func NewAuthenticateTask(decoder TokenDecoder) *AuthenticateTask {
	return &AuthenticateTask{decoder: decoder}
}

// Run Authorization 헤더가 있을 때 Bearer JWT를 검증하고 claims를 상태에 기록합니다.
func (t *AuthenticateTask) Run(_ context.Context, state *State) error {
	// 인증이 필요 없는 라우트도 있으므로 Authorization 헤더가 없으면 검증을 건너뜁니다.
	if strings.TrimSpace(state.Request.AuthorizationHeader) == "" {
		return nil
	}

	// Authorization 헤더에서 Bearer 토큰 값만 분리합니다.
	token, err := internalauth.BearerToken(state.Request.AuthorizationHeader)
	if err != nil {
		return fmt.Errorf("extract bearer token: %w", err)
	}

	// 토큰을 검증할 decoder가 없으면 인증 설정이 불완전하므로 실패합니다.
	if t.decoder == nil {
		return fmt.Errorf("%w: token decoder is required", ErrNilTokenDecoder)
	}

	// 토큰 서명과 claims를 검증하고 이후 task에서 사용할 수 있도록 저장합니다.
	var claims internalauth.Claims
	if decoder, ok := t.decoder.(serviceNameTokenDecoder); ok {
		claims, err = decoder.DecodeFor(state.Request.ServiceName, token)
	} else {
		claims, err = t.decoder.Decode(token)
	}
	if err != nil {
		return fmt.Errorf("decode bearer token: %w", err)
	}

	state.Claims = &claims

	return nil
}
