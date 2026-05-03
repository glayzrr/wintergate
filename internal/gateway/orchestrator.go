package gateway

import (
	"context"
	"fmt"
	"strings"
)

// Task 게이트웨이 요청 처리 중 개별 작업 단위를 정의합니다.
type Task interface {
	Run(ctx context.Context, state *State) error
}

// Orchestrator 게이트웨이 요청 처리 작업을 순차적으로 조율합니다.
type Orchestrator struct {
	tasks []Task
}

// NewOrchestrator 게이트웨이 요청 처리용 Orchestrator를 생성합니다.
func NewOrchestrator(tasks ...Task) *Orchestrator {
	return &Orchestrator{
		tasks: append([]Task(nil), tasks...),
	}
}

// Receive 게이트웨이로 들어온 요청에 대해 등록된 작업을 순차 실행합니다.
func (o *Orchestrator) Receive(ctx context.Context, request Request) error {
	trimmedService := strings.TrimSpace(request.Service)
	trimmedHost := strings.TrimSpace(request.Host)
	trimmedPort := strings.TrimSpace(request.Port)
	trimmedMethod := strings.TrimSpace(request.Method)
	if trimmedMethod == "" {
		return fmt.Errorf("%w: method is required", ErrInvalidRequest)
	}

	trimmedPath := strings.TrimSpace(request.Path)
	if trimmedPath == "" {
		return fmt.Errorf("%w: path is required", ErrInvalidRequest)
	}

	state := &State{
		Request: Request{
			Host:                 trimmedHost,
			Port:                 trimmedPort,
			Service:             trimmedService,
			Method:              trimmedMethod,
			Path:                trimmedPath,
			AuthorizationHeader: request.AuthorizationHeader,
			ResponseWriter:      request.ResponseWriter,
			HTTPRequest:         request.HTTPRequest,
		},
	}

	for index, task := range o.tasks {
		if task == nil {
			return fmt.Errorf("%w: index %d", ErrNilTask, index)
		}

		if err := task.Run(ctx, state); err != nil {
			return fmt.Errorf("run gateway task: %w", err)
		}
	}

	return nil
}
