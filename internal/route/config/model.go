package config

// Entry 하나의 URL 경로와 대상 서비스 매핑을 표현합니다.
type Entry struct {
	Path       string
	Service    string
	HttpMethod string
	Roles      []string
}

// RuntimeConfig 라우팅 런타임 설정과 엔트리를 함께 보관합니다.
type RuntimeConfig struct {
	Entries []Entry
}

// RegistryRouteInfo 하나의 서비스에 속한 라우팅 정보를 표현합니다.
type RegistryRouteInfo struct {
	Path       string
	HttpMethod string
	Roles      []string
}

// RouteInfo 외부로 반환하는 라우팅 정보를 표현합니다.
type RouteInfo struct {
	Path       string
	HttpMethod string
	Roles      []string
}

func toRouteInfos(routeInfos []RegistryRouteInfo) []RouteInfo {
	if len(routeInfos) == 0 {
		return nil
	}

	converted := make([]RouteInfo, 0, len(routeInfos))
	for _, routeInfo := range routeInfos {
		converted = append(converted, RouteInfo{
			Path:       routeInfo.Path,
			HttpMethod: routeInfo.HttpMethod,
			Roles:      append([]string(nil), routeInfo.Roles...),
		})
	}

	return converted
}
