package pool

import (
	"fmt"
	"net/http"
)

// NewTransport 티어 풀 설정을 반영한 새 http.Transport를 생성합니다.
func NewTransport(tier Tier) (*http.Transport, error) {
	config, err := GetConfig(tier)
	if err != nil {
		return nil, err
	}

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("%w: default transport is not *http.Transport", ErrInvalidConfig)
	}

	transport := defaultTransport.Clone()
	transport.MaxIdleConns = config.MaxIdleConns
	transport.MaxIdleConnsPerHost = config.MaxIdleConnsPerHost
	transport.MaxConnsPerHost = config.MaxConnsPerHost
	transport.IdleConnTimeout = config.IdleConnTimeout
	transport.ResponseHeaderTimeout = config.ResponseHeaderTimeout
	transport.TLSHandshakeTimeout = config.TLSHandshakeTimeout
	transport.ExpectContinueTimeout = config.ExpectContinueTimeout

	return transport, nil
}

func HandleRequest(serviceName string, r *http.Request) {
	//해당하는 서비스에 따라 저장된 풀을 사용. 풀은 없으면 만들고 있으면 정책에 따라 생성및 있는거 사용
	//서비스 이름이랑 경로 받아서 구별하는거임
}
