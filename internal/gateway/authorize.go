package gateway

import (
	"context"
	"fmt"
	"log/slog"

	internalauth "wintergate/internal/auth"
	internalconfig "wintergate/internal/route/config"
)

// AuthorizeTask 매칭된 라우트 정책과 JWT claims의 권한을 비교합니다.
type AuthorizeTask struct{}

// NewAuthorizeTask 라우트 권한 검사용 AuthorizeTask를 생성합니다.
func NewAuthorizeTask() *AuthorizeTask {
	return &AuthorizeTask{}
}

// Run 라우트 정책에 roles가 있으면 claims의 roles와 비교합니다.
func (t *AuthorizeTask) Run(_ context.Context, state *State) error {
	// 매칭된 라우트가 없거나 roles가 비어 있으면 권한 검사가 필요 없습니다.
	if state.Route == nil || len(state.Route.Roles) == 0 {
		return nil
	}

	// roles가 필요한 라우트에서는 인증 task가 저장한 claims가 반드시 있어야 합니다.
	if state.Claims == nil {
		err := fmt.Errorf("authorization header is required: %w", internalauth.ErrInvalidAuthorizationHeader)
		slog.Info(
			logAuthorizeFailed,
			logAttrServiceName, state.Request.ServiceName,
			logAttrMethod, state.Request.Method,
			logAttrPath, state.Request.Path,
			logAttrRequestHost, requestHostForLog(state),
			logAttrUpstreamHost, upstreamHostForLog(state),
			logAttrAllowedRoles, state.Route.Roles,
			logAttrError, err,
		)
		return err
	}

	// 라우트에서 허용한 role 중 하나라도 claims에 있는지 확인합니다.
	if !checkRole(*state.Route, state.Claims.Roles) {
		err := fmt.Errorf(
			"%w: service-name %q does not allow %s %s",
			ErrInvalidRequest,
			state.Request.ServiceName,
			state.Request.Method,
			state.Request.Path,
		)
		slog.Info(
			logAuthorizeFailed,
			logAttrServiceName, state.Request.ServiceName,
			logAttrMethod, state.Request.Method,
			logAttrPath, state.Request.Path,
			logAttrRequestHost, requestHostForLog(state),
			logAttrUpstreamHost, upstreamHostForLog(state),
			logAttrAllowedRoles, state.Route.Roles,
			logAttrRoles, state.Claims.Roles,
			logAttrError, err,
		)
		return err
	}

	slog.Info(
		logAuthorizeSucceeded,
		logAttrServiceName, state.Request.ServiceName,
		logAttrMethod, state.Request.Method,
		logAttrPath, state.Request.Path,
		logAttrRequestHost, requestHostForLog(state),
		logAttrUpstreamHost, upstreamHostForLog(state),
		logAttrAllowedRoles, state.Route.Roles,
		logAttrRoles, state.Claims.Roles,
	)

	return nil
}

func checkRole(routeInfo internalconfig.RouteInfo, roles []string) bool {
	if len(routeInfo.Roles) == 0 {
		return true
	}

	for _, role := range roles {
		for _, allowedRole := range routeInfo.Roles {
			if role == allowedRole {
				return true
			}
		}
	}

	return false
}
