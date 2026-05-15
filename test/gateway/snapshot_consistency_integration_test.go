package gateway_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	internalauth "wintergate/internal/auth"
	authconfig "wintergate/internal/auth/config"
	internalconfig "wintergate/internal/config"
	internalgateway "wintergate/internal/gateway"
	"wintergate/internal/pool"
	routeconfig "wintergate/internal/route/config"
)

func TestGatewayUsesCapturedSnapshotWhenConfigChangesDuringRouting(t *testing.T) {
	manager, _, _ := newSnapshotRuntime(t, snapshotServiceSettings(
		"/orders",
		"old-secret",
		"127.0.0.1",
		"8080",
		[]string{"USER"},
		nil,
	))

	router := routeconfig.NewRouter()
	routeTask := internalgateway.NewRouteTask(
		manager,
		routerFunc(func(snapshot *internalconfig.Snapshot, method, path string) (routeconfig.RouteInfo, bool) {
			if snapshot == nil || snapshot.Revision != 1 {
				t.Fatalf("router snapshot revision = %v, want 1", snapshotRevision(snapshot))
			}

			if err := manager.Register(snapshotServiceSettings(
				"/payments",
				"new-secret",
				"127.0.0.2",
				"9090",
				[]string{"USER"},
				nil,
			)); err != nil {
				t.Fatalf("Register during routing returned error: %v", err)
			}

			return router.RouteFor(snapshot, method, path)
		}),
		routeconfig.NewLoadBalancer(),
	)

	state := &internalgateway.State{
		Request: internalgateway.Request{
			Method: http.MethodGet,
			Path:   "/orders",
		},
	}
	if err := routeTask.Run(context.Background(), state); err != nil {
		t.Fatalf("RouteTask returned error: %v", err)
	}

	if manager.Settings().Revision != 2 {
		t.Fatalf("manager revision = %d, want 2", manager.Settings().Revision)
	}
	if state.Settings.Revision != 1 {
		t.Fatalf("state snapshot revision = %d, want 1", state.Settings.Revision)
	}
	if state.Route == nil || state.Route.Path != "/orders" {
		t.Fatalf("state.Route = %#v, want /orders route from captured snapshot", state.Route)
	}
	if state.Route.Instance.Host != "127.0.0.1" || state.Route.Instance.Port != "8080" {
		t.Fatalf("instance = %s:%s, want 127.0.0.1:8080", state.Route.Instance.Host, state.Route.Instance.Port)
	}
}

func TestGatewayAuthenticateUsesCapturedSnapshotAfterConfigCommit(t *testing.T) {
	manager, authStore, _ := newSnapshotRuntime(t, snapshotServiceSettings(
		"/orders",
		"old-secret",
		"127.0.0.1",
		"8080",
		[]string{"USER"},
		nil,
	))

	orchestrator := internalgateway.NewOrchestrator(
		internalgateway.NewRouteTask(manager, routeconfig.NewRouter(), routeconfig.NewLoadBalancer()),
		taskFunc(func(_ context.Context, state *internalgateway.State) error {
			if state.Settings == nil || state.Settings.Revision != 1 {
				t.Fatalf("state snapshot revision before auth = %v, want 1", snapshotRevision(state.Settings))
			}

			return manager.Register(snapshotServiceSettings(
				"/orders",
				"new-secret",
				"127.0.0.1",
				"8080",
				[]string{"USER"},
				nil,
			))
		}),
		internalgateway.NewAuthenticateTask(internalauth.NewDecoder(authStore)),
		taskFunc(func(_ context.Context, state *internalgateway.State) error {
			if state.Settings == nil || state.Settings.Revision != 1 {
				t.Fatalf("state snapshot revision after auth = %v, want 1", snapshotRevision(state.Settings))
			}
			return nil
		}),
	)

	err := orchestrator.Receive(context.Background(), internalgateway.Request{
		Method:              http.MethodGet,
		Path:                "/orders",
		AuthorizationHeader: "Bearer " + signedSnapshotToken(t, "old-secret"),
	})
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
	if manager.Settings().Revision != 2 {
		t.Fatalf("manager revision = %d, want 2", manager.Settings().Revision)
	}
}

