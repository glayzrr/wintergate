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
	if strings.TrimSpace(state.Request.AuthorizationHeader) == "" {
		return nil
	}

	token, err := internalauth.BearerToken(state.Request.AuthorizationHeader)
	if err != nil {
		return fmt.Errorf("extract bearer token: %w", err)
	}

	claims, err := t.decoder.Decode(token)
	if err != nil {
		return fmt.Errorf("decode bearer token: %w", err)
	}

	state.Claims = &claims

	return nil
}
