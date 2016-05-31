package autorange

import (
	"container/heap"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

type fragment struct {
	data  []byte
	pos   int
	index int
}

type fragmentQueue []*fragment

func (q fragmentQueue) Len() int {
	return len(q)
}

func (q fragmentQueue) Less(i, j int) bool {
	return q[i].pos < q[j].pos
}

func (q fragmentQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *fragmentQueue) Push(x interface{}) {
	n := len(*q)
	item := x.(*fragment)
	item.index = n
	*q = append(*q, item)
}

func (q *fragmentQueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*q = old[0 : n-1]
	return item
}

func (q *fragmentQueue) Top() *fragment {
	return (*q)[len(*q)-1]
}

type FragmentPipe struct {
	l    int64
	mu   sync.Mutex
	err  error
	pos  int
	size int
	q    fragmentQueue
}

func (p *FragmentPipe) Len() int64 {
	return atomic.LoadInt64(&p.l)
}

func (p *FragmentPipe) Write(data []byte, pos int) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	heap.Push(&p.q, &fragment{data, pos, -1})
	atomic.AddInt64(&p.l, int64(len(data)))
	return len(data), nil
}

func (p *FragmentPipe) Close() error {
	return p.CloseWithError(io.EOF)
}

func (p *FragmentPipe) CloseWithError(err error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err == nil {
		err = io.EOF
	}
	p.err = err
	return nil
}

func (p *FragmentPipe) Read(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.err != nil {
		return 0, p.err
	}

	if p.pos == p.size-1 {
		p.Close()
		return 0, io.EOF
	}

	top := p.q.Top()
	if p.pos > top.pos {
		return 0, fmt.Errorf("%T.pos=%d is larger then %d", p, p.pos, top.pos)
	} else if p.pos < top.pos {
		return 0, nil
	} else {
		n := copy(data, top.data)
		if n < len(top.data) {
			top.data = top.data[n:]
			top.pos += n
		} else {
			heap.Pop(&p.q)
		}
		atomic.AddInt64(&p.l, -int64(n))
		return n, nil
	}
}