func TestGatewayTransferUsesCapturedSnapshotAfterConfigCommit(t *testing.T) {
	manager, _, poolStore := newSnapshotRuntime(t, snapshotServiceSettings(
		"/orders",
		"old-secret",
		"127.0.0.1",
		"8080",
		nil,
		&internalconfig.ThresholdSettings{
			Hot: internalconfig.ThresholdPoint{InFlight: 1},
		},
	))
	forwarder := &recordingForwarder{}
	recorder := pool.NewRecorder()
	orchestrator := internalgateway.NewOrchestrator(
		internalgateway.NewRouteTask(manager, routeconfig.NewRouter(), routeconfig.NewLoadBalancer()),
		taskFunc(func(_ context.Context, state *internalgateway.State) error {
			if state.Settings == nil || state.Settings.Revision != 1 {
				t.Fatalf("state snapshot revision before transfer = %v, want 1", snapshotRevision(state.Settings))
			}

			return manager.Register(snapshotServiceSettings(
				"/orders",
				"new-secret",
				"127.0.0.2",
				"9090",
				nil,
				&internalconfig.ThresholdSettings{
					Hot: internalconfig.ThresholdPoint{InFlight: 100},
				},
			))
		}),
		internalgateway.NewTransferTask(poolStore, forwarder, recorder),
	)

	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	recorderWriter := httptest.NewRecorder()
	err := orchestrator.Receive(context.Background(), internalgateway.Request{
		Method:         request.Method,
		Path:           request.URL.Path,
		ResponseWriter: recorderWriter,
		HTTPRequest:    request,
	})
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}

	if forwarder.address != "http://127.0.0.1:8080" {
		t.Fatalf("forward address = %q, want old snapshot address", forwarder.address)
	}
	if !forwarder.assignment.Dedicated {
		t.Fatal("assignment is shared, want dedicated from old threshold")
	}
	if forwarder.assignment.Tier != pool.TierHot {
		t.Fatalf("assignment tier = %q, want %q", forwarder.assignment.Tier, pool.TierHot)
	}
	if manager.Settings().Revision != 2 {
		t.Fatalf("manager revision = %d, want 2", manager.Settings().Revision)
	}
}

func newSnapshotRuntime(t *testing.T, settings internalconfig.Settings) (*internalconfig.Manager, *authconfig.Store, *pool.Store) {
	t.Helper()

	manager := internalconfig.NewManager()
	authStore := authconfig.NewStore()
	poolStore := pool.NewStore()
	manager.AddValidator(routeconfig.NewValidator())
	manager.AddValidator(authStore)
	manager.AddValidator(poolStore)

	if err := manager.Register(settings); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	return manager, authStore, poolStore
}

func snapshotServiceSettings(path, secret, host, port string, roles []string, threshold *internalconfig.ThresholdSettings) internalconfig.Settings {
	return internalconfig.Settings{
		ServiceName: "order-service",
		Global: &internalconfig.GlobalSettings{
			Auth: &internalconfig.AuthSettings{
				JWTAlgorithm: "HS256",
				JWTAudience:  "wintergate",
				JWTClockSkew: "1m",
				JWTIssuer:    "auth-service",
				JWTSecret:    secret,
			},
		},
		Instance: &internalconfig.InstanceSettings{
			Scheme: "http",
			Host:   host,
			Port:   port,
		},
		Threshold: threshold,
		Endpoints: []internalconfig.EndpointSettings{
			{
				Path:   path,
				Method: http.MethodGet,
				Roles:  roles,
			},
		},
	}
}

func signedSnapshotToken(t *testing.T, secret string) string {
	t.Helper()

	now := time.Now().UTC()
	headerPayload, err := json.Marshal(map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("Marshal returned error for header: %v", err)
	}
	claimsPayload, err := json.Marshal(map[string]any{
		"aud": "wintergate",
		"exp": now.Add(time.Minute).Unix(),
		"iat": now.Unix(),
		"iss": "auth-service",
		"sub": "user-1",
	})
	if err != nil {
		t.Fatalf("Marshal returned error for claims: %v", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerPayload) + "." + base64.RawURLEncoding.EncodeToString(claimsPayload)
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(signingInput)); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func snapshotRevision(snapshot *internalconfig.Snapshot) uint64 {
	if snapshot == nil {
		return 0
	}

	return snapshot.Revision
}

type routerFunc func(*internalconfig.Snapshot, string, string) (routeconfig.RouteInfo, bool)

func (f routerFunc) RouteFor(snapshot *internalconfig.Snapshot, method, path string) (routeconfig.RouteInfo, bool) {
	return f(snapshot, method, path)
}

type taskFunc func(context.Context, *internalgateway.State) error

func (f taskFunc) Run(ctx context.Context, state *internalgateway.State) error {
	return f(ctx, state)
}

type recordingForwarder struct {
	address    string
	assignment pool.Assignment
}

func (f *recordingForwarder) Handle(request pool.ForwardRequest) error {
	f.address = request.Address
	f.assignment = request.Assignment
	return nil
}
