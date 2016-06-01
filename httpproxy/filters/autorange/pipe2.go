package autorange

import (
	"container/heap"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type fragment struct {
	data  []byte
	pos   int64
	index int
}

type fragmentQueue struct {
	q  []fragment
	mu sync.Mutex
}

func (q fragmentQueue) Len() int {
	return len(q.q)
}

func (q fragmentQueue) Less(i, j int) bool {
	return q.q[i].pos < q.q[j].pos
}

func (q fragmentQueue) Swap(i, j int) {
	q.q[i], q.q[j] = q.q[j], q.q[i]
	q.q[i].index = i
	q.q[j].index = j
}

func (q *fragmentQueue) Push(x interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	n := len(q.q)
	item := x.(fragment)
	item.index = n
	q.q = append(q.q, item)
}

func (q *fragmentQueue) Pop() interface{} {
	q.mu.Lock()
	defer q.mu.Unlock()

	old := q.q
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	q.q = old[0 : n-1]
	return item
}

type FragmentPipe struct {
	l    int64
	err  error
	pos  int64
	size int64
	q    fragmentQueue
	c    *sync.Cond
}

func NewFragmentPipe(size int64) *FragmentPipe {
	return &FragmentPipe{
		size: size,
		q: fragmentQueue{
			q: []fragment{},
		},
		c: sync.NewCond(&sync.Mutex{}),
	}
}

func (p *FragmentPipe) Len() int64 {
	return atomic.LoadInt64(&p.l)
}

func (p *FragmentPipe) Write(data []byte, pos int64) (int, error) {
	if pos == p.pos {
		defer p.c.Signal()
	}
	heap.Push(&p.q, fragment{data, pos, -1})
	atomic.AddInt64(&p.l, int64(len(data)))
	return len(data), nil
}

func (p *FragmentPipe) Close() error {
	defer p.c.Signal()
	return p.CloseWithError(io.EOF)
}

func (p *FragmentPipe) CloseWithError(err error) error {
	defer p.c.Signal()
	if err == nil {
		err = io.EOF
	}
	p.err = err
	return nil
}

func (p *FragmentPipe) Read(data []byte) (int, error) {
	if p.err != nil {
		return 0, p.err
	}

	if p.pos == p.size-1 {
		p.Close()
		return 0, io.EOF
	}

	top := heap.Pop(&p.q).(fragment)
	if p.pos > top.pos {
		return 0, fmt.Errorf("%T.pos=%d is larger then %d", p, p.pos, top.pos)
	} else if p.pos < top.pos {
		return 0, nil
	} else {
		n := copy(data, top.data)
		if n < len(top.data) {
			top.data = top.data[n:]
			top.pos += int64(n)
			heap.Push(&p.q, top)
		}
		atomic.AddInt64(&p.l, -int64(n))
		return n, nil
	}
}

func (p *FragmentPipe) WriteTo(w io.Writer) (int64, error) {
	var n int64
	for n < p.size {
		if p.err != nil {
			return 0, p.err
		}
		if p.q.Len() == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		top := heap.Pop(&p.q).(fragment)
		if top.pos < n {
			return 0, fmt.Errorf("%T.pos=%d is larger then %d", top, top.pos, n)
		} else if top.pos > n {
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			n1, err := w.Write(top.data)
			n += int64(n1)
			if err != nil {
				return n, err
			}
			if n1 < len(top.data) {
				top.data = top.data[n1:]
				top.pos += int64(n1)
				heap.Push(&p.q, top)
			}
		}
	}
	return n, nil
}
