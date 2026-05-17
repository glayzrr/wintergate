package configapi

import (
	"encoding/json"
	"errors"
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
	router.POST(ConfigApplyRoute, h.ApplyConfig)
	router.GET(ConfigForRoute, h.ConfigFor)
	router.DELETE(ConfigInstanceRoute, h.DeregisterInstance)
}

// ApplyConfig 전달받은 설정 정보를 내부 저장소에 반영합니다.
func (h *Handler) ApplyConfig(ctx *gin.Context) {
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

	if err := h.manager.Register(settings); err != nil {
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

// DeregisterInstance 전달받은 서비스 인스턴스를 중앙 설정 스냅샷에서 제거합니다.
func (h *Handler) DeregisterInstance(ctx *gin.Context) {
	slog.Info(
		logConfigDeregisterRequested,
		logAttrMethod,
		ctx.Request.Method,
		logAttrPath,
		ctx.Request.URL.Path,
		logAttrClientIP,
		ctx.ClientIP(),
	)

	var instance internalconfig.InstanceSettings
	if err := decodeInstanceSettings(ctx.Request.Body, &instance); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, responseapi.APIResponse{
			Success: false,
			Message: responseBindFailed,
		})
		return
	}

	configPayload, err := encodeInstanceSettingsJson(instance)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, responseapi.APIResponse{
			Success: false,
			Message: responseDeregisterFailed,
		})
		return
	}

	slog.Info(
		logConfigDeregisterPayload,
		logAttrConfig,
		configPayload,
		logAttrClientIP,
		ctx.ClientIP(),
	)

	if err := h.manager.DeregisterInstance(ctx.Param("serviceName"), instance); err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, internalconfig.ErrServiceNotFound) || errors.Is(err, internalconfig.ErrInstanceNotFound) {
			statusCode = http.StatusNotFound
		}
		ctx.AbortWithStatusJSON(statusCode, responseapi.APIResponse{
			Success: false,
			Message: responseDeregisterFailed,
		})
		return
	}

	ctx.JSON(http.StatusOK, responseapi.APIResponse{
		Success: true,
		Message: responseDeregisterSuccess,
	})
}

func (h *Handler) ConfigFor(ctx *gin.Context) {
	settings, found := h.manager.ConfigFor(ctx.Param("serviceName"))
	if !found {
		ctx.AbortWithStatusJSON(http.StatusNotFound, responseapi.APIResponse{
			Success: false,
			Message: responseConfigNotFound,
		})
		return
	}

	ctx.JSON(http.StatusOK, responseapi.APIResponse{
		Success: true,
		Message: responseConfigFound,
		Data:    settings,
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

func decodeInstanceSettings(body io.Reader, settings *internalconfig.InstanceSettings) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(settings); err != nil {
		return fmt.Errorf("decode instance settings: %w", err)
	}

	var extra struct{}
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("decode instance settings trailing data")
	} else if err != io.EOF {
		return fmt.Errorf("decode instance settings trailing data: %w", err)
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

func encodeInstanceSettingsJson(settings internalconfig.InstanceSettings) (string, error) {
	payload, err := json.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("marshal instance settings log payload: %w", err)
	}

	return string(payload), nil
}
