package gateway

import (
	"context"
	"fmt"

	"wintergate/internal/trace"
	"wintergate/internal/utils"
)

// TraceTask 게이트웨이 요청 ID를 생성하거나 기존 요청 ID를 전파합니다.
type TraceTask struct {
	generator *trace.Generator
}

// NewTraceTask 요청 ID 생성용 TraceTask를 생성합니다.
func NewTraceTask(generator *trace.Generator) *TraceTask {
	return &TraceTask{
		generator: generator,
	}
}

// Run 요청 ID를 상태, 요청 헤더, 응답 헤더에 반영합니다.
func (t *TraceTask) Run(_ context.Context, state *State) error {
	if t == nil || t.generator == nil {
		return fmt.Errorf("trace generator: %w", trace.ErrNilGenerator)
	}
	if state == nil {
		return fmt.Errorf("%w: state is required", ErrInvalidRequest)
	}

	requestID, found := utils.NormalizeRequestID(state.Request.ID, trace.MaxRequestIDLength)
	if !found && state.Request.HTTPRequest != nil {
		requestID, found = utils.NormalizeRequestID(state.Request.HTTPRequest.Header.Get(trace.RequestIDHeader), trace.MaxRequestIDLength)
	}
	if !found {
		generatedID, err := t.generator.Generate(state.Request.ServiceName)
		if err != nil {
			return fmt.Errorf("generate trace id: %w", err)
		}
		requestID = generatedID
	}

	state.Request.ID = requestID
	if state.Request.HTTPRequest != nil {
		state.Request.HTTPRequest.Header.Set(trace.RequestIDHeader, requestID)
	}
	if state.Request.ResponseWriter != nil {
		state.Request.ResponseWriter.Header().Set(trace.RequestIDHeader, requestID)
	}

	return nil
}
