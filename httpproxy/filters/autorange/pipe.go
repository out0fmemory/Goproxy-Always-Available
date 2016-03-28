package autorange

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

var (
	ErrClosedPipe = errors.New("io: read/write on closed pipe")
	ErrFailedPipe = errors.New("pipe failed previously")
)

type autoPipe struct {
	Threads chan bool
	len     int64
	pipers  []*piper
	eindex  int32 // 如果某个 "range" 出错了，应改写这个值。用以告诉 read 端不要读 eindex 之后的 piper 上的数据。以及通知 eindex 之后 piper 申请停止被 write
	l       sync.Mutex
	rwait   sync.Cond
	rindex  uint32 // 正在被 read 的 piper 序号
	rerr    error  // 当 read 端失败后应被触发以通知内存中 piper 申请停止被 write
	werr    error  // 这里的 werr 只在 write 端最后的 range 被发起后被引发
}

type piper struct {
	*pipe
	index  uint32
	parent *autoPipe
}

func (p *autoPipe) Read(b []byte) (n int, err error) {
	for {
		p.l.Lock()
		if p.eindex != -1 && int32(p.rindex) > p.eindex {
			p.l.Unlock()
			return 0, ErrFailedPipe
		}
		if uint32(len(p.pipers)) > p.rindex {
			p.l.Unlock()
			break
		}
		if p.werr != nil {
			err = p.werr
			p.l.Unlock()
			return
		}

		p.rwait.Wait()
		p.l.Unlock()
	}
	n, err = p.pipers[p.rindex].read(b)
	if err == io.EOF {
		p.pipers[p.rindex] = nil
		atomic.AddUint32(&p.rindex, 1)
	}
	return n, nil
}

func (p *piper) Write(b []byte) (n int, err error) {
	p.parent.l.Lock()
	if p.parent.rerr != nil {
		err = p.parent.rerr
		p.parent.l.Unlock()
		p = nil
		return
	}
	if p.parent.eindex != -1 && int32(p.index) > p.parent.eindex {
		p.parent.l.Unlock()
		p = nil
		err = ErrFailedPipe
		return
	}
	p.parent.l.Unlock()

	if b == nil {
		b = zero[:]
	}

	p.l.Lock()
	defer p.l.Unlock()

	p.data = append(p.data, b...)
	lenb := len(b)
	atomic.AddInt64(&p.parent.len, int64(lenb))

	if p.index != atomic.LoadUint32(&p.parent.rindex) {
		p.parent.l.Lock()
		if p.parent.rerr != nil {
			err = p.parent.rerr
			p.parent.l.Unlock()
			p = nil
			return lenb, err
		}
		if p.parent.eindex != -1 && int32(p.index) > p.parent.eindex {
			p.parent.l.Unlock()
			p = nil
			err = ErrFailedPipe
			return lenb, err
		}
		p.parent.l.Unlock()
		return lenb, nil
	}
	p.rwait.Signal()
	for {
		p.parent.l.Lock()
		if p.parent.rerr != nil {
			err = p.parent.rerr
			p.parent.l.Unlock()
			p = nil
			n = lenb - len(p.data)
			return
		}
		if p.parent.eindex != -1 && int32(p.index) > p.parent.eindex {
			p.parent.l.Unlock()
			p = nil
			err = ErrFailedPipe
			n = lenb - len(p.data)
			return
		}
		p.parent.l.Unlock()

		if p.data == nil {
			n = lenb
			return
		}
		p.wwait.Wait()
	}
	return
}

type pipe struct {
	l     sync.Mutex
	data  []byte
	rwait sync.Cond
	wwait sync.Cond
	werr  error
}

func (p *piper) read(b []byte) (n int, err error) {
	p.l.Lock()
	defer p.l.Unlock()
	for {
		if p.data != nil {
			break
		}
		if p.werr != nil {
			err = p.werr
			return
		}
		p.rwait.Wait()
	}

	n = copy(b, p.data)
	p.data = p.data[n:]
	atomic.AddInt64(&p.parent.len, -int64(n))
	if len(p.data) == 0 {
		p.data = nil
		p.wwait.Signal()
	}
	return
}

var zero [0]byte

func (p *autoPipe) RClose(err error) {
	if err == nil {
		err = ErrClosedPipe
	}
	p.l.Lock()
	defer p.l.Unlock()
	p.rerr = err
	rindex := atomic.LoadUint32(&p.rindex)
	p.pipers[rindex].wwait.Signal()
}

func (p *autoPipe) WClose() {
	p.l.Lock()
	defer p.l.Unlock()
	p.werr = io.EOF
	p.rwait.Signal()
}

func (p *piper) WClose() {
	p.l.Lock()
	defer p.l.Unlock()
	p.werr = io.EOF
	p.rwait.Signal()
}

func (p *piper) EIndex() {
	p.parent.l.Lock()
	defer p.parent.l.Unlock()
	if p.parent.eindex == -1 {
		p.parent.eindex = int32(p.index)
	} else {
		if int32(p.index) < p.parent.eindex {
			p.parent.eindex = int32(p.index)
		}
	}
}

func NewAutoPipe(threads int) (r *autoPipe) {
	r = new(autoPipe)
	r.Threads = make(chan bool, threads)
	r.eindex = -1
	r.rwait.L = &r.l
	return
}

func (parent *autoPipe) NewPiper(index uint32) (r *piper) {
	r = &piper{
		pipe:   newPipe(),
		index:  index,
		parent: parent,
	}
	parent.l.Lock()
	defer parent.l.Unlock()
	if length := len(parent.pipers); length <= int(index) {
		parent.pipers = append(parent.pipers, make([]*piper, int(index)+1-length)...)
	}
	parent.pipers[index] = r
	parent.rwait.Signal()
	return
}

func newPipe() *pipe {
	p := new(pipe)
	p.rwait.L = &p.l
	p.wwait.L = &p.l
	return p
}

func (ap *autoPipe) GetEIndex() int32 {
	ap.l.Lock()
	defer ap.l.Unlock()
	return ap.eindex
}

func (ap *autoPipe) Len() int64 {
	return atomic.LoadInt64(&ap.len)
}
