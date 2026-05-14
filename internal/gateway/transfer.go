package gateway

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"wintergate/internal/pool"
)

// PoolProvider 현재 트래픽 상태에 맞는 pool 할당 결과를 제공합니다.
type PoolProvider interface {
	AssignmentFor(status pool.Status) pool.Assignment
}

// PoolForwarder 선택된 pool 할당 결과로 업스트림 요청을 전달합니다.
type PoolForwarder interface {
	Handle(request pool.ForwardRequest) error
}

// TrafficRecorder 서비스별 트래픽 상태를 기록하고 조회합니다.
type TrafficRecorder interface {
	Start(configKey string) pool.DoneFunc
	StatusFor(configKey string) (pool.Status, error)
}

// TransferTask 인증과 인가를 통과한 요청을 업스트림 서비스로 전달합니다.
type TransferTask struct {
	provider  PoolProvider
	forwarder PoolForwarder
	recorder  TrafficRecorder
}

// NewTransferTask 업스트림 전달용 TransferTask를 생성합니다.
func NewTransferTask(provider PoolProvider, forwarder PoolForwarder, recorder TrafficRecorder) *TransferTask {
	return &TransferTask{
		provider:  provider,
		forwarder: forwarder,
		recorder:  recorder,
	}
}

// Run 현재 요청을 업스트림 host와 port로 전달하고 업스트림 응답을 클라이언트에 기록합니다.
func (t *TransferTask) Run(_ context.Context, state *State) error {
	// RouteTask가 식별한 서비스 이름이 있어야 풀 정책과 트래픽 기록을 적용할 수 있습니다.
	if strings.TrimSpace(state.Request.ServiceName) == "" {
		return fmt.Errorf("%w: service-name is required", ErrInvalidRequest)
	}

	if state.Route == nil {
		return fmt.Errorf("%w: route is required", ErrInvalidRequest)
	}

	instance := state.Route.Instance
	upstreamHost, err := upstreamHost(instance.Scheme, instance.Host, instance.Port)
	if err != nil {
		return err
	}

	if t.recorder == nil {
		return fmt.Errorf("%w: traffic recorder is required", ErrNilTrafficRecorder)
	}

	// 요청 시작과 종료 시점을 기록해 서비스 이름별 트래픽 상태를 갱신합니다.
	doneFunc := t.recorder.Start(state.Request.ServiceName)
	defer doneFunc()

	status, err := t.recorder.StatusFor(state.Request.ServiceName)
	if err != nil {
		return fmt.Errorf("read pool status: %w", err)
	}

	assignment := t.provider.AssignmentFor(status)

	// 커넥션 풀 정책을 적용해 업스트림으로 요청을 전달하고 응답을 클라이언트에 씁니다.
	if err := t.forwarder.Handle(pool.ForwardRequest{
		Address:    upstreamHost,
		Writer:     state.Request.ResponseWriter,
		Request:    state.Request.HTTPRequest,
		Assignment: assignment,
	}); err != nil {
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
