package pool

import "sync"

type byteBufferPool struct {
	pool sync.Pool
}

func (p *byteBufferPool) Get() []byte {
	buffer, ok := p.pool.Get().([]byte)
	if !ok || cap(buffer) < reverseProxyBufferSize {
		return make([]byte, reverseProxyBufferSize)
	}

	return buffer[:reverseProxyBufferSize]
}

func (p *byteBufferPool) Put(buffer []byte) {
	if cap(buffer) < reverseProxyBufferSize {
		return
	}

	p.pool.Put(buffer[:reverseProxyBufferSize])
}
