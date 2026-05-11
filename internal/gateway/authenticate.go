package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	internalauth "wintergate/internal/auth"
)

// TokenDecoder Bearer 토큰을 검증하고 claims를 반환하는 계약입니다.
type TokenDecoder interface {
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
	// 매칭된 라우트가 없거나 roles가 비어 있으면 권한 검사가 필요 없습니다.
	if state.Route == nil || len(state.Route.Roles) == 0 {
		return nil
	}

	if strings.TrimSpace(state.Request.AuthorizationHeader) == "" {
		return fmt.Errorf("authorization header is required: %w", internalauth.ErrInvalidAuthorizationHeader)
	}

	// Authorization 헤더에서 Bearer 토큰 값만 분리합니다.
	token, err := internalauth.BearerTokenFor(state.Request.AuthorizationHeader)
	if err != nil {
		return fmt.Errorf("extract bearer token: %w", err)
	}

	// 토큰을 검증할 decoder가 없으면 인증 설정이 불완전하므로 실패합니다.
	if t.decoder == nil {
		return fmt.Errorf("%w: token decoder is required", ErrNilTokenDecoder)
	}

	// 토큰 서명과 claims를 검증하고 이후 task에서 사용할 수 있도록 저장합니다.
	claims, err := t.decoder.DecodeFor(state.Request.ServiceName, token)
	if err != nil {
		slog.Info(
			logJWTTokenDecodeFailed,
			logAttrServiceName, state.Request.ServiceName,
			logAttrMethod, state.Request.Method,
			logAttrPath, state.Request.Path,
			logAttrRequestHost, requestHostForLog(state),
			logAttrUpstreamHost, upstreamHostForLog(state),
			logAttrError, err,
		)
		return fmt.Errorf("decode bearer token: %w", err)
	}

	state.Claims = &claims
	slog.Info(
		logJWTTokenDecodeSucceeded,
		logAttrServiceName, state.Request.ServiceName,
		logAttrMethod, state.Request.Method,
		logAttrPath, state.Request.Path,
		logAttrRequestHost, requestHostForLog(state),
		logAttrUpstreamHost, upstreamHostForLog(state),
	)

	return nil
}

func requestHostForLog(state *State) string {
	if state == nil || state.Request.HTTPRequest == nil {
		return ""
	}

	return strings.TrimSpace(state.Request.HTTPRequest.Host)
}

func upstreamHostForLog(state *State) string {
	if state == nil || state.Route == nil {
		return ""
	}

	instance := state.Route.Instance
	if strings.TrimSpace(instance.Port) == "" {
		return strings.TrimSpace(instance.Host)
	}

	return strings.TrimSpace(instance.Host) + ":" + strings.TrimSpace(instance.Port)
}
