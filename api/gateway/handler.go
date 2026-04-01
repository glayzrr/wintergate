package gatewayapi

import (
	"errors"
	"fmt"
	"net/http"

	configapi "wintergate/api/config"
	responseapi "wintergate/api/response"
	internalauth "wintergate/internal/auth"
	authconfig "wintergate/internal/auth/config"
	internalgateway "wintergate/internal/gateway"

	"github.com/gin-gonic/gin"
)

// Handler 게이트웨이의 공통 트래픽 수신 진입점을 담당합니다.
type Handler struct {
	orchestrator *internalgateway.Orchestrator
}

// NewHandler 게이트웨이 트래픽 수신 Handler를 생성합니다.
func NewHandler() *Handler {
	return &Handler{
		orchestrator: internalgateway.NewOrchestrator(),
	}
}

// NewHandlerWithAuthRegistry 인증 디코딩 작업이 등록된 게이트웨이 Handler를 생성합니다.
func NewHandlerWithAuthRegistry(authRegistry *authconfig.Registry) (*Handler, error) {
	decoder := internalauth.NewDecoder()
	if err := decoder.ReplaceRegistry(authRegistry); err != nil {
		return nil, fmt.Errorf("use auth registry: %w", err)
	}

	authenticateTask, err := internalgateway.NewAuthenticateTask(decoder)
	if err != nil {
		return nil, fmt.Errorf("create authenticate task: %w", err)
	}

	return &Handler{
		orchestrator: internalgateway.NewOrchestrator(authenticateTask),
	}, nil
}

// RegisterRoutes 게이트웨이 트래픽 수신 진입점을 Gin 엔진의 기본 라우트로 등록합니다.
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	router.NoRoute(h.Receive)
}

// Receive 등록되지 않은 모든 외부 요청을 게이트웨이 수준에서 수신합니다.
func (h *Handler) Receive(ctx *gin.Context) {
	requestPath := ctx.Request.URL.Path
	if requestPath == configapi.DefaultRoute {
		ctx.AbortWithStatusJSON(http.StatusNotFound, responseapi.APIResponse{
			Success: false,
			Message: responseNotFound,
		})
		return
	}

	result, err := h.orchestrator.Receive(ctx.Request.Context(), internalgateway.Request{
		Method:              ctx.Request.Method,
		Path:                requestPath,
		AuthorizationHeader: ctx.GetHeader("Authorization"),
	})
	if err != nil {
		statusCode, message := receiveFailure(err)
		ctx.AbortWithStatusJSON(statusCode, responseapi.APIResponse{
			Success: false,
			Message: message,
		})
		return
	}

	ctx.JSON(http.StatusOK, responseapi.APIResponse{
		Success: true,
		Message: responseReceiveSuccess,
		Data: ReceiveResponse{
			Received: result.Received,
			Method:   result.Method,
			Path:     result.Path,
		},
	})
}

func receiveFailure(err error) (int, string) {
	switch {
	case errors.Is(err, internalgateway.ErrInvalidRequest):
		return http.StatusBadRequest, responseReceiveFailed
	case errors.Is(err, internalauth.ErrInvalidAuthorizationHeader):
		return http.StatusUnauthorized, responseUnauthorized
	case errors.Is(err, internalauth.ErrInvalidAudience):
		return http.StatusUnauthorized, responseUnauthorized
	case errors.Is(err, internalauth.ErrInvalidIssuer):
		return http.StatusUnauthorized, responseUnauthorized
	case errors.Is(err, internalauth.ErrInvalidSignature):
		return http.StatusUnauthorized, responseUnauthorized
	case errors.Is(err, internalauth.ErrInvalidToken):
		return http.StatusUnauthorized, responseUnauthorized
	case errors.Is(err, internalauth.ErrTokenExpired):
		return http.StatusUnauthorized, responseUnauthorized
	case errors.Is(err, internalauth.ErrTokenNotYetValid):
		return http.StatusUnauthorized, responseUnauthorized
	case errors.Is(err, internalauth.ErrUnsupportedAlgorithm):
		return http.StatusUnauthorized, responseUnauthorized
	case errors.Is(err, authconfig.ErrKeyNotFound):
		return http.StatusUnauthorized, responseUnauthorized
	default:
		return http.StatusInternalServerError, responseReceiveFailed
	}
}
