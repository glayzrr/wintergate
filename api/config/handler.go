package configapi

import (
	"fmt"
	"net/http"

	responseapi "wintergate/api/response"
	internalconfig "wintergate/internal/config"

	"github.com/gin-gonic/gin"
)

// Handler 외부 설정 정보를 수신해 내부 레지스트리에 반영합니다.
type Handler struct {
	registerer *internalconfig.Registerer
}

// NewHandler 설정 수신 Handler를 생성합니다.
func NewHandler(registerer *internalconfig.Registerer) (*Handler, error) {
	if registerer == nil {
		return nil, fmt.Errorf("%w: registerer is required", ErrNilRegisterer)
	}

	return &Handler{
		registerer: registerer,
	}, nil
}

// RegisterRoutes 설정 수신 엔드포인트를 Gin 라우터에 등록합니다.
func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST(DefaultRoute, h.EnrollConfig)
}

// EnrollConfig 전달받은 설정 정보를 내부 저장소에 반영합니다.
func (h *Handler) EnrollConfig(ctx *gin.Context) {
	var settings internalconfig.Settings
	if err := ctx.ShouldBindJSON(&settings); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, responseapi.APIResponse{
			Success: false,
			Message: responseBindFailed,
		})
		return
	}

	if err := h.registerer.Register(settings); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, responseapi.APIResponse{
			Success: false,
			Message: responseRegisterFailed,
		})
		return
	}

	ctx.JSON(http.StatusOK, responseapi.APIResponse{
		Success: true,
		Message: responseRegisterSuccess,
	})
}
