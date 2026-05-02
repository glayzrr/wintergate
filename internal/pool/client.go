package pool

import (
	"net/http"
	"sync"
)

type cachedClient struct {
	tier   Tier
	client *http.Client

	// 커넥션 풀 교체시 미완료된 요청을 기다리기 위해 사용됩니다.
	wg sync.WaitGroup
}

func newCachedClient(tier Tier) (*cachedClient, error) {
	transport, err := NewTransport(tier)
	if err != nil {
		return nil, err
	}

	return &cachedClient{
		tier: tier,
		client: &http.Client{
			Transport: transport,
		},
	}, nil
}

func (c *cachedClient) closeIdleConnections() {
	if c == nil || c.client == nil {
		return
	}

	transport, ok := c.client.Transport.(*http.Transport)
	if !ok {
		return
	}

	transport.CloseIdleConnections()
}

func (c *cachedClient) acquire() {
	if c == nil {
		return
	}

	c.wg.Add(1)
}

func (c *cachedClient) release() {
	if c == nil {
		return
	}

	c.wg.Done()
}

func (c *cachedClient) retire() {
	if c == nil {
		return
	}

	c.closeIdleConnections()
	go func() {
		c.wg.Wait()
		c.closeIdleConnections()
	}()
}
