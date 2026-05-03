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
	router, err := newRouter()
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	if err := router.Run(listenAddress()); err != nil {
		return fmt.Errorf("run gin server: %w", err)
	}

	return nil
}

func newRouter() (*gin.Engine, error) {
	registerer := internalconfig.NewRegisterer()
	configHandler, err := configapi.NewHandler(registerer)
	if err != nil {
		return nil, fmt.Errorf("create config handler: %w", err)
	}

	routerTask := internalgateway.NewRouteTask(
		registerer.RouteRegistry(),
	)
	authenticateTask := internalgateway.NewAuthenticateTask(internalauth.NewDecoder(registerer.AuthRegistry()))
	authorizeTask := internalgateway.NewAuthorizeTask()
	transferTask := internalgateway.NewTransferTask()

	gatewayHandler := gatewayapi.NewHandler(internalgateway.NewOrchestrator(
		routerTask,
		authenticateTask,
		authorizeTask,
		transferTask,
	))

	router := gin.New()
	router.Use(gin.Recovery())
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
