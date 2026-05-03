package gateway

import (
	"context"
	"fmt"
	"net"
	"strings"

	"wintergate/internal/pool"
)

// TransferTask 인증과 인가를 통과한 요청을 업스트림 서비스로 전달합니다.
type TransferTask struct{}

// NewTransferTask 업스트림 전달용 TransferTask를 생성합니다.
func NewTransferTask() *TransferTask {
	return &TransferTask{}
}

// Run 현재 요청을 service host와 port로 전달하고 업스트림 응답을 클라이언트에 기록합니다.
func (t *TransferTask) Run(_ context.Context, state *State) error {
	if state.Request.ResponseWriter == nil {
		return fmt.Errorf("%w: response writer is required", ErrInvalidRequest)
	}
	if state.Request.HTTPRequest == nil {
		return fmt.Errorf("%w: http request is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(state.Request.Service) == "" {
		return fmt.Errorf("%w: service is required", ErrInvalidRequest)
	}

	upstreamHost, err := upstreamHost(state.Request.Host, state.Request.Port)
	if err != nil {
		return err
	}

	if err := pool.HandleRequest(state.Request.Service, upstreamHost, state.Request.ResponseWriter, state.Request.HTTPRequest); err != nil {
		return fmt.Errorf("handle upstream request: %w", err)
	}

	return nil
}

func upstreamHost(host, port string) (string, error) {
	trimmedHost := strings.TrimSpace(host)
	if trimmedHost == "" {
		return "", fmt.Errorf("%w: host is required", ErrInvalidRequest)
	}

	trimmedPort := strings.TrimSpace(port)
	if trimmedPort == "" {
		return "", fmt.Errorf("%w: port is required", ErrInvalidRequest)
	}

	return "http://" + net.JoinHostPort(trimmedHost, trimmedPort), nil
}
