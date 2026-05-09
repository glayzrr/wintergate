package configapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	responseapi "wintergate/api/response"
	internalconfig "wintergate/internal/config"

	"github.com/gin-gonic/gin"
)

// Handler 외부 설정 정보를 수신해 내부 레지스트리에 반영합니다.
type Handler struct {
	manager *internalconfig.Manager
}

// NewHandler 설정 수신 Handler를 생성합니다.
func NewHandler(manager *internalconfig.Manager) (*Handler, error) {
	if manager == nil {
		return nil, fmt.Errorf("%w: manager is required", ErrNilManager)
	}

	return &Handler{
		manager: manager,
	}, nil
}

// RegisterRoutes 설정 수신 엔드포인트를 Gin 라우터에 등록합니다.
func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST(ConfigRoute, h.EnrollConfig)
}

// EnrollConfig 전달받은 설정 정보를 내부 저장소에 반영합니다.
func (h *Handler) EnrollConfig(ctx *gin.Context) {
	slog.Info(
		logConfigRegisterRequested,
		logAttrMethod,
		ctx.Request.Method,
		logAttrPath,
		ctx.Request.URL.Path,
		logAttrClientIP,
		ctx.ClientIP(),
	)

	var settings internalconfig.Settings
	if err := decodeSettings(ctx.Request.Body, &settings); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, responseapi.APIResponse{
			Success: false,
			Message: responseBindFailed,
		})
		return
	}

	configPayload, err := encodeSettingsJson(settings)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, responseapi.APIResponse{
			Success: false,
			Message: responseRegisterFailed,
		})
		return
	}

	slog.Info(
		logConfigRegisterPayload,
		logAttrConfig,
		configPayload,
		logAttrClientIP,
		ctx.ClientIP(),
	)

	if err := h.manager.Register(settings, ctx.GetHeader(requestHeaderHost), ctx.GetHeader(requestHeaderPort)); err != nil {
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

func decodeSettings(body io.Reader, settings *internalconfig.Settings) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(settings); err != nil {
		return fmt.Errorf("decode settings: %w", err)
	}

	var extra struct{}
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("decode settings trailing data")
	} else if err != io.EOF {
		return fmt.Errorf("decode settings trailing data: %w", err)
	}

	return nil
}

func encodeSettingsJson(settings internalconfig.Settings) (string, error) {
	payload, err := json.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("marshal settings log payload: %w", err)
	}

	return string(payload), nil
}
