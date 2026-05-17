package health

import (
	"sync"
	"sync/atomic"
	"time"
)

// Store 라우팅 hot path에서 lock 없이 읽을 수 있는 헬스 상태 저장소입니다.
type Store struct {
	state atomic.Pointer[map[string]statusRecord]
	mu    sync.Mutex
}

// NewStore 빈 헬스 상태 저장소를 생성합니다.
func NewStore() *Store {
	store := &Store{}
	state := make(map[string]statusRecord)
	store.state.Store(&state)

	return store
}

// IsRoutableKey 지정한 health key가 라우팅 가능한 상태인지 반환합니다.
func (s *Store) IsRoutableKey(healthKey string) bool {
	if s == nil || healthKey == "" {
		return true
	}

	record, found := s.recordFor(healthKey)
	if !found {
		return true
	}

	return record.status != StatusUnhealthy
}

// SetUnknown 지정한 health key를 unknown 상태로 초기화합니다.
func (s *Store) SetUnknown(healthKey string, generation uint64) {
	if s == nil || healthKey == "" {
		return
	}

	s.update(func(records map[string]statusRecord) {
		records[healthKey] = statusRecord{
			status:     StatusUnknown,
			generation: generation,
		}
	})
}

// Delete 지정한 health key의 상태를 제거합니다.
func (s *Store) Delete(healthKey string) {
	if s == nil || healthKey == "" {
		return
	}

	s.update(func(records map[string]statusRecord) {
		delete(records, healthKey)
	})
}

// UpdateStatus 등록된 health key의 상태를 갱신합니다.
func (s *Store) UpdateStatus(healthKey string, generation uint64, status Status, consecutiveFailures, consecutiveSuccesses int, err error) (statusRecord, bool) {
	if s == nil || healthKey == "" {
		return statusRecord{}, false
	}

	var lastError string
	if err != nil {
		lastError = err.Error()
	}

	var previous statusRecord
	var updated bool
	s.update(func(records map[string]statusRecord) {
		var found bool
		previous, found = records[healthKey]
		// health check 도중 설정이 바뀌게 되는 경우 이전 상태는 무시합니다.
		if !found || previous.generation != generation {
			return
		}

		records[healthKey] = statusRecord{
			status:               status,
			checkedAt:            time.Now(),
			consecutiveFailures:  consecutiveFailures,
			consecutiveSuccesses: consecutiveSuccesses,
			generation:           generation,
			lastError:            lastError,
		}
		updated = true
	})

	return previous, updated
}

func (s *Store) recordFor(healthKey string) (statusRecord, bool) {
	records := s.state.Load()
	if records == nil {
		return statusRecord{}, false
	}

	record, found := (*records)[healthKey]

	return record, found
}

func (s *Store) update(apply func(map[string]statusRecord)) {
	// write path만 직렬화합니다. 라우팅 hot path의 read는 atomic pointer만 읽으므로 이 lock을 기다리지 않습니다.
	s.mu.Lock()
	defer s.mu.Unlock()

	// 현재 read-only map을 직접 수정하지 않고 새 map에 복사합니다.
	// 이미 atomic pointer를 통해 읽고 있는 라우팅 고루틴들이 일관된 snapshot을 계속 보게 하기 위함입니다.
	nextRecords := make(map[string]statusRecord)
	currentRecords := s.state.Load()
	if currentRecords != nil {
		for key, record := range *currentRecords {
			nextRecords[key] = record
		}
	}

	// 호출자가 새 map에만 변경을 적용하게 한 뒤, 완성된 map 포인터를 한 번에 공개합니다.
	// 이 시점 이후 새 read는 nextRecords를 보고, 기존 read는 이전 map을 계속 안전하게 봅니다.
	apply(nextRecords)
	s.state.Store(&nextRecords)
}
