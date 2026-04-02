package gateway

import (
	"context"
	"wintergate/internal/route"
)

type RouteTask struct {
	router *route.Router
}

func NewRouteTask(router *route.Router) *RouteTask {
	return &RouteTask{
		router: router,
	}
}

func (t *RouteTask) Run(ctx context.Context, state *State) error {
	addr, err := t.router.Route(state.Request.Path)
	if err != nil {
		return err
	}

	state.Result.UpstreamURL = addr
	return nil
}
