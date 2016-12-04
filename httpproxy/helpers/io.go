package helpers

import (
	"io"
	"sync"
	// "github.com/cloudflare/golibs/bytepool"
)

const (
	BUFSZ = 32 * 1024
)

var (
	bufpool = sync.Pool{
		New: func() interface{} {
			return make([]byte, BUFSZ)
		},
	}
)

func IOCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := bufpool.Get().([]byte)
	written, err = io.CopyBuffer(dst, src, buf)
	bufpool.Put(buf)
	return written, err
}
