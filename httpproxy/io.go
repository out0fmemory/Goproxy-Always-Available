package httpproxy

import (
	"io"
	"sync"
	"sync/atomic"
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

func IoCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(io.ReaderFrom); ok {
		return rt.ReadFrom(src)
	}
	buf := bufpool.Get().([]byte)
	defer bufpool.Put(buf)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
	}
	return written, err
}

type PipeReader struct {
	*io.PipeReader
	l *int64
}

func (r *PipeReader) Close() error {
	return r.PipeReader.Close()
}

func (r *PipeReader) CloseWithError(err error) error {
	return r.PipeReader.CloseWithError(err)
}

func (r *PipeReader) Read(data []byte) (n int, err error) {
	if n, err = r.PipeReader.Read(data); err == nil {
		atomic.AddInt64(r.l, -int64(n))
	}
	return n, err
}

func (r *PipeReader) Len() int64 {
	return atomic.LoadInt64(r.l)
}

type PipeWriter struct {
	*io.PipeWriter
	l *int64
}

func (w *PipeWriter) Close() error {
	return w.Close()
}

func (w *PipeWriter) CloseWithError(err error) error {
	return w.CloseWithError(err)
}

func (w *PipeWriter) Write(data []byte) (n int, err error) {
	if n, err = w.PipeWriter.Write(data); err == nil {
		atomic.AddInt64(w.l, int64(n))
	}
	return n, err
}

func (w *PipeWriter) Len() int64 {
	return atomic.LoadInt64(w.l)
}

func IoPipe() (*PipeReader, *PipeWriter) {
	r, w := io.Pipe()
	l := new(int64)
	return &PipeReader{r, l}, &PipeWriter{w, l}
}
