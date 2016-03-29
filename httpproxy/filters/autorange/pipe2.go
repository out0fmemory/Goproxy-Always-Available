package autorange

import (
	"io"
	"sync/atomic"
)

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
