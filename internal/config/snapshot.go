package config

import "fmt"

func buildCandidate(current *Snapshot, serviceName string, settings Settings) (*Snapshot, error) {
	builder := newSnapshotBuilder(current)
	if err := builder.replaceService(serviceName, settings); err != nil {
		return nil, err
	}

	return builder.snapshot(), nil
}

func buildDeregisterCandidate(current *Snapshot, serviceName string, instance InstanceSettings) (*Snapshot, error) {
	builder := newSnapshotBuilder(current)
	if err := builder.removeServiceInstance(serviceName, instance); err != nil {
		return nil, err
	}

	return builder.snapshot(), nil
}

type snapshotBuilder struct {
	candidate *Snapshot
}

func newSnapshotBuilder(current *Snapshot) *snapshotBuilder {
	return &snapshotBuilder{
		candidate: copySnapshotIndex(current),
	}
}

func (b *snapshotBuilder) replaceService(serviceName string, settings Settings) error {
	serviceSettings, routes, err := b.buildService(serviceName, settings)
	if err != nil {
		return err
	}

	b.removeServiceRoutes(serviceName)
	if err := b.addServiceRoutes(serviceName, routes); err != nil {
		return err
	}

	b.candidate.Services[serviceName] = serviceSettings

	return nil
}

func (b *snapshotBuilder) buildService(serviceName string, settings Settings) (ServiceSettings, []RouteEntry, error) {
	endpoints, routes, err := routeEntriesFromEndpointSettings(serviceName, settings.Endpoints)
	if err != nil {
		return ServiceSettings{}, nil, err
	}
	settings.Endpoints = endpoints

	serviceSettings, found := b.candidate.Services[serviceName]
	if !found {
		return convertSettings(serviceName, settings), routes, nil
	}

	serviceSettings.Global = settings.Global.Clone()
	serviceSettings.Health = settings.Health.Clone()
	serviceSettings.Threshold = settings.Threshold.Clone()
	serviceSettings.Endpoints = EndpointSettingsList(settings.Endpoints).Clone()
	serviceSettings.Instances = upsertInstanceSettings(
		append([]InstanceSettings(nil), serviceSettings.Instances...),
		*settings.Instance.Clone(),
	)

	return serviceSettings, routes, nil
}

func (b *snapshotBuilder) removeServiceInstance(serviceName string, instance InstanceSettings) error {
	serviceSettings, found := b.candidate.Services[serviceName]
	if !found {
		return fmt.Errorf("%w: %s", ErrServiceNotFound, serviceName)
	}

	instances := make([]InstanceSettings, 0, len(serviceSettings.Instances))
	removed := false
	for _, registeredInstance := range serviceSettings.Instances {
		if registeredInstance.Host == instance.Host && registeredInstance.Port == instance.Port {
			removed = true
			continue
		}

		instances = append(instances, registeredInstance)
	}
	if !removed {
		return fmt.Errorf("%w: %s:%s", ErrInstanceNotFound, instance.Host, instance.Port)
	}

	serviceSettings.Instances = instances
	b.candidate.Services[serviceName] = serviceSettings

	return nil
}

func (b *snapshotBuilder) removeServiceRoutes(serviceName string) {
	for key, route := range b.candidate.Routes {
		if route.ServiceName == serviceName {
			delete(b.candidate.Routes, key)
		}
	}

	routes := b.candidate.WildcardRoutes
	nextIndex := 0
	for _, route := range routes {
		if route.ServiceName != serviceName {
			routes[nextIndex] = route
			nextIndex++
		}
	}
	for index := nextIndex; index < len(routes); index++ {
		routes[index] = RouteEntry{}
	}
	b.candidate.WildcardRoutes = routes[:nextIndex]
}

func (b *snapshotBuilder) addServiceRoutes(serviceName string, routes []RouteEntry) error {
	for _, route := range routes {
		key := RouteKey{Method: route.Method, Path: route.Path}
		existingRoute, found := b.candidate.Routes[key]
		if found && existingRoute.ServiceName != serviceName {
			return fmt.Errorf(
				"%w: route %s %s already belongs to service %q",
				ErrInvalidSettings,
				route.Method,
				route.Path,
				existingRoute.ServiceName,
			)
		}

		b.candidate.Routes[key] = route
		if isWildcardPath(route.Path) {
			b.candidate.WildcardRoutes = append(b.candidate.WildcardRoutes, route)
		}
	}

	return nil
}

func (b *snapshotBuilder) snapshot() *Snapshot {
	b.candidate.Revision++

	return b.candidate
}

func copySnapshotIndex(snapshot *Snapshot) *Snapshot {
	if snapshot == nil {
		return &Snapshot{
			Services: make(map[string]ServiceSettings),
			Routes:   make(map[RouteKey]RouteEntry),
		}
	}

	cloned := &Snapshot{
		Revision:       snapshot.Revision,
		Services:       make(map[string]ServiceSettings, len(snapshot.Services)),
		Routes:         make(map[RouteKey]RouteEntry, len(snapshot.Routes)),
		WildcardRoutes: make([]RouteEntry, 0, len(snapshot.WildcardRoutes)),
	}
	for serviceName, settings := range snapshot.Services {
		cloned.Services[serviceName] = settings
	}
	for key, route := range snapshot.Routes {
		cloned.Routes[key] = route
	}
	cloned.WildcardRoutes = append(cloned.WildcardRoutes, snapshot.WildcardRoutes...)

	return cloned
}

func convertSettings(serviceName string, settings Settings) ServiceSettings {
	return ServiceSettings{
		ServiceName: serviceName,
		Global:      settings.Global.Clone(),
		Health:      settings.Health.Clone(),
		Threshold:   settings.Threshold.Clone(),
		Endpoints:   EndpointSettingsList(settings.Endpoints).Clone(),
		Instances:   []InstanceSettings{*settings.Instance.Clone()},
	}
}

func upsertInstanceSettings(instances []InstanceSettings, next InstanceSettings) []InstanceSettings {
	for index, instance := range instances {
		if instance.Host == next.Host && instance.Port == next.Port {
			instances[index] = next
			return instances
		}
	}

	return append(instances, next)
}
