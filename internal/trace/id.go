package trace

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type uuidFunc func() (uuid.UUID, error)

// Generator 게이트웨이 요청 추적에 사용할 요청 ID를 생성합니다.
type Generator struct {
	newID uuidFunc
}

// NewGenerator UUID 기반 요청 ID Generator를 생성합니다.
func NewGenerator() *Generator {
	return &Generator{
		newID: uuid.NewRandom,
	}
}

// Generate 서비스 식별자를 포함한 새 요청 ID를 생성합니다.
func (g *Generator) Generate(service string) (string, error) {
	if g == nil {
		return "", ErrNilGenerator
	}

	newID := g.newID
	if newID == nil {
		newID = uuid.NewRandom
	}

	id, err := newID()
	if err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}

	normalizedService := strings.TrimSpace(service)
	if normalizedService == "" || strings.ContainsAny(normalizedService, "\r\n") {
		return id.String(), nil
	}

	requestID := fmt.Sprintf("%s-%s", normalizedService, id)
	if _, ok := NormalizeID(requestID); !ok {
		return id.String(), nil
	}

	return requestID, nil
}
