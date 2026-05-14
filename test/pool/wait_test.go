package pool_test

import (
	"testing"
	"time"
)

func waitForSignal(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func assertRemoteAddressStillOpen(t *testing.T, ch <-chan string, remoteAddress string, duration time.Duration) {
	t.Helper()

	timeout := time.After(duration)
	for {
		select {
		case closedRemoteAddress := <-ch:
			if closedRemoteAddress == remoteAddress {
				t.Fatalf("connection %q closed before in-flight request completed", remoteAddress)
			}
		case <-timeout:
			return
		}
	}
}

func waitForString(t *testing.T, ch <-chan string, name string) string {
	t.Helper()

	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}

	return ""
}

func waitForResult(t *testing.T, ch <-chan receiveResult, name string) receiveResult {
	t.Helper()

	select {
	case result := <-ch:
		return result
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}

	return receiveResult{}
}

func waitForClosedRemoteAddress(t *testing.T, ch <-chan string, remoteAddress string) {
	t.Helper()

	timeout := time.After(time.Second)
	for {
		select {
		case closedRemoteAddress := <-ch:
			if closedRemoteAddress == remoteAddress {
				return
			}
		case <-timeout:
			t.Fatalf("timed out waiting for closed connection %q", remoteAddress)
		}
	}
}
