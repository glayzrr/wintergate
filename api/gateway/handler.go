package gatewayapi

import (
	"net/http"

	configapi "wintergate/api/config"
	responseapi "wintergate/api/response"

	"github.com/gin-gonic/gin"
)

// Handler 게이트웨이의 공통 트래픽 수신 진입점을 담당합니다.
type Handler struct{}

// NewHandler 게이트웨이 트래픽 수신 Handler를 생성합니다.
func NewHandler() *Handler {
	return &Handler{}
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
