package gateway

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	metricrecord "wintergate/internal/metric/record"
	"wintergate/internal/pool"
)

type PoolProvider interface {
	DecisionFor(status pool.Status) pool.Assignment
}

// TransferTask 인증과 인가를 통과한 요청을 업스트림 서비스로 전달합니다.
type TransferTask struct {
	recorder *metricrecord.Recorder
	provider PoolProvider
}

// NewTransferTask 업스트림 전달용 TransferTask를 생성합니다.
func NewTransferTask(recorder *metricrecord.Recorder, provider PoolProvider) *TransferTask {
	return &TransferTask{
		recorder: recorder,
		provider: provider,
	}
}

// Run 현재 요청을 업스트림 host와 port로 전달하고 업스트림 응답을 클라이언트에 기록합니다.
func (t *TransferTask) Run(_ context.Context, state *State) error {
	// 업스트림 응답을 그대로 기록해야 하므로 원본 ResponseWriter가 필요합니다.
	if state.Request.ResponseWriter == nil {
		return fmt.Errorf("%w: response writer is required", ErrInvalidRequest)
	}
	// 원본 요청을 복제해 업스트림으로 전달해야 하므로 HTTP 요청이 필요합니다.
	if state.Request.HTTPRequest == nil {
		return fmt.Errorf("%w: http request is required", ErrInvalidRequest)
	}
	// RouteTask가 식별한 서비스 이름이 있어야 풀 정책과 트래픽 기록을 적용할 수 있습니다.
	if strings.TrimSpace(state.Request.ServiceName) == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidRequest)
	}
	if t.provider == nil {
		return fmt.Errorf("%w: pool provider is required", ErrInvalidRequest)
	}
	if state.Route == nil {
		return fmt.Errorf("%w: route is required", ErrInvalidRequest)
	}

	instance := state.Route.Instance
	upstreamHost, err := upstreamHost(instance.Scheme, instance.Host, instance.Port)
	if err != nil {
		return err
	}

	// 요청 시작과 종료 시점을 기록해 서비스 이름별 트래픽 상태를 갱신합니다.
	doneFunc := pool.StartRecord(state.Request.ServiceName)
	defer doneFunc()

	status, err := pool.StatusFor(state.Request.ServiceName)
	if err != nil {
		return fmt.Errorf("read pool status: %w", err)
	}

	decision := t.provider.DecisionFor(status)

	// 커넥션 풀 정책을 적용해 업스트림으로 요청을 전달하고 응답을 클라이언트에 씁니다.
	if err := pool.HandleRequest(upstreamHost, state.Request.ResponseWriter, state.Request.HTTPRequest, decision, t.recorder); err != nil {
		return fmt.Errorf("handle upstream request: %w", err)
	}

	return nil
}

func upstreamHost(scheme, host, port string) (string, error) {
	trimmedScheme := strings.ToLower(strings.TrimSpace(scheme))
	if trimmedScheme == "" {
		return "", fmt.Errorf("%w: scheme is required", ErrInvalidRequest)
	}
	if trimmedScheme != "http" && trimmedScheme != "https" {
		return "", fmt.Errorf("%w: scheme is invalid", ErrInvalidRequest)
	}

	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		return "", fmt.Errorf("%w: host is required", ErrInvalidRequest)
	}
	if strings.Contains(trimmedHost, "://") || strings.ContainsAny(trimmedHost, "/?#@") {
		return "", fmt.Errorf("%w: host must not include scheme, path, query, or user info", ErrInvalidRequest)
	}

	trimmedPort := strings.TrimSpace(port)
	if trimmedPort == "" {
		return "", fmt.Errorf("%w: port is required", ErrInvalidRequest)
	}
	parsedPort, err := strconv.Atoi(trimmedPort)
	if err != nil {
		return "", fmt.Errorf("%w: parse port: %w", ErrInvalidRequest, err)
	}
	if parsedPort <= 0 || parsedPort > 65535 {
		return "", fmt.Errorf("%w: port is invalid", ErrInvalidRequest)
	}

	return trimmedScheme + "://" + net.JoinHostPort(trimmedHost, strconv.Itoa(parsedPort)), nil
}
