package route

import (
	"fmt"
	"net"
	"strconv"
	routeconfig "wintergate/internal/route/config"
)

type Router struct {
	registry *routeconfig.Registry
}

func NewRouter(registry *routeconfig.Registry) *Router {
	return &Router{registry: registry}
}

func (r *Router) ReplaceRegistry(registry *routeconfig.Registry) error {
	if registry == nil {
		return fmt.Errorf("%w: registry is required", ErrNilRegistry)
	}

	r.registry = registry

	return nil
}

func (r *Router) Route(path string) (string, error) {
	if r.registry == nil {
		return "", fmt.Errorf("%w: registry is required", ErrNilRegistry)
	}

	routeInfo, found := r.registry.Route(path)
	if !found {
		return "", fmt.Errorf("%w: %s", ErrServiceNotFound, path)
	}

	return net.JoinHostPort(routeInfo.ClientIP, strconv.Itoa(routeInfo.Port)) + path, nil
}
