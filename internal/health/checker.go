package health

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// run 하나의 인스턴스에 대해 jitter와 backoff를 적용한 health loop를 실행합니다.
func (m *Manager) run(ctx context.Context, target *target) {
	// 첫 실행도 interval+jitter 뒤에 시작해 config commit 직후 모든 인스턴스가 동시에 ping되지 않게 합니다.
	interval := nextInterval(target.settings, target.jitter, 0)
	timer := time.NewTimer(interval)
	defer timer.Stop()

	// 상태 전환 판단에 필요한 카운터는 target 고루틴 하나가 단독으로 소유합니다.
	// Manager에는 계산 결과만 저장해 lock을 잡는 시간을 짧게 유지합니다.
	status := StatusUnknown
	consecutiveFailures := 0
	consecutiveSuccesses := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			// check는 health 전용 client만 사용하므로 메인 라우팅 pool의 고갈 여부와 분리됩니다.
			ok, err := m.check(ctx, target)
			if ok {
				// 성공 응답은 backoff를 즉시 원복하고 success threshold를 만족하면 healthy로 전환합니다.
				consecutiveSuccesses++
				consecutiveFailures = 0
				if consecutiveSuccesses >= target.settings.successThreshold {
					status = StatusHealthy
				}
			} else {
				// 실패가 threshold에 도달하기 전까지는 unknown/healthy 상태를 유지해 순간 장애를 흡수합니다.
				// threshold 이후에는 unhealthy로 전환되고, 이후 실패 횟수에 따라 다음 interval이 점진적으로 증가합니다.
				consecutiveFailures++
				consecutiveSuccesses = 0
				if consecutiveFailures >= target.settings.failureThreshold {
					status = StatusUnhealthy
				}
			}

			// setStatus는 상태 저장과 상태 전환 로그만 담당하고, 다음 주기 계산은 고루틴 로컬 카운터로 처리합니다.
			m.setStatus(target, status, consecutiveFailures, consecutiveSuccesses, err)
			interval = nextInterval(target.settings, target.jitter, consecutiveFailures)
			timer.Reset(interval)
		}
	}
}

// check 지정한 target의 health endpoint를 한 번 호출하고 성공 여부를 반환합니다.
func (m *Manager) check(ctx context.Context, target *target) (bool, error) {
	// client는 reconcile에서 target 수에 맞춰 교체될 수 있으므로 호출 시점마다 현재 포인터를 가져옵니다.
	client := m.currentClient()
	if client == nil {
		return false, fmt.Errorf("health client is unavailable")
	}

	// 개별 health request timeout은 루프 context의 종료 신호보다 좁은 범위에서만 적용합니다.
	// deregister로 ctx가 취소되면 timeout을 기다리지 않고 request도 함께 중단됩니다.
	requestCtx, cancel := context.WithTimeout(ctx, target.settings.timeout)
	defer cancel()

	// health URL은 등록된 인스턴스 주소와 중앙 설정의 path만 사용해 구성합니다.
	// 사용자 요청의 path/query/header는 health check로 전달하지 않습니다.
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, healthURL(target.instance, target.settings.path), nil)
	if err != nil {
		return false, fmt.Errorf("build health request: %w", err)
	}

	// health 전용 client를 사용하므로 이 요청은 메인 트래픽용 transport와 connection pool을 공유하지 않습니다.
	response, err := client.Do(request)
	if err != nil {
		return false, fmt.Errorf("send health request: %w", err)
	}
	defer func() {
		// body를 제한적으로 비워 keep-alive connection 재사용 가능성을 유지합니다.
		if _, err := io.Copy(io.Discard, io.LimitReader(response.Body, healthDrainLimit)); err != nil {
			slog.Info(
				logHealthResponseBodyDiscard,
				logAttrServiceName, target.key.serviceName,
				logAttrInstance, target.key.instance(),
				logAttrError, err,
			)
		}
		if err := response.Body.Close(); err != nil {
			slog.Info(
				logHealthResponseBodyDiscard,
				logAttrServiceName, target.key.serviceName,
				logAttrInstance, target.key.instance(),
				logAttrError, err,
			)
		}
	}()

	// 2xx와 3xx는 살아 있는 endpoint로 보고, 4xx/5xx는 라우팅 제외 후보로 누적합니다.
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusBadRequest {
		return false, fmt.Errorf("health response status %d", response.StatusCode)
	}

	return true, nil
}

// currentClient 현재 health 전용 HTTP client를 반환합니다.
func (m *Manager) currentClient() *http.Client {
	return m.client.Load()
}

// setStatus health check 결과를 저장하고 상태 전환 시 한 번만 로그를 남깁니다.
func (m *Manager) setStatus(target *target, status Status, consecutiveFailures, consecutiveSuccesses int, err error) {
	var lastError string
	if err != nil {
		lastError = err.Error()
	}

	key := target.key
	previousRecord, updated := m.store.UpdateStatus(key.key(), target.generation, status, consecutiveFailures, consecutiveSuccesses, err)
	if !updated {
		return
	}

	if previousRecord.status != status {
		slog.Info(
			logHealthStatusChanged,
			logAttrServiceName, key.serviceName,
			logAttrInstance, key.instance(),
			logAttrStatus, status,
			logAttrPreviousStatus, previousRecord.status,
			logAttrError, lastError,
			logAttrConsecutiveFailures, consecutiveFailures,
			logAttrConsecutiveSuccess, consecutiveSuccesses,
		)
	}
}
