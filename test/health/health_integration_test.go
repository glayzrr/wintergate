package health_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	authconfig "wintergate/internal/auth/config"
	internalconfig "wintergate/internal/config"
	internalhealth "wintergate/internal/health"
	"wintergate/internal/pool"
	routeconfig "wintergate/internal/route/config"
	"wintergate/test/harness"
)

const (
	healthServiceName = "order-service"
	healthRoutePath   = "/orders"
	healthPath        = "/healthz"
)

type healthRuntime struct {
	manager       *internalconfig.Manager
	healthStore   *internalhealth.Store
	healthManager *internalhealth.Manager
	loadBalancer  *routeconfig.LoadBalancer
}

func TestHealthCheckExcludesUnhealthyInstanceFromRouting(t *testing.T) {
	runtime := newHealthRuntime(t)

	unhealthyUpstream := newStatusUpstream(t, http.StatusInternalServerError)
	defer unhealthyUpstream.Close()
	healthyUpstream := newStatusUpstream(t, http.StatusOK)
	defer healthyUpstream.Close()

	unhealthyInstance := harness.InstanceFromURL(t, unhealthyUpstream.URL)
	healthyInstance := harness.InstanceFromURL(t, healthyUpstream.URL)
	runtime.register(t, healthServiceName, unhealthyInstance, defaultHealthSettings())
	runtime.register(t, healthServiceName, healthyInstance, defaultHealthSettings())

	unhealthyKey := snapshotInstance(t, runtime, unhealthyInstance).HealthKey
	waitUntil(t, "unhealthy instance excluded", func() bool {
		return !runtime.healthStore.IsRoutableKey(unhealthyKey)
	})

	for index := 0; index < 4; index++ {
		selected := runtime.nextInstance(t, healthServiceName)
		if selected.Host != healthyInstance.Host || selected.Port != healthyInstance.Port {
			t.Fatalf("selected instance = %s:%s, want healthy %s:%s", selected.Host, selected.Port, healthyInstance.Host, healthyInstance.Port)
		}
	}
}

