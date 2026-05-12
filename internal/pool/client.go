package pool

import (
	"net/http"
	"sync"
)

// managedClient 풀 교체 중에도 이미 선택된 http.Client의 요청 생명주기를 추적합니다.
//
// 전용 pool tier가 바뀌면 store에서는 새 요청을 새 client로 보내지만, 교체 직전에
// 기존 client를 받은 요청은 해당 client와 transport로 계속 업스트림 응답을 기다립니다.
// wg는 그 요청들이 모두 끝난 뒤 교체된 transport의 idle connection을 정리하기 위해
// 사용합니다.
type managedClient struct {
	tier   Tier
	client *http.Client

	// 커넥션 풀 교체시 미완료된 요청을 기다리기 위해 사용됩니다.
	wg sync.WaitGroup
}

// newManagedClient 지정한 tier의 transport 설정을 고정한 cachedClient를 생성합니다.
//
// http.Transport의 pool 설정은 요청 처리 중 바꾸지 않고, tier 변경 시 새 client를 만들어
// 이후 요청만 새 pool을 사용하게 합니다.
func newManagedClient(tier Tier) (*managedClient, error) {
	transport, err := NewTransport(tier)
	if err != nil {
		return nil, err
	}

	return &managedClient{
		tier: tier,
		client: &http.Client{
			Transport: transport,
		},
	}, nil
}

// closeIdleConnections 교체 대상 client가 더 이상 붙잡을 필요 없는 idle connection을 닫습니다.
//
// CloseIdleConnections는 진행 중인 요청의 connection은 닫지 않으므로, retire는 교체 직후와
// drain 완료 후에 이 메서드를 호출해 기존 pool의 유휴 connection을 정리합니다.
func (c *managedClient) closeIdleConnections() {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// acquire 이 client를 배정받은 요청을 drain 대기 대상에 추가합니다.
//
// store에서 client 포인터를 반환하기 전에 호출해, 이후 pool이 교체되더라도 retire가 해당
// 요청의 종료를 기다릴 수 있게 합니다.
func (c *managedClient) acquire() {
	c.wg.Add(1)
}

// release 이 client를 배정받은 요청이 끝났음을 retire 대기자에게 알립니다.
//
// Forwarder는 upstream 응답 처리까지 마친 뒤 defer로 호출해, 교체된 client의 정리 시점이
// 실제 요청 생명주기보다 앞서지 않게 합니다.
func (c *managedClient) release() {
	c.wg.Done()
}

// retire 새 요청에서 제외된 client를 drain하고 idle connection을 정리합니다.
//
// store의 캐시에서 제거된 뒤에도 이미 acquire한 요청은 이 client를 계속 사용할 수 있습니다.
// 그래서 즉시 active 요청을 끊지 않고, 먼저 현재 idle connection을 닫은 다음 모든 요청이
// release될 때까지 기다렸다가 마지막으로 남은 idle connection을 다시 닫습니다.
func (c *managedClient) retire() {
	// 현재 생성된 유휴 커넥션을 바로 종료합니다.
	c.closeIdleConnections()

	// 아직 실행중인 요청이 만료될때까지 기다린 후 유휴커넥션을 종료합니다.
	go func() {
		c.wg.Wait()
		c.closeIdleConnections()
	}()
}
