package pool

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	metricrecord "wintergate/internal/metric/record"
)

// Forwarder 결정된 pool client를 사용해 HTTP 요청을 업스트림으로 전달합니다.
type Forwarder struct {
	clients  ClientProvider
	recorder *metricrecord.Recorder
}

// ForwardRequest 업스트림 전달에 필요한 요청별 데이터입니다.
type ForwardRequest struct {
	Address    string
	Writer     http.ResponseWriter
	Request    *http.Request
	Assignment Assignment
}

// NewForwarder pool client provider와 metric recorder를 사용하는 Forwarder를 생성합니다.
func NewForwarder(clients ClientProvider, recorder *metricrecord.Recorder) *Forwarder {
	return &Forwarder{
		clients:  clients,
		recorder: recorder,
	}
}

// NewTransport 티어 풀 설정을 반영한 새 http.Transport를 생성합니다.
func NewTransport(tier Tier) (*http.Transport, error) {
	config, err := ConfigFor(tier)
	if err != nil {
		return nil, err
	}

	return makePool(config)
}

func makePool(config Config) (*http.Transport, error) {
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

// Handle 결정된 커넥션 풀로 요청을 업스트림에 전달합니다.
func (f *Forwarder) Handle(request ForwardRequest) (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		// ReverseProxy가 응답 본문 복사 실패를 http.ErrAbortHandler panic으로 전달하면 함수 에러로 변환합니다.
		recoveredErr, ok := recovered.(error)
		if ok && errors.Is(recoveredErr, http.ErrAbortHandler) {
			err = fmt.Errorf("copy upstream response body: %w", recoveredErr)
			return
		}

		// 예상한 프록시 전송 실패가 아닌 panic은 상위 런타임의 복구 정책을 따르도록 다시 전파합니다.
		panic(recovered)
	}()

	// 요청 배정 결과에 맞는 클라이언트 lease를 확보하고, 핸들러 종료 시 반납합니다.
	lease, err := f.clients.Acquire(request.Assignment)
	if err != nil {
		return err
	}
	if lease.Finish != nil {
		defer lease.Finish()
	}
	if lease.Client == nil {
		return fmt.Errorf("%w: http client is nil", ErrInvalidConfig)
	}

	// ReverseProxy 구성 전에 요청 객체와 응답 writer가 유효한지 확인합니다.
	if request.Request == nil {
		return fmt.Errorf("%w: request is nil", ErrInvalidConfig)
	}
	if request.Writer == nil {
		return fmt.Errorf("%w: response writer is nil", ErrInvalidConfig)
	}

	// 업스트림 주소와 원본 요청 경로를 합쳐 ReverseProxy가 사용할 대상 URL을 만듭니다.
	targetURL, err := upstreamURL(request.Address, request.Request.URL)
	if err != nil {
		return err
	}

	// pool 선택 결과를 한 번 만들어 요청 메트릭과 connection trace가 같은 label을 사용하게 합니다.
	poolObservation := metricrecord.PoolObservation{
		ServiceName: request.Assignment.ServiceName,
		Tier:        string(request.Assignment.Tier),
		Dedicated:   request.Assignment.Dedicated,
		Instance:    targetURL.Host,
	}

	// 메트릭 수집을 위해 pool 선택과 upstream 요청 시작 시점을 기록합니다.
	var donePool metricrecord.PoolDoneFunc
	if f.recorder != nil {
		donePool = f.recorder.RecordPool(poolObservation)
	}

	// 성공 응답과 프록시 실패가 같은 방식으로 pool 메트릭을 종료하도록 처리 지점을 통일합니다.
	finishPool := func(statusCode int) {
		if donePool != nil {
			donePool(metricrecord.PoolResult{
				StatusCode: statusCode,
			})
		}
	}

	// Rewrite 콜백에서 매 요청마다 독립된 URL 포인터를 사용할 수 있도록 대상 값을 복사합니다.
	target := *targetURL
	var proxyErr error
	proxy := &httputil.ReverseProxy{
		Rewrite: func(proxyRequest *httputil.ProxyRequest) {
			// 원본 요청의 메서드와 본문은 유지하고, 목적지만 선택된 업스트림으로 교체합니다.
			outURL := target
			proxyRequest.Out.URL = &outURL
			proxyRequest.Out.Host = outURL.Host
			proxyRequest.Out.RequestURI = ""
			proxyRequest.SetXForwarded()
		},
		Transport: tracingTransport{
			base:        lease.Client.Transport,
			recorder:    f.recorder,
			observation: poolObservation,
		},
		ErrorLog:   reverseProxyErrorLog,
		BufferPool: reverseProxyBufferPool,
		ModifyResponse: func(response *http.Response) error {
			// 업스트림 응답 헤더를 받은 시점의 상태 코드를 pool 요청 결과로 기록합니다.
			finishPool(response.StatusCode)
			return nil
		},
		ErrorHandler: func(writer http.ResponseWriter, _ *http.Request, proxyError error) {
			// 업스트림 연결 또는 전송 실패는 클라이언트에게 502로 응답하고 호출자에게도 반환합니다.
			proxyErr = proxyError
			finishPool(http.StatusBadGateway)
			http.Error(writer, "Bad Gateway", http.StatusBadGateway)
		},
	}

	// ReverseProxy가 응답 복사까지 수행하므로 ServeHTTP 반환 후에는 프록시 에러만 확인합니다.
	proxy.ServeHTTP(request.Writer, request.Request)
	if proxyErr != nil {
		return fmt.Errorf("proxy upstream request: %w", proxyErr)
	}

	return nil
}

func upstreamURL(host string, requestURL *url.URL) (*url.URL, error) {
	base, err := url.Parse(strings.TrimSpace(host))
	if err != nil {
		return nil, fmt.Errorf("%w: parse upstream host: %w", ErrInvalidConfig, err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("%w: upstream host must include scheme and host", ErrInvalidConfig)
	}

	target := *base
	if requestURL != nil {
		target.Path = joinURLPath(base.Path, requestURL.Path)
		target.RawQuery = requestURL.RawQuery
	}
	target.Fragment = ""

	return &target, nil
}

func joinURLPath(basePath, requestPath string) string {
	switch {
	case basePath == "":
		if requestPath == "" {
			return "/"
		}
		return requestPath
	case requestPath == "":
		return basePath
	case strings.HasSuffix(basePath, "/") && strings.HasPrefix(requestPath, "/"):
		return basePath + requestPath[1:]
	case !strings.HasSuffix(basePath, "/") && !strings.HasPrefix(requestPath, "/"):
		return basePath + "/" + requestPath
	default:
		return basePath + requestPath
	}
}
