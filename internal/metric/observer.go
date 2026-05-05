package metric

import (
	metricrecord "wintergate/internal/metric/record"

	"github.com/gin-gonic/gin"
)

// BuildRequestObserver HTTP 요청 메트릭을 기록하는 Gin observer를 생성합니다.
func BuildRequestObserver(recorder *metricrecord.Recorder) (gin.HandlerFunc, error) {
	if recorder == nil {
		return nil, ErrNilRecorder
	}

	return func(ctx *gin.Context) {
		if ctx.Request.URL.Path == Route {
			ctx.Next()
			return
		}

		done := recorder.RecordHTTP()
		ctx.Next()

		path := ctx.FullPath()
		if path == "" {
			path = routeGateway
		}

		done(metricrecord.RequestObservation{
			Method:     ctx.Request.Method,
			Path:       path,
			StatusCode: ctx.Writer.Status(),
		})
	}, nil
}
