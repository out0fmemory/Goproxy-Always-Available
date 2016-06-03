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

type FragmentPipe struct {
	length int64
	err    atomic.Value
	pos    int64
	size   int64
	heap   fragmentHeap
	mu     *sync.Mutex
	cond   *sync.Cond
}

func NewFragmentPipe(size int64) *FragmentPipe {
	return &FragmentPipe{
		pos:  0,
		size: size,
		heap: []*fragment{},
		mu:   new(sync.Mutex),
		cond: sync.NewCond(new(sync.Mutex)),
	}
}

func (p *FragmentPipe) Len() int64 {
	return atomic.LoadInt64(&p.length)
}

func (p *FragmentPipe) Write(data []byte, pos int64) (int, error) {
	if err := p.err.Load().(error); err != nil {
		return 0, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if pos == p.pos {
		defer p.cond.Signal()
	}
	heap.Push(&p.heap, &fragment{pos, data})
	atomic.AddInt64(&p.length, int64(len(data)))
	return len(data), nil
}

func (p *FragmentPipe) Read(data []byte) (int, error) {
	p.cond.L.Lock()
	defer p.cond.L.Unlock()
	p.cond.Wait()

	if err := p.err.Load().(error); err != nil {
		return 0, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pos == p.size-1 {
		p.Close()
		return 0, nil
	}

	top := p.heap[0]
	if p.pos != top.pos {
		err := fmt.Errorf("%T.pos=%d is not equal to %T.pos=%d", top, top.pos, p, p.pos)
		defer p.CloseWithError(err)
		return 0, err
	}

	n := copy(data, top.data)
	atomic.AddInt64(&p.length, -int64(n))
	p.pos += int64(n)
	if n < len(top.data) {
		top.pos += int64(n)
		top.data = top.data[n:]
		defer p.cond.Signal()
	} else {
		heap.Pop(&p.heap)
		if p.heap[0].pos == p.pos {
			defer p.cond.Signal()
		}
	}
	return n, nil
}

func (p *FragmentPipe) Close() error {
	defer p.cond.Signal()
	return p.CloseWithError(io.EOF)
}

func (p *FragmentPipe) CloseWithError(err error) error {
	defer p.cond.Signal()
	if err == nil {
		err = io.EOF
	}
	p.err.Store(err)
	return nil
}

func (p *FragmentPipe) writeTo(w io.Writer) (int64, error) {
	p.cond.L.Lock()
	p.cond.Wait()
	defer p.cond.L.Unlock()

	if err := p.err.Load().(error); err != nil {
		return 0, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for {
		top := p.heap[0]
		if p.pos != top.pos {
			err := fmt.Errorf("%T.pos=%d is not equal to %T.pos=%d", top, top.pos, p, p.pos)
			defer p.CloseWithError(err)
			return 0, err
		}

		n, err := w.Write(top.data)
		atomic.AddInt64(&p.length, -int64(n))
		p.pos += int64(n)
		if err != nil {
			defer p.CloseWithError(err)
			return int64(n), err
		}

		if n < len(top.data) {
			top.pos += int64(n)
			top.data = top.data[n:]
			defer p.cond.Signal()
		} else {
			heap.Pop(&p.heap)
			if p.heap[0].pos == p.pos {
				defer p.cond.Signal()
			}
		}
	}
}

func (p *FragmentPipe) WriteTo(w io.Writer) (int64, error) {
	var size, n int64
	var err error
	for err == nil && size < p.size {
		n, err = p.writeTo(w)
		size += n
	}
	return size, err
}
