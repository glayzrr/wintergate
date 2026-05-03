package gatewayapi

import (
	"errors"
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

// NewHandler 생성된 Orchestrator를 주입받아 게이트웨이 Handler를 생성합니다.
func NewHandler(orchestrator *internalgateway.Orchestrator) *Handler {
	return &Handler{
		orchestrator: orchestrator,
	}
}

// RegisterRoutes 게이트웨이 트래픽 수신 진입점을 Gin 엔진의 기본 라우트로 등록합니다.
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	router.NoRoute(h.Receive)
}

// Receive 등록되지 않은 모든 외부 요청을 게이트웨이 수준에서 수신합니다.
func (h *Handler) Receive(ctx *gin.Context) {
	requestPath := ctx.Request.URL.Path
	if requestPath == configapi.ConfigRoute {
		ctx.AbortWithStatusJSON(http.StatusNotFound, responseapi.APIResponse{
			Success: false,
			Message: responseNotFound,
		})
		return
	}

	err := h.orchestrator.Receive(ctx.Request.Context(), internalgateway.Request{
		Service:             ctx.GetHeader(requestHeaderService),
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
			Received: true,
			Method:   ctx.Request.Method,
			Path:     requestPath,
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
