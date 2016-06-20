package helpers

import (
	"container/heap"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type fragment struct {
	pos  int64
	data []byte
}

type fragmentHeap []*fragment

func (h fragmentHeap) Len() int           { return len(h) }
func (h fragmentHeap) Less(i, j int) bool { return h[i].pos < h[j].pos }
func (h fragmentHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *fragmentHeap) Push(x interface{}) {
	*h = append(*h, x.(*fragment))
}

func (h *fragmentHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type FragmentPipe interface {
	io.ReadCloser
	io.WriterTo
	CloseWithError(err error) error
	Write(data []byte, pos int64) (int, error)
	WriteString(data string, pos int64) (int, error)
}

type fragmentPipe struct {
	length int64
	pos    int64
	err    atomic.Value
	size   int64
	heap   fragmentHeap
	mu     *sync.Mutex
	token  chan struct{}
}

func NewFragmentPipe(size int64) FragmentPipe {
	return &fragmentPipe{
		pos:   0,
		size:  size,
		heap:  []*fragment{},
		mu:    new(sync.Mutex),
		token: make(chan struct{}, 1024),
	}
}

func (p *fragmentPipe) Len() int64 {
	return atomic.LoadInt64(&p.length)
}

func (p *fragmentPipe) WriteString(data string, pos int64) (int, error) {
	return p.Write([]byte(data), pos)
}

func (p *fragmentPipe) Write(data []byte, pos int64) (int, error) {
	if err := p.err.Load(); err != nil {
		return 0, err.(error)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	heap.Push(&p.heap, &fragment{pos, data})
	atomic.AddInt64(&p.length, int64(len(data)))
	if pos == atomic.LoadInt64(&p.pos) {
		p.token <- struct{}{}
	}
	return len(data), nil
}

func (p *fragmentPipe) Read(data []byte) (int, error) {
	if err := p.err.Load(); err != nil {
		return 0, err.(error)
	}

	if atomic.LoadInt64(&p.pos) == p.size {
		p.Close()
		return 0, nil
	}

	<-p.token

	if err := p.err.Load(); err != nil {
		return 0, err.(error)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	top := p.heap[0]
	if atomic.LoadInt64(&p.pos) != top.pos {
		err := fmt.Errorf("%T.pos=%d is not equal to %T.pos=%d", top, top.pos, p, atomic.LoadInt64(&p.pos))
		defer p.CloseWithError(err)
		return 0, err
	}

	n := copy(data, top.data)
	atomic.AddInt64(&p.length, -int64(n))
	atomic.AddInt64(&p.pos, int64(n))
	if n < len(top.data) {
		top.pos += int64(n)
		top.data = top.data[n:]
		p.token <- struct{}{}
	} else {
		heap.Pop(&p.heap)
		if len(p.heap) > 0 && p.heap[0].pos == atomic.LoadInt64(&p.pos) {
			p.token <- struct{}{}
		}
	}
	return n, nil
}

func (p *fragmentPipe) Close() error {
	defer p.CloseWithError(io.EOF)
	return nil
}

func (p *fragmentPipe) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	p.err.Store(err)
	p.token <- struct{}{}
	return nil
}

func (p *fragmentPipe) writeTo(w io.Writer) (int64, error) {
	if err := p.err.Load(); err != nil {
		return 0, err.(error)
	}

	<-p.token

	if err := p.err.Load(); err != nil {
		return 0, err.(error)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	top := p.heap[0]
	if atomic.LoadInt64(&p.pos) != top.pos {
		err := fmt.Errorf("%T.pos=%d is not equal to %T.pos=%d", top, top.pos, p, atomic.LoadInt64(&p.pos))
		defer p.CloseWithError(err)
		return 0, err
	}

	n, err := w.Write(top.data)
	atomic.AddInt64(&p.length, -int64(n))
	atomic.AddInt64(&p.pos, int64(n))
	if err != nil {
		defer p.CloseWithError(err)
		return int64(n), err
	}

	if n < len(top.data) {
		top.pos += int64(n)
		top.data = top.data[n:]
		p.token <- struct{}{}
	} else {
		heap.Pop(&p.heap)
		if len(p.heap) > 0 && p.heap[0].pos == atomic.LoadInt64(&p.pos) {
			p.token <- struct{}{}
		}
	}

	return int64(n), nil
}

func (p *fragmentPipe) WriteTo(w io.Writer) (int64, error) {
	var size, n int64
	var err error
	for err == nil && size < p.size {
		n, err = p.writeTo(w)
		size += n
	}
	return size, err
}
