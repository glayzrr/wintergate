package pool

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const defaultWindow = time.Minute

// DoneFunc 요청 처리가 끝났을 때 호출해 트래픽 기록을 마무리합니다.
type DoneFunc func()

// Status 특정 설정 키의 현재 트래픽 상태입니다.
type Status struct {
	ConfigKey        string
	InFlight         int64
	StartedRequests  uint64
	FinishedRequests uint64
	RequestsInWindow uint64
	RPS              float64
	AverageLatency   time.Duration
	Window           time.Duration
	LastSeenAt       time.Time
}

// Recorder 설정 키별 트래픽 상태를 기록합니다.
type Recorder struct {
	clock func() time.Time

	// recoder가 관찰할 시간의 범위를 의미합니다.
	window  time.Duration
	records map[string]*configRecord

	// records map의 동시 조회와 설정 키별 record 생성을 보호합니다.
	mu sync.RWMutex
}

type configRecord struct {
	inFlight         atomic.Int64
	startedRequests  atomic.Uint64
	finishedRequests atomic.Uint64
	totalLatencyNano atomic.Int64
	lastSeenNano     atomic.Int64
	buckets          []bucket
}

// bucket RPS계산을 위한 요청 저장소.
type bucket struct {
	Second   atomic.Int64
	requests atomic.Uint64

	// bucket이 다른 초로 재사용될 때 Second와 requests reset을 보호합니다.
	mu sync.Mutex
}

var defaultRecorder = NewRecorder()

// NewRecorder 기본 window를 사용하는 트래픽 Recorder를 생성합니다.
func NewRecorder() *Recorder {
	return newRecorder(time.Now, defaultWindow)
}

// DefaultRecorder 패키지 기본 트래픽 Recorder를 반환합니다.
func DefaultRecorder() *Recorder {
	return defaultRecorder
}

// StartRecord 기본 Recorder에 설정 키별 요청 시작을 기록하고 완료 함수를 반환합니다.
func StartRecord(configKey string) DoneFunc {
	return DefaultRecorder().Start(configKey)
}

// StatusFor 기본 Recorder에서 설정 키별 트래픽 상태를 반환합니다.
func StatusFor(configKey string) (Status, error) {
	return DefaultRecorder().StatusFor(configKey)
}

// Start 설정 키별 요청 시작을 기록하고 완료 함수를 반환합니다.
func (r *Recorder) Start(configKey string) DoneFunc {
	if r == nil {
		return noopDone
	}

	normalizedConfigKey := normalizeConfigKey(configKey)
	if normalizedConfigKey == "" {
		return noopDone
	}

	// 시작 시각은 done 호출 시 latency 계산에 사용합니다.
	startedAt := r.now()

	// 설정 키별 record를 가져온 뒤 hot path 카운터는 atomic으로 갱신합니다.
	record := r.recordFor(normalizedConfigKey)
	record.inFlight.Add(1)
	record.startedRequests.Add(1)
	record.lastSeenNano.Store(startedAt.UnixNano())
	record.bucketAdd(startedAt)

	// done이 여러 번 호출돼도 in-flight 감소와 완료 기록은 한 번만 반영합니다.
	var once sync.Once
	return func() {
		once.Do(func() {
			r.finish(record, startedAt)
		})
	}
}

// StatusFor 설정 키별 트래픽 상태의 현재 값을 반환합니다.
func (r *Recorder) StatusFor(configKey string) (Status, error) {
	if r == nil {
		return Status{}, fmt.Errorf("%w: recorder is nil", ErrStatusNotFound)
	}

	normalizedConfigKey := normalizeConfigKey(configKey)
	if normalizedConfigKey == "" {
		return Status{}, fmt.Errorf("%w: config key is required", ErrInvalidConfigKey)
	}

	now := r.now()

	record, found := r.findRecord(normalizedConfigKey)
	if !found {
		return Status{}, fmt.Errorf("%w: %s", ErrStatusNotFound, normalizedConfigKey)
	}

	requestsInWindow := record.requestsInWindow(now)
	finishedRequests := record.finishedRequests.Load()
	totalLatencyNano := record.totalLatencyNano.Load()
	status := Status{
		ConfigKey:        normalizedConfigKey,
		InFlight:         record.inFlight.Load(),
		StartedRequests:  record.startedRequests.Load(),
		FinishedRequests: finishedRequests,
		RequestsInWindow: requestsInWindow,
		RPS:              float64(requestsInWindow) / r.window.Seconds(),
		Window:           r.window,
		LastSeenAt:       unixNanoToTime(record.lastSeenNano.Load()),
	}
	if finishedRequests > 0 {
		status.AverageLatency = time.Duration(totalLatencyNano / int64(finishedRequests))
	}

	return status, nil
}

