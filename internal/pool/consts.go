package pool

import (
	"io"
	"log"
	"sync"
)

const reverseProxyBufferSize = 32 * 1024

var (
	reverseProxyErrorLog   = log.New(io.Discard, "", 0)
	reverseProxyBufferPool = &byteBufferPool{
		pool: sync.Pool{
			New: func() any {
				return make([]byte, reverseProxyBufferSize)
			},
		},
	}
)
