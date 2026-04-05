package gateway

import (
	"context"
	"fmt"
	"strings"

	internalroute "wintergate/internal/route"

	internalauth "wintergate/internal/auth"
	internalconfig "wintergate/internal/route/config"
)

type RouteTask struct {
	router  *internalroute.Router
	decoder *internalauth.Decoder
}

func NewRouteTask(router *internalroute.Router, decoder *internalauth.Decoder) *RouteTask {
	return &RouteTask{
		router:  router,
		decoder: decoder,
	}
}

func (t *RouteTask) Run(ctx context.Context, state *State) error {
	routeInfos, err := t.router.Route(state.Request.Service)
	if err != nil {
		return err
	}

	for _, routeInfo := range routeInfos {
		// 요청과 동일한 엔트포인트 조건인지 확인힙나다.
		if checkAuthRoute(routeInfo, state.Request.Method, state.Request.Path) {
			if strings.TrimSpace(state.Request.AuthorizationHeader) == "" {
				return fmt.Errorf("authorization header is required: %w", internalauth.ErrInvalidAuthorizationHeader)
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

			// 적절한 권한을 지고 있는지 확인합니다.
			if !checkRole(routeInfo, state.Claims.Roles) {
				return fmt.Errorf(
					"%w: service %q does not allow %s %s",
					ErrInvalidRequest,
					state.Request.Service,
					state.Request.Method,
					state.Request.Path,
				)
			}

			return nil
		}
	}

	return nil
}

func checkAuthRoute(routeInfo internalconfig.RouteInfo, method, path string) bool {
	if routeInfo.HttpMethod != method || routeInfo.Path != path {
		return false
	}

	return true
}

// checkRole 엔트포인트 통과에 필요한 권한이 현재 claim에 존재하는지 확인합니다.
func checkRole(routeInfo internalconfig.RouteInfo, roles []string) bool {
	// 권한이 필요없으므로 true로 반환합니다.
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
