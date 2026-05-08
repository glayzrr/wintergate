package main

import (
	"fmt"
	"log/slog"
	"os"
	configapi "wintergate/api/config"
	gatewayapi "wintergate/api/gateway"
	internalauth "wintergate/internal/auth"
	internalconfig "wintergate/internal/config"
	internalgateway "wintergate/internal/gateway"
	internalmetric "wintergate/internal/metric"
	metricrecord "wintergate/internal/metric/record"
	internaltrace "wintergate/internal/trace"

	"github.com/gin-gonic/gin"
)

const defaultListenAddress = ":1313"

func main() {
	if err := run(); err != nil {
		slog.Error(logRunFailed, "error", err)
		os.Exit(1)
	}
}

func run() error {
	// 서버 실행 전에 라우터와 의존성을 구성합니다.
	router, err := newRouter()
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	// 환경 변수 또는 기본 주소를 사용해 Gin 서버를 시작합니다.
	if err := router.Run(listenAddress()); err != nil {
		return fmt.Errorf("run gin server: %w", err)
	}

	return nil
}

func newRouter() (*gin.Engine, error) {
	// 설정 등록기는 config, route, auth 런타임 상태를 함께 관리합니다.
	registerer := internalconfig.NewRegisterer()
	configHandler, err := configapi.NewHandler(registerer)
	if err != nil {
		return nil, fmt.Errorf("create config handler: %w", err)
	}

	// 메트릭 레지스트리와 recorder를 같은 라우터 생명주기 안에서 공유합니다.
	metricRegistry := internalmetric.NewRegistry()
	metricRecorder := metricrecord.NewRecorder(metricRegistry)
	metricObserver, err := internalmetric.BuildRequestObserver(metricRecorder)
	if err != nil {
		return nil, fmt.Errorf("create metric observer: %w", err)
	}
	metricHandler := internalmetric.NewHandler(metricRegistry)

	// 게이트웨이 요청은 라우팅, 인증, 인가, 업스트림 전송 순서로 처리합니다.
	routerTask := internalgateway.NewRouteTask(registerer.RouteRegistry())
	traceTask := internalgateway.NewTraceTask(internaltrace.NewGenerator())
	authenticateTask := internalgateway.NewAuthenticateTask(internalauth.NewDecoder(registerer.AuthRegistry()))
	authorizeTask := internalgateway.NewAuthorizeTask()
	transferTask := internalgateway.NewTransferTask(metricRecorder)

	gatewayHandler := gatewayapi.NewHandler(internalgateway.NewOrchestrator(
		routerTask,
		traceTask,
		authenticateTask,
		authorizeTask,
		transferTask,
	))

	router := gin.New()
	router.Use(metricObserver)
	router.Use(gin.Recovery())

	// 명시 라우트를 먼저 등록하고, 마지막에 NoRoute 기반 게이트웨이 진입점을 등록합니다.
	metricHandler.RegisterRoutes(router)
	configHandler.RegisterRoutes(router)
	gatewayHandler.RegisterRoutes(router)

	return router, nil
}

func listenAddress() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}

	return defaultListenAddress
}