func newRecorder(clock func() time.Time, window time.Duration) *Recorder {
	if clock == nil {
		clock = time.Now
	}
	if window <= 0 {
		window = defaultWindow
	}
	if window < time.Second {
		window = time.Second
	}

	return &Recorder{
		clock:   clock,
		window:  window.Truncate(time.Second),
		records: make(map[string]*configRecord),
	}
}

func (r *Recorder) finish(record *configRecord, startedAt time.Time) {
	finishedAt := r.now()
	latency := finishedAt.Sub(startedAt)
	if latency < 0 {
		latency = 0
	}

	record.inFlight.Add(-1)
	record.finishedRequests.Add(1)
	record.totalLatencyNano.Add(latency.Nanoseconds())
	record.lastSeenNano.Store(finishedAt.UnixNano())
}

func (r *Recorder) recordFor(configKey string) *configRecord {
	// 대부분의 요청은 이미 생성된 record를 읽기만 하므로 read lock 경로를 먼저 탑니다.
	if record, found := r.findRecord(configKey); found {
		return record
	}

	// record가 없을 때만 write lock을 잡아 설정 키별 저장소를 생성합니다.
	r.mu.Lock()
	defer r.mu.Unlock()

	// lock 대기 중 다른 고루틴이 먼저 만들 수 있으므로 한 번 더 확인합니다.
	record, found := r.records[configKey]
	if found {
		return record
	}

	// RPS 계산은 고정 크기 ring bucket을 사용해 요청 수와 무관하게 메모리를 제한합니다.
	record = &configRecord{
		buckets: make([]bucket, bucketCount(r.window)),
	}
	r.records[configKey] = record

	return record
}

func (r *Recorder) findRecord(configKey string) (*configRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, found := r.records[configKey]

	return record, found
}

func (r *Recorder) now() time.Time {
	return r.clock()
}

func (r *configRecord) bucketAdd(startedAt time.Time) {
	second := startedAt.Unix()
	index := bucketIndex(second, len(r.buckets))
	currentBucket := &r.buckets[index]

	// 같은 초에 들어온 요청은 bucket 회전 없이 atomic 증가만 수행합니다.
	if currentBucket.Second.Load() == second {
		currentBucket.requests.Add(1)
		return
	}

	// bucket index는 window 길이만큼 재사용되므로 초가 바뀌는 순간에만 잠급니다.
	currentBucket.mu.Lock()
	defer currentBucket.mu.Unlock()

	// lock 대기 중 이미 같은 초로 교체됐을 수 있어 다시 확인합니다.
	if currentBucket.Second.Load() != second {
		currentBucket.Second.Store(second)
		currentBucket.requests.Store(0)
	}

	// 새 초로 reset한 뒤 현재 요청을 포함합니다.
	currentBucket.requests.Add(1)
}

func (r *configRecord) requestsInWindow(now time.Time) uint64 {
	nowSecond := now.Unix()
	var total uint64
	for index := range r.buckets {
		currentBucket := &r.buckets[index]
		currentBucket.mu.Lock()
		unixSecond := currentBucket.Second.Load()
		requests := currentBucket.requests.Load()
		currentBucket.mu.Unlock()

		delta := nowSecond - unixSecond
		if delta >= 0 && delta < int64(len(r.buckets)) {
			total += requests
		}
	}

	return total
}

func bucketCount(window time.Duration) int {
	count := int(window / time.Second)

	// 윈도우가 1s보다 작은 경우 1을 반환합니다.
	if count < 1 {
		return 1
	}

	return count
}

func bucketIndex(unixSecond int64, bucketCount int) int {
	index := int(unixSecond % int64(bucketCount))
	if index < 0 {
		index += bucketCount
	}

	return index
}

func unixNanoToTime(unixNano int64) time.Time {
	if unixNano == 0 {
		return time.Time{}
	}

	return time.Unix(0, unixNano)
}

func noopDone() {}
