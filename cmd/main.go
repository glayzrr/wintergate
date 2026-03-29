package main

import (
	"fmt"
	"log/slog"
	"os"

	configapi "sidecargo/api/config"
	authconfig "sidecargo/internal/auth/config"
	routeconfig "sidecargo/internal/route/config"

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
	authRegistry := authconfig.NewRegistry()
	routingRegistry := routeconfig.NewRegistry()

	registerer, err := configapi.NewRegisterer(authRegistry, routingRegistry)
	if err != nil {
		return fmt.Errorf("create config registerer: %w", err)
	}

	handler, err := configapi.NewHandler(registerer)
	if err != nil {
		return fmt.Errorf("create config handler: %w", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	handler.RegisterRoutes(router)

	if err := router.Run(listenAddress()); err != nil {
		return fmt.Errorf("run gin server: %w", err)
	}

	return nil
}

func listenAddress() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}

	return defaultListenAddress
}
