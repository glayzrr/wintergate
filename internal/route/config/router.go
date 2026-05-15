package config

import (
	"strings"

	internalconfig "wintergate/internal/config"
	"wintergate/internal/utils"
)

// Router Router는 snapshot을 기반으로 라우트를 찾는 stateless 조회 객체입니다.
type Router struct{}

// NewRouter stateless 라우터를 생성합니다.
func NewRouter() *Router {
	return &Router{}
}

// RouteFor 지정한 HTTP method와 path에 연결된 라우팅 정보를 반환합니다.
func (r *Router) RouteFor(snapshot *internalconfig.Snapshot, method, path string) (RouteInfo, bool) {
	if r == nil || snapshot == nil {
		return RouteInfo{}, false
	}

	key := internalconfig.RouteKey{
		Method: utils.NormalizeHTTPMethod(method),
		Path:   utils.NormalizeHTTPPath(path),
	}
	if key.Method == "" || key.Path == "" {
		return RouteInfo{}, false
	}

	if route, found := snapshot.Routes[key]; found && !isWildcardPath(route.Path) {
		return routeInfoFromEntry(route), true
	}
	if route, found := snapshot.Routes[internalconfig.RouteKey{Method: "ALL", Path: key.Path}]; found && !isWildcardPath(route.Path) {
		return routeInfoFromEntry(route), true
	}
	for _, route := range snapshot.WildcardRoutes {
		if matchRouteEntry(route, key.Method, key.Path) {
			return routeInfoFromEntry(route), true
		}
	}

	return RouteInfo{}, false
}

func matchRouteEntry(route internalconfig.RouteEntry, method, path string) bool {
	if route.Method != "ALL" && route.Method != method {
		return false
	}

	routePath := strings.TrimSpace(route.Path)
	if isWildcardPath(routePath) {
		return strings.HasPrefix(path, strings.TrimSuffix(routePath, "/**"))
	}

	return routePath == path
}

func isWildcardPath(path string) bool {
	return strings.HasSuffix(strings.TrimSpace(path), "/**")
}

func routeInfoFromEntry(route internalconfig.RouteEntry) RouteInfo {
	return RouteInfo{
		ServiceName: route.ServiceName,
		Path:        route.Path,
		HttpMethod:  route.Method,
		Roles:       append([]string(nil), route.Roles...),
	}
}
