package health

import (
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	internalconfig "wintergate/internal/config"
	"wintergate/internal/utils"
)

// desiredTargets 중앙 스냅샷에서 활성화된 health target 목록을 계산합니다.
func desiredTargets(snapshot *internalconfig.Snapshot) map[targetKey]desiredTarget {
	if snapshot == nil {
		return nil
	}

	// targetKey를 map key로 사용해 같은 서비스 인스턴스가 한 번만 health loop를 갖도록 합니다.
	targets := make(map[targetKey]desiredTarget)
	for serviceName, service := range snapshot.Services {
		settings, err := runtimeSettingsFrom(service.Health)
		if err != nil {
			slog.Info(
				logHealthCheckConfigSkipped,
				logAttrServiceName, serviceName,
				logAttrError, err,
			)
			continue
		}
		if !settings.enabled {
			continue
		}

		for _, instance := range service.Instances {
			// snapshot에 정규화된 인스턴스가 들어온다는 전제를 쓰되, 빈 값은 방어적으로 제외합니다.
			key := targetKeyFor(serviceName, instance)
			if key.serviceName == "" || key.host == "" || key.port == "" || key.key() == "" {
				continue
			}

			// jitter는 target 주소에서 결정적으로 계산해 재시작 전까지 같은 인스턴스가 같은 offset을 갖게 합니다.
			targets[key] = desiredTarget{
				key:      key,
				instance: instance,
				settings: settings,
				jitter:   jitterFor(key, settings.jitter),
			}
		}
	}

	return targets
}

// runtimeSettingsFrom 저장된 문자열 설정을 health loop가 사용할 duration 설정으로 변환합니다.
func runtimeSettingsFrom(settings *internalconfig.HealthSettings) (runtimeSettings, error) {
	defaultSettings := internalconfig.DefaultHealthSettings()
	if settings == nil {
		settings = defaultSettings
	}

	enabled := true
	if settings.Enabled != nil {
		enabled = *settings.Enabled
	}

	interval, err := parseDurationWithDefault(settings.Interval, defaultSettings.Interval, "health interval")
	if err != nil {
		return runtimeSettings{}, err
	}
	timeout, err := parseDurationWithDefault(settings.Timeout, defaultSettings.Timeout, "health timeout")
	if err != nil {
		return runtimeSettings{}, err
	}
	jitter, err := parseDurationWithDefault(settings.Jitter, defaultSettings.Jitter, "health jitter")
	if err != nil {
		return runtimeSettings{}, err
	}
	maxBackoff, err := parseDurationWithDefault(settings.MaxBackoff, defaultSettings.MaxBackoff, "health max backoff")
	if err != nil {
		return runtimeSettings{}, err
	}

	path := strings.TrimSpace(settings.Path)
	if path == "" {
		path = defaultSettings.Path
	}
	failureThreshold := settings.FailureThreshold
	if failureThreshold <= 0 {
		failureThreshold = defaultSettings.FailureThreshold
	}
	successThreshold := settings.SuccessThreshold
	if successThreshold <= 0 {
		successThreshold = defaultSettings.SuccessThreshold
	}

	return runtimeSettings{
		enabled:          enabled,
		path:             path,
		interval:         interval,
		timeout:          timeout,
		jitter:           jitter,
		maxBackoff:       maxBackoff,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
	}, nil
}

// parseDurationWithDefault 비어 있는 duration 문자열에는 기본값을 적용합니다.
func parseDurationWithDefault(value, defaultValue, fieldName string) (time.Duration, error) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		trimmedValue = defaultValue
	}

	duration, err := time.ParseDuration(trimmedValue)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", fieldName, err)
	}

	return duration, nil
}

// nextInterval 실패 횟수를 바탕으로 다음 health check 간격을 계산합니다.
func nextInterval(settings runtimeSettings, jitter time.Duration, consecutiveFailures int) time.Duration {
	interval := settings.interval
	if consecutiveFailures >= settings.failureThreshold {
		// unhealthy가 길어질수록 체크 간격을 늘리되 max backoff에서 고정합니다.
		for index := 0; index <= consecutiveFailures-settings.failureThreshold; index++ {
			if interval >= settings.maxBackoff/2 {
				interval = settings.maxBackoff
				break
			}
			interval *= 2
		}
	}

	if interval > settings.maxBackoff {
		interval = settings.maxBackoff
	}

	return interval + jitter
}

// jitterFor target 주소에 대해 고정된 jitter 값을 계산합니다.
func jitterFor(key targetKey, maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 {
		return 0
	}

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(key.key()))

	return time.Duration(hasher.Sum64() % uint64(maxJitter))
}

// targetKeyFor 서비스 이름과 인스턴스 설정을 health target key로 변환합니다.
func targetKeyFor(serviceName string, instance internalconfig.InstanceSettings) targetKey {
	normalizedServiceName := utils.NormalizeServiceName(serviceName)
	scheme := strings.ToLower(strings.TrimSpace(instance.Scheme))
	host := strings.TrimSpace(instance.Host)
	port := strings.TrimSpace(instance.Port)
	healthKey := strings.TrimSpace(instance.HealthKey)
	if healthKey == "" && normalizedServiceName != "" && scheme != "" && host != "" && port != "" {
		healthKey = normalizedServiceName + "|" + scheme + "|" + host + "|" + port
	}

	return targetKey{
		serviceName: normalizedServiceName,
		scheme:      scheme,
		host:        host,
		port:        port,
		healthKey:  healthKey,
	}
}

// healthURL 인스턴스 주소와 health path를 호출 가능한 URL로 조합합니다.
func healthURL(instance internalconfig.InstanceSettings, path string) string {
	targetURL := url.URL{
		Scheme: strings.ToLower(strings.TrimSpace(instance.Scheme)),
		Host:   net.JoinHostPort(strings.TrimSpace(instance.Host), strings.TrimSpace(instance.Port)),
		Path:   path,
	}

	return targetURL.String()
}
