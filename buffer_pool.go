package main

import (
	"github.com/valyala/bytebufferpool"
	"sync/atomic"
)

var bufferCounter int64

func GetBuffer() *bytebufferpool.ByteBuffer {
	defer func() {
		atomic.AddInt64(&bufferCounter, 1)
	}()
	return bytebufferpool.Get()
}

func PutBuffer(buffer *bytebufferpool.ByteBuffer) {
	defer func() {
		atomic.AddInt64(&bufferCounter, -1)
	}()
	bytebufferpool.Put(buffer)
}
