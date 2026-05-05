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
)

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

// HandleRequest 서비스 트래픽 상태에 맞는 커넥션 풀로 요청을 업스트림에 전달합니다.
func HandleRequest(serviceName, address string, w http.ResponseWriter, r *http.Request, recorder *metricrecord.Recorder) error {
	// 요청 시작과 종료 시점을 기록해 서비스별 트래픽 상태를 갱신합니다.
	doneFunc := StartRecord(serviceName)
	defer doneFunc()

	status, err := StatusFor(serviceName)
	if err != nil {
		return err
	}

	// 현재 트래픽 상태를 기준으로 사용할 풀 티어와 전용 풀 여부를 결정합니다.
	decision := DecidePolicy(status)
	cachedClient, err := defaultClients.ClientFor(decision)
	if err != nil {
		return err
	}
	defer cachedClient.release()

	// 원본 요청을 업스트림 서버로 전달할 수 있는 형태로 복제합니다.
	outReq, err := upstreamRequest(address, r)
	if err != nil {
		return err
	}

	// pool 선택 결과를 한 번 만들어 요청 메트릭과 connection trace가 같은 label을 사용하게 합니다.
	poolObservation := metricrecord.PoolObservation{
		Service:   decision.Service,
		Tier:      string(decision.Tier),
		Dedicated: decision.Dedicated,
	}

	// 메트릭 수집을 위해 pool 선택과 upstream 요청 시작 시점을 기록합니다.
	var donePool metricrecord.PoolDoneFunc
	if recorder != nil {
		donePool = recorder.RecordPool(poolObservation)
	}

	var getConnAt time.Time
	if recorder != nil {
		trace := &httptrace.ClientTrace{
			GetConn: func(_ string) {
				getConnAt = time.Now()
			},
			GotConn: func(info httptrace.GotConnInfo) {
				waitDuration := time.Duration(0)
				if !getConnAt.IsZero() {
					waitDuration = time.Since(getConnAt)
				}

				// httptrace가 알려준 connection 획득 결과를 record 패키지에 전달해 event label은 내부에서 정합니다.
				recorder.RecordConnection(poolObservation, metricrecord.ConnectionObservation{
					Reused:       info.Reused,
					WasIdle:      info.WasIdle,
					WaitDuration: waitDuration,
				})
			},
		}

		outReq = outReq.WithContext(httptrace.WithClientTrace(outReq.Context(), trace))
	}

	// 선택된 클라이언트로 업스트림에 요청하고 응답을 클라이언트에게 그대로 전달합니다.
	resp, err := cachedClient.client.Do(outReq)
	if err != nil {
		if donePool != nil {
			donePool(metricrecord.PoolResult{
				StatusCode: http.StatusBadGateway,
			})
		}
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return err
	}
	defer resp.Body.Close()

	if donePool != nil {
		donePool(metricrecord.PoolResult{
			StatusCode: resp.StatusCode,
		})
	}

	copyHeader(w.Header(), resp.Header)
	removeHopByHopHeaders(w.Header())
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("copy upstream response body: %w", err)
	}

	return nil
}

func upstreamRequest(address string, r *http.Request) (*http.Request, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrInvalidConfig)
	}

	targetURL, err := upstreamURL(address, r.URL)
	if err != nil {
		return nil, err
	}

	outReq := r.Clone(r.Context())
	outReq.URL = targetURL
	outReq.Host = targetURL.Host
	outReq.RequestURI = ""
	outReq.Header = r.Header.Clone()
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
