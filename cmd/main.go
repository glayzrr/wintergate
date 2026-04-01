package main

import (
	"fmt"
	"log/slog"
	"os"

	configapi "wintergate/api/config"
	gatewayapi "wintergate/api/gateway"

	"github.com/gin-gonic/gin"
)

const defaultListenAddress = ":1313"

var (
	runMain  = run
	logError = func(msg string, args ...any) {
		slog.Error(msg, args...)
	}
	exitProcess = os.Exit
	buildRouter = newRouter
	runServer   = func(router *gin.Engine, addr string) error {
		return router.Run(addr)
	}
)

func main() {
	if err := runMain(); err != nil {
		logError(logRunFailed, "error", err)
		exitProcess(1)
	}
}

func run() error {
	router, err := buildRouter()
	if err != nil {
		return fmt.Errorf("build router: %w", err)
	}

	if err := runServer(router, listenAddress()); err != nil {
		return fmt.Errorf("run gin server: %w", err)
	}

	return nil
}

func newRouter() (*gin.Engine, error) {
	registerer := configapi.NewRegisterer()
	configHandler, err := configapi.NewHandler(registerer)
	if err != nil {
		return nil, fmt.Errorf("create config handler: %w", err)
	}

	gatewayHandler, err := gatewayapi.NewHandlerWithAuthRegistry(registerer.AuthRegistry())
	if err != nil {
		return nil, fmt.Errorf("create gateway handler: %w", err)
	}

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
