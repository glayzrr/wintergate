package pool

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	metricrecord "wintergate/internal/metric/record"
	poolconfig "wintergate/internal/pool/config"
	"wintergate/internal/pool/policy"
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
	Assignment policy.Assignment
}

// NewForwarder pool client provider와 metric recorder를 사용하는 Forwarder를 생성합니다.
func NewForwarder(clients ClientProvider, recorder *metricrecord.Recorder) *Forwarder {
	return &Forwarder{
		clients:  clients,
		recorder: recorder,
	}
}

// NewTransport 티어 풀 설정을 반영한 새 http.Transport를 생성합니다.
func NewTransport(tier poolconfig.Tier) (*http.Transport, error) {
	config, err := poolconfig.ConfigFor(tier)
	if err != nil {
		return nil, err
	}

	return makePool(config)
}

func makePool(config poolconfig.Config) (*http.Transport, error) {
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
func (f *Forwarder) Handle(request ForwardRequest) error {
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

	// 업스트림 요청 전에 요청 객체와 응답 writer가 유효한지 확인합니다.
	if request.Request == nil {
		return fmt.Errorf("%w: request is nil", ErrInvalidConfig)
	}
	if request.Writer == nil {
		return fmt.Errorf("%w: response writer is nil", ErrInvalidConfig)
	}

	// 업스트림 주소와 원본 요청 경로를 합쳐 업스트림 요청 대상 URL을 만듭니다.
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

	// 웹소켓 요청만 ReverseProxy로 처리합니다.
	if isWebSocketRequest(request.Request) {
		target := *targetURL
		var proxyErr error
		proxy := buildReverseProxy(reverseProxyConfig{
			target:      target,
			transport:   lease.Client.Transport,
			recorder:    f.recorder,
			observation: poolObservation,
			onStatus:    finishPool,
			onError: func(proxyError error) {
				proxyErr = proxyError
			},
		})

		if err := serveReverseProxy(proxy, request.Writer, request.Request); err != nil {
			return err
		}
		if proxyErr != nil {
			return fmt.Errorf("proxy upstream request: %w", proxyErr)
		}

		return nil
	}

	outReq, err := upstreamRequest(*targetURL, request.Request)
	if err != nil {
		return err
	}

	var getConnAt time.Time
	var dialStartedAt time.Time
	var dialDuration time.Duration
	var dialed bool
	if f.recorder != nil {
		trace := &httptrace.ClientTrace{
			GetConn: func(_ string) {
				getConnAt = time.Now()
			},
			ConnectStart: func(_, _ string) {
				dialStartedAt = time.Now()
			},
			ConnectDone: func(_, _ string, err error) {
				if err != nil || dialStartedAt.IsZero() {
					return
				}

				dialDuration = time.Since(dialStartedAt)
				dialed = true
			},
			GotConn: func(info httptrace.GotConnInfo) {
				waitDuration := time.Duration(0)
				if !getConnAt.IsZero() {
					waitDuration = time.Since(getConnAt)
				}

				// httptrace가 알려준 connection 획득 결과를 record 패키지에 전달해 event label은 내부에서 정합니다.
				f.recorder.RecordConnection(poolObservation, metricrecord.ConnectionObservation{
					Reused:       info.Reused,
					WasIdle:      info.WasIdle,
					WaitDuration: waitDuration,
					Dialed:       dialed,
					DialDuration: dialDuration,
				})
			},
		}

		outReq = outReq.WithContext(httptrace.WithClientTrace(outReq.Context(), trace))
	}

	response, err := lease.Client.Do(outReq)
	if err != nil {
		finishPool(http.StatusBadGateway)
		http.Error(request.Writer, "Bad Gateway", http.StatusBadGateway)
		return err
	}
	defer response.Body.Close()

	finishPool(response.StatusCode)
	copyHeader(request.Writer.Header(), response.Header)
	removeHopByHopHeaders(request.Writer.Header())
	request.Writer.WriteHeader(response.StatusCode)

	if _, err := io.Copy(request.Writer, response.Body); err != nil {
		return fmt.Errorf("copy upstream response body: %w", err)
	}

	return nil
}

func upstreamRequest(target url.URL, request *http.Request) (*http.Request, error) {
	if request == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrInvalidConfig)
	}

	outReq := request.Clone(request.Context())
	outReq.URL = &target
	outReq.Host = target.Host
	outReq.RequestURI = ""
	outReq.Header = request.Header.Clone()
	removeHopByHopHeaders(outReq.Header)

	return outReq, nil
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

func isWebSocketRequest(request *http.Request) bool {
	if request == nil {
		return false
	}

	return headerHasToken(request.Header, "Connection", "upgrade") &&
		strings.EqualFold(strings.TrimSpace(request.Header.Get("Upgrade")), "websocket")
}

func headerHasToken(header http.Header, key, token string) bool {
	for _, value := range header.Values(key) {
		for _, field := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(field), token) {
				return true
			}
		}
	}

	return false
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func removeHopByHopHeaders(header http.Header) {
	for _, connectionHeader := range header.Values("Connection") {
		for _, field := range strings.Split(connectionHeader, ",") {
			if trimmedField := strings.TrimSpace(field); trimmedField != "" {
				header.Del(trimmedField)
			}
		}
	}

	for _, key := range hopByHopHeaders {
		header.Del(key)
	}
}

var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}
