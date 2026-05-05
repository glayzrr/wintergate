package metric

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler Prometheus 메트릭 엔드포인트를 담당합니다.
type Handler struct {
	handler http.Handler
}

// NewHandler Prometheus 메트릭 Handler를 생성합니다.
func NewHandler(gatherer prometheus.Gatherer) *Handler {
	if gatherer == nil {
		gatherer = NewRegistry()
	}

	return &Handler{
		handler: promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}),
	}
}

// RegisterRoutes Prometheus 메트릭 엔드포인트를 Gin 라우터에 등록합니다.
func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.GET(Route, h.Serve)
}

// Serve Prometheus 메트릭 응답을 작성합니다.
func (h *Handler) Serve(ctx *gin.Context) {
	h.handler.ServeHTTP(ctx.Writer, ctx.Request)
}
