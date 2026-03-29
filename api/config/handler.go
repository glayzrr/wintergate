package configapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 외부 설정 스냅샷을 수신해 내부 레지스트리에 반영합니다.
type Handler struct {
	registerer *Registerer
}

// NewHandler 설정 수신 Handler를 생성합니다.
func NewHandler(registerer *Registerer) (*Handler, error) {
	if registerer == nil {
		return nil, fmt.Errorf("%w: registerer is required", ErrNilRegisterer)
	}

	return &Handler{
		registerer: registerer,
	}, nil
}

// RegisterRoutes 설정 수신 엔드포인트를 Gin 라우터에 등록합니다.
func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST(DefaultRoute, h.PutSnapshot)
}

// PutSnapshot 전달받은 설정 스냅샷을 내부 저장소에 반영합니다.
func (h *Handler) PutSnapshot(ctx *gin.Context) {
	var snapshot Snapshot
	if err := ctx.ShouldBindJSON(&snapshot); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": responseBindFailed})
		return
	}

	if err := h.registerer.Register(snapshot); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": responseRegisterFailed})
		return
	}

	ctx.Status(http.StatusNoContent)
}