func TestHealthCheckRecoversUnhealthyInstanceAndKeepsCheckingWithBackoff(t *testing.T) {
	runtime := newHealthRuntime(t)

	var statusCode atomic.Int64
	statusCode.Store(http.StatusInternalServerError)
	var checks atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == healthPath {
			checks.Add(1)
			w.WriteHeader(int(statusCode.Load()))
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	instance := harness.InstanceFromURL(t, upstream.URL)
	runtime.register(t, healthServiceName, instance, defaultHealthSettings())

	key := snapshotInstance(t, runtime, instance).HealthKey
	waitUntil(t, "instance marked unhealthy", func() bool {
		return !runtime.healthStore.IsRoutableKey(key)
	})
	checksAfterUnhealthy := checks.Load()
	waitUntil(t, "unhealthy instance keeps being checked", func() bool {
		return checks.Load() > checksAfterUnhealthy
	})

	if _, err := runtime.loadBalancer.NextInstance(runtime.manager.Settings(), healthServiceName); !errors.Is(err, routeconfig.ErrNoHealthyInstance) {
		t.Fatalf("NextInstance error = %v, want ErrNoHealthyInstance", err)
	}

	statusCode.Store(http.StatusOK)
	waitUntil(t, "instance recovered", func() bool {
		return runtime.healthStore.IsRoutableKey(key)
	})

	selected := runtime.nextInstance(t, healthServiceName)
	if selected.Host != instance.Host || selected.Port != instance.Port {
		t.Fatalf("selected instance = %s:%s, want recovered %s:%s", selected.Host, selected.Port, instance.Host, instance.Port)
	}
}

func TestHealthDisabledDoesNotStartChecksOrExcludeInstance(t *testing.T) {
	runtime := newHealthRuntime(t)

	var checks atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == healthPath {
			checks.Add(1)
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	instance := harness.InstanceFromURL(t, upstream.URL)
	settings := defaultHealthSettings()
	disabled := false
	settings.Enabled = &disabled
	runtime.register(t, healthServiceName, instance, settings)

	time.Sleep(50 * time.Millisecond)

	selected := runtime.nextInstance(t, healthServiceName)
	if selected.Host != instance.Host || selected.Port != instance.Port {
		t.Fatalf("selected instance = %s:%s, want disabled-health instance %s:%s", selected.Host, selected.Port, instance.Host, instance.Port)
	}
	if checks.Load() != 0 {
		t.Fatalf("health checks = %d, want 0 when health disabled", checks.Load())
	}
}

func TestDeregisterRemovesInstanceFromRoutingAndHealthTargets(t *testing.T) {
	runtime := newHealthRuntime(t)

	firstUpstream := newStatusUpstream(t, http.StatusInternalServerError)
	defer firstUpstream.Close()
	secondUpstream := newStatusUpstream(t, http.StatusOK)
	defer secondUpstream.Close()

	firstInstance := harness.InstanceFromURL(t, firstUpstream.URL)
	secondInstance := harness.InstanceFromURL(t, secondUpstream.URL)
	runtime.register(t, healthServiceName, firstInstance, defaultHealthSettings())
	runtime.register(t, healthServiceName, secondInstance, defaultHealthSettings())

	firstKey := snapshotInstance(t, runtime, firstInstance).HealthKey
	waitUntil(t, "first instance unhealthy before deregister", func() bool {
		return !runtime.healthStore.IsRoutableKey(firstKey)
	})

	if err := runtime.manager.DeregisterInstance(healthServiceName, firstInstance); err != nil {
		t.Fatalf("DeregisterInstance returned error: %v", err)
	}
	if !runtime.healthStore.IsRoutableKey(firstKey) {
		t.Fatal("deregistered instance health state should be removed")
	}

	for index := 0; index < 4; index++ {
		selected := runtime.nextInstance(t, healthServiceName)
		if selected.Host != secondInstance.Host || selected.Port != secondInstance.Port {
			t.Fatalf("selected instance = %s:%s, want remaining %s:%s", selected.Host, selected.Port, secondInstance.Host, secondInstance.Port)
		}
	}

	if err := runtime.manager.DeregisterInstance(healthServiceName, secondInstance); err != nil {
		t.Fatalf("DeregisterInstance returned error for second instance: %v", err)
	}
	if _, err := runtime.loadBalancer.NextInstance(runtime.manager.Settings(), healthServiceName); !errors.Is(err, routeconfig.ErrNoHealthyInstance) {
		t.Fatalf("NextInstance error = %v, want ErrNoHealthyInstance after all instances deregistered", err)
	}
}

func TestHealthCheckUsesConfiguredPath(t *testing.T) {
	runtime := newHealthRuntime(t)

	const customPath = "/internal/live"
	var customPathChecks atomic.Int64
	var unexpectedPathChecks atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case customPath:
			customPathChecks.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			unexpectedPathChecks.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer upstream.Close()

	settings := defaultHealthSettings()
	settings.Path = customPath
	instance := harness.InstanceFromURL(t, upstream.URL)
	runtime.register(t, healthServiceName, instance, settings)

	waitUntil(t, "custom health path checked", func() bool {
		return customPathChecks.Load() > 0
	})
	if unexpectedPathChecks.Load() != 0 {
		t.Fatalf("unexpected path checks = %d, want 0", unexpectedPathChecks.Load())
	}
}

func TestHealthCheckStatusCodePolicy(t *testing.T) {
	runtime := newHealthRuntime(t)

	healthyUpstream := newStatusUpstream(t, http.StatusNotModified)
	defer healthyUpstream.Close()
	unhealthyUpstream := newStatusUpstream(t, http.StatusNotFound)
	defer unhealthyUpstream.Close()

	healthyInstance := harness.InstanceFromURL(t, healthyUpstream.URL)
	unhealthyInstance := harness.InstanceFromURL(t, unhealthyUpstream.URL)
	runtime.register(t, healthServiceName, healthyInstance, defaultHealthSettings())
	runtime.register(t, healthServiceName, unhealthyInstance, defaultHealthSettings())

	healthyKey := snapshotInstance(t, runtime, healthyInstance).HealthKey
	unhealthyKey := snapshotInstance(t, runtime, unhealthyInstance).HealthKey
	waitUntil(t, "4xx instance excluded", func() bool {
		return !runtime.healthStore.IsRoutableKey(unhealthyKey)
	})
	if !runtime.healthStore.IsRoutableKey(healthyKey) {
		t.Fatal("3xx health response should remain routable")
	}

	for index := 0; index < 4; index++ {
		selected := runtime.nextInstance(t, healthServiceName)
		if selected.Host != healthyInstance.Host || selected.Port != healthyInstance.Port {
			t.Fatalf("selected instance = %s:%s, want 3xx-healthy %s:%s", selected.Host, selected.Port, healthyInstance.Host, healthyInstance.Port)
		}
	}
}

func TestHealthCheckTimeoutExcludesInstance(t *testing.T) {
	runtime := newHealthRuntime(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == healthPath {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	settings := defaultHealthSettings()
	settings.Timeout = "10ms"
	instance := harness.InstanceFromURL(t, upstream.URL)
	runtime.register(t, healthServiceName, instance, settings)

	key := snapshotInstance(t, runtime, instance).HealthKey
	waitUntil(t, "timeout instance excluded", func() bool {
		return !runtime.healthStore.IsRoutableKey(key)
	})
	if _, err := runtime.loadBalancer.NextInstance(runtime.manager.Settings(), healthServiceName); !errors.Is(err, routeconfig.ErrNoHealthyInstance) {
		t.Fatalf("NextInstance error = %v, want ErrNoHealthyInstance after health timeout", err)
	}
}

func TestHealthStoreIgnoresStaleGenerationUpdates(t *testing.T) {
	store := internalhealth.NewStore()
	const key = "order-service|http|127.0.0.1|8080"

	store.SetUnknown(key, 2)
	if _, updated := store.UpdateStatus(key, 1, internalhealth.StatusUnhealthy, 1, 0, nil); updated {
		t.Fatal("UpdateStatus updated stale generation, want ignored")
	}
	if !store.IsRoutableKey(key) {
		t.Fatal("stale unhealthy update made key unroutable")
	}

	if _, updated := store.UpdateStatus(key, 2, internalhealth.StatusUnhealthy, 1, 0, nil); !updated {
		t.Fatal("UpdateStatus ignored current generation")
	}
	if store.IsRoutableKey(key) {
		t.Fatal("current unhealthy update did not make key unroutable")
	}

	store.Delete(key)
	if !store.IsRoutableKey(key) {
		t.Fatal("deleted key should fall back to routable unknown state")
	}
}

func newHealthRuntime(t *testing.T) *healthRuntime {
	t.Helper()

	manager := internalconfig.NewManager()
	authStore := authconfig.NewStore()
	healthStore := internalhealth.NewStore()
	healthManager := internalhealth.NewManager(healthStore)
	loadBalancer := routeconfig.NewLoadBalancer(healthStore)
	poolStore := pool.NewStore()

	manager.AddValidator(routeconfig.NewValidator())
	manager.AddValidator(authStore)
	manager.AddValidator(poolStore)
	manager.AddSnapshotListener(healthManager)

	t.Cleanup(func() {
		healthManager.OnSnapshotCommitted(&internalconfig.Snapshot{
			Services: make(map[string]internalconfig.ServiceSettings),
			Routes:   make(map[internalconfig.RouteKey]internalconfig.RouteEntry),
		})
	})

	return &healthRuntime{
		manager:       manager,
		healthStore:   healthStore,
		healthManager: healthManager,
		loadBalancer:  loadBalancer,
	}
}

func (r *healthRuntime) register(t *testing.T, serviceName string, instance internalconfig.InstanceSettings, healthSettings *internalconfig.HealthSettings) {
	t.Helper()

	if err := r.manager.Register(harness.ServiceSettings(
		serviceName,
		instance,
		[]internalconfig.EndpointSettings{
			{
				Path:   healthRoutePath,
				Method: http.MethodGet,
			},
		},
		func(settings *internalconfig.Settings) {
			settings.Health = healthSettings
		},
	)); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
}

func (r *healthRuntime) nextInstance(t *testing.T, serviceName string) internalconfig.InstanceSettings {
	t.Helper()

	instance, err := r.loadBalancer.NextInstance(r.manager.Settings(), serviceName)
	if err != nil {
		t.Fatalf("NextInstance returned error: %v", err)
	}

	return instance
}

func snapshotInstance(t *testing.T, runtime *healthRuntime, instance internalconfig.InstanceSettings) internalconfig.InstanceSettings {
	t.Helper()

	snapshot := runtime.manager.Settings()
	service, found := snapshot.Services[healthServiceName]
	if !found {
		t.Fatalf("service %q not found in snapshot", healthServiceName)
	}

	for _, candidate := range service.Instances {
		if candidate.Host == instance.Host && candidate.Port == instance.Port {
			return candidate
		}
	}

	t.Fatalf("instance %s:%s not found in snapshot", instance.Host, instance.Port)
	return internalconfig.InstanceSettings{}
}

func defaultHealthSettings() *internalconfig.HealthSettings {
	enabled := true

	return &internalconfig.HealthSettings{
		Enabled:          &enabled,
		Path:             healthPath,
		Interval:         "10ms",
		Timeout:          "50ms",
		Jitter:           "0s",
		MaxBackoff:        "20ms",
		FailureThreshold: 1,
		SuccessThreshold: 1,
	}
}

func newStatusUpstream(t *testing.T, statusCode int) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == healthPath {
			w.WriteHeader(statusCode)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
}

func waitUntil(t *testing.T, name string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", name)
}
