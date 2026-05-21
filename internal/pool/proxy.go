package pool

import (
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/url"
	"time"
	metricrecord "wintergate/internal/metric/record"
)

type tracingTransport struct {
	base        http.RoundTripper
	recorder    *metricrecord.Recorder
	observation metricrecord.PoolObservation
}

func (t tracingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	if t.recorder == nil {
		return base.RoundTrip(request)
	}

	var getConnAt time.Time
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
			t.recorder.RecordConnection(t.observation, metricrecord.ConnectionObservation{
				Reused:       info.Reused,
				WasIdle:      info.WasIdle,
				WaitDuration: waitDuration,
			})
		},
	}

	return base.RoundTrip(request.WithContext(httptrace.WithClientTrace(request.Context(), trace)))
}

func ProxyFor(target url.URL, lease ClientLease) *httputil.ReverseProxy {
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

	return proxy
}
