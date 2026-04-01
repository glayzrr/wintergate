package gatewayapi

import (
	"errors"
	"net/http"

	configapi "wintergate/api/config"
	responseapi "wintergate/api/response"
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
		Method: ctx.Request.Method,
		Path:   requestPath,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, internalgateway.ErrInvalidRequest) {
			statusCode = http.StatusBadRequest
		}

		ctx.AbortWithStatusJSON(statusCode, responseapi.APIResponse{
			Success: false,
			Message: responseReceiveFailed,
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
