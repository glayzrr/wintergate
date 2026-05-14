package pool_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

type trackedUpstream struct {
	server       *httptest.Server
	firstStarted chan string
	releaseCh    chan struct{}
	closed       chan string
	releaseOnce  sync.Once
	requests     atomic.Int64
}

type blockingPairUpstream struct {
	server            *httptest.Server
	firstStarted      chan struct{}
	secondStarted     chan struct{}
	releaseFirstCh    chan struct{}
	releaseSecondCh   chan struct{}
	releaseFirstOnce  sync.Once
	releaseSecondOnce sync.Once
	requests          atomic.Int64
}

type timeoutTrackedUpstream struct {
	server       *httptest.Server
	firstStarted chan string
	closed       chan string
	requests     atomic.Int64
}

func newBlockingPairUpstream(t *testing.T) *blockingPairUpstream {
	t.Helper()

	upstream := &blockingPairUpstream{
		firstStarted:    make(chan struct{}, 1),
		secondStarted:   make(chan struct{}, 1),
		releaseFirstCh:  make(chan struct{}),
		releaseSecondCh: make(chan struct{}),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := upstream.requests.Add(1)
		switch count {
		case 1:
			upstream.firstStarted <- struct{}{}
			<-upstream.releaseFirstCh
			w.WriteHeader(http.StatusOK)
			if _, err := fmt.Fprint(w, "first response"); err != nil {
				t.Errorf("Write returned error: %v", err)
			}
		case 2:
			upstream.secondStarted <- struct{}{}
			<-upstream.releaseSecondCh
			w.WriteHeader(http.StatusAccepted)
			if _, err := fmt.Fprint(w, "second response"); err != nil {
				t.Errorf("Write returned error: %v", err)
			}
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	upstream.server = server

	t.Cleanup(func() {
		upstream.releaseFirst()
		upstream.releaseSecond()
		server.Close()
	})

	return upstream
}

func newTimeoutTrackedUpstream(t *testing.T) *timeoutTrackedUpstream {
	t.Helper()

	upstream := &timeoutTrackedUpstream{
		firstStarted: make(chan string, 1),
		closed:       make(chan string, 4),
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := upstream.requests.Add(1)
		switch count {
		case 1:
			upstream.firstStarted <- r.RemoteAddr
			<-r.Context().Done()
		default:
			w.WriteHeader(http.StatusAccepted)
			if _, err := fmt.Fprint(w, "second response"); err != nil {
				t.Errorf("Write returned error: %v", err)
			}
		}
	}))
	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			upstream.closed <- conn.RemoteAddr().String()
		}
	}
	server.Start()
	upstream.server = server

	t.Cleanup(server.Close)

	return upstream
}

func newTrackedUpstream(t *testing.T) *trackedUpstream {
	t.Helper()

	upstream := &trackedUpstream{
		firstStarted: make(chan string, 1),
		releaseCh:    make(chan struct{}),
		closed:       make(chan string, 4),
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := upstream.requests.Add(1)
		switch count {
		case 1:
			upstream.firstStarted <- r.RemoteAddr
			<-upstream.releaseCh
			w.WriteHeader(http.StatusOK)
			if _, err := fmt.Fprint(w, "first response"); err != nil {
				t.Errorf("Write returned error: %v", err)
			}
		default:
			w.WriteHeader(http.StatusAccepted)
			if _, err := fmt.Fprint(w, "second response"); err != nil {
				t.Errorf("Write returned error: %v", err)
			}
		}
	}))
	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			upstream.closed <- conn.RemoteAddr().String()
		}
	}
	server.Start()
	upstream.server = server

	t.Cleanup(func() {
		upstream.releaseFirst()
		server.Close()
	})

	return upstream
}

func (u *blockingPairUpstream) URL() string {
	return u.server.URL
}

func (u *blockingPairUpstream) releaseFirst() {
	u.releaseFirstOnce.Do(func() {
		close(u.releaseFirstCh)
	})
}

func (u *blockingPairUpstream) releaseSecond() {
	u.releaseSecondOnce.Do(func() {
		close(u.releaseSecondCh)
	})
}

func (u *timeoutTrackedUpstream) URL() string {
	return u.server.URL
}

func (u *trackedUpstream) URL() string {
	return u.server.URL
}

func (u *trackedUpstream) releaseFirst() {
	u.releaseOnce.Do(func() {
		close(u.releaseCh)
	})
}
