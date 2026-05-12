package main

import (
	"fmt"
	"log/slog"
	"os"
	configapi "wintergate/api/config"
	gatewayapi "wintergate/api/gateway"
	internalauth "wintergate/internal/auth"
	authconfig "wintergate/internal/auth/config"
	internalconfig "wintergate/internal/config"
	internalgateway "wintergate/internal/gateway"
	internalmetric "wintergate/internal/metric"
	metricrecord "wintergate/internal/metric/record"
	"wintergate/internal/pool"
	routeconfig "wintergate/internal/route/config"
	internaltrace "wintergate/internal/trace"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
)

const (
	defaultListenAddress  = ":1313"
	defaultPoolConfigPath = "config/config.yml"
)

func main() {
	if err := run(); err != nil {
		slog.Error(logRunFailed, "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := pool.LoadConfig(defaultPoolConfigPath); err != nil {
		return fmt.Errorf("load pool config: %w", err)
	}

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
	// 설정 Manager는 각 런타임 저장소에 서비스 설정을 브로드캐스팅합니다.
	manager := internalconfig.NewManager()
	authStore := authconfig.NewStore()
	routeStore := routeconfig.NewStore()
	poolStore := pool.NewStore()

	manager.AddApplier(routeStore)
	manager.AddApplier(authStore)
	manager.AddApplier(poolStore)

	configHandler, err := configapi.NewHandler(manager)
	if err != nil {
		return nil, fmt.Errorf("create config handler: %w", err)
	}

	// 메트릭 레지스트리와 recorder를 같은 라우터 생명주기 안에서 공유합니다.
	metricRegistry := internalmetric.NewRegistry()
	metricRecorder := metricrecord.NewRecorder(metricRegistry)
	poolCoordinator := pool.NewCoordinator()
	poolForwarder := pool.NewForwarder(poolCoordinator, metricRecorder)
	metricObserver, err := internalmetric.BuildRequestObserver(metricRecorder)
	if err != nil {
		return nil, fmt.Errorf("create metric observer: %w", err)
	}
	metricHandler := internalmetric.NewHandler(metricRegistry)

	// 게이트웨이 요청은 라우팅, 인증, 인가, 업스트림 전송 순서로 처리합니다.
	routerTask := internalgateway.NewRouteTask(routeStore)
	traceTask := internalgateway.NewTraceTask(internaltrace.NewGenerator())
	authenticateTask := internalgateway.NewAuthenticateTask(internalauth.NewDecoder(authStore))
	authorizeTask := internalgateway.NewAuthorizeTask()
	transferTask := internalgateway.NewTransferTask(poolStore, poolForwarder)

	gatewayHandler := gatewayapi.NewHandler(internalgateway.NewOrchestrator(
		routerTask,
		traceTask,
		authenticateTask,
		authorizeTask,
		transferTask,
	))

	router := gin.New()
	pprof.Register(router)

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
