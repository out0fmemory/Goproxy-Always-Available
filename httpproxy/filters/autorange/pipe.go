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
	len     int64
	rb      readBegin
	threads chan bool
	pipers  []*piper
	piperr  piperr // 如果某个 "range" 出错了，应改写这个值。用以告诉 read 端不要读 eindex 之后的 piper 上的数据。以及通知 eindex 之后 piper 申请停止被 write
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

type readBegin struct {
	yep  bool
	cond sync.Cond
}

type piperr struct {
	yep    bool
	eindex uint32
}

func (p *autoPipe) read(b []byte) (n int, err error) {
	if !p.rb.yep {
		p.l.Lock()
		p.rb.yep = true
		p.rb.cond.Signal()
		p.l.Unlock()
	}
	for {
		p.l.Lock()
		if p.piperr.yep && p.rindex > p.piperr.eindex {
			p.l.Unlock()
			return 0, ErrFailedPipe
		}
		if p.rindex < uint32(len(p.pipers)) {
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
		p.l.Lock()
		p.pipers[p.rindex] = nil
		p.rindex++
		p.l.Unlock()
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
	if p.parent.piperr.yep && p.index > p.parent.piperr.eindex {
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

	p.parent.l.Lock()
	if p.index != p.parent.rindex {
		if p.parent.rerr != nil {
			err = p.parent.rerr
			p.parent.l.Unlock()
			p = nil
			return lenb, err
		}
		if p.parent.piperr.yep && p.index > p.parent.piperr.eindex {
			p.parent.l.Unlock()
			p = nil
			err = ErrFailedPipe
			return lenb, err
		}
		p.parent.l.Unlock()
		return lenb, nil
	}
	p.parent.l.Unlock()

	p.rwait.Signal()
	for {
		p.parent.l.Lock()
		if p.parent.rerr != nil {
			err = p.parent.rerr
			p.parent.l.Unlock()
			n = lenb - len(p.data)
			p = nil
			return
		}
		if p.parent.piperr.yep && p.index > p.parent.piperr.eindex {
			p.parent.l.Unlock()
			n = lenb - len(p.data)
			p = nil
			err = ErrFailedPipe
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

func (p *autoPipe) rclose(err error) {
	if err == nil {
		err = ErrClosedPipe
	}
	p.l.Lock()
	defer p.l.Unlock()
	p.rerr = err
	if p.pipers == nil {
		p.rb.cond.Signal()
	} else {
		if p.rindex < uint32(len(p.pipers)) && p.pipers[p.rindex] != nil {
			p.pipers[p.rindex].wwait.Signal()
		}
	}
}

func (p *autoPipe) wclose() {
	p.l.Lock()
	defer p.l.Unlock()
	p.werr = io.EOF
	p.rwait.Signal()
}

func (p *autoPipe) newPiper(index uint32) (r *piper) {
	r = &piper{
		pipe:   newPipe(),
		index:  index,
		parent: p,
	}
	p.l.Lock()
	defer p.l.Unlock()
	if length := len(p.pipers); length <= int(index) {
		p.pipers = append(p.pipers, make([]*piper, int(index)+1-length)...)
	}
	p.pipers[index] = r
	p.rwait.Signal()
	return
}

func (p *autoPipe) fatalErr() bool {
	p.l.Lock()
	defer p.l.Unlock()
	if p.piperr.yep || p.rerr != nil {
		return true
	}
	return false
}

func (p *autoPipe) waitForReading() (err error) {
	p.l.Lock()
	defer p.l.Unlock()

	for {
		if p.rerr != nil {
			err = p.rerr
			break
		}
		if p.rb.yep {
			break
		}
		p.rb.cond.Wait()
	}
	return
}

func (p *autoPipe) threadHello() {
	p.threads <- true
}

func (p *autoPipe) threadBye() {
	<-p.threads
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
	if !p.parent.piperr.yep {
		p.parent.piperr.eindex = p.index
		p.parent.piperr.yep = true
	} else {
		if p.index < p.parent.piperr.eindex {
			p.parent.piperr.eindex = p.index
		}
	}
}

type autoPipeReader struct {
	p *autoPipe
}

func (r *autoPipeReader) Read(data []byte) (n int, err error) {
	return r.p.read(data)
}

func (r *autoPipeReader) Close() error {
	r.p.rclose(nil)
	return nil
}

type autoPipeWriter struct {
	p *autoPipe
}

func (w *autoPipeWriter) WaitForReading() (err error) {
	return w.p.waitForReading()
}

func (w *autoPipeWriter) NewPiper(index uint32) (r *piper) {
	return w.p.newPiper(index)
}

func (w *autoPipeWriter) Close() error {
	w.p.wclose()
	return nil
}

func (w *autoPipeWriter) FatalErr() bool {
	return w.p.fatalErr()
}

func (w *autoPipeWriter) ThreadHello() {
	w.p.threadHello()
}

func (w *autoPipeWriter) ThreadBye() {
	w.p.threadBye()
}

func (w *autoPipeWriter) Len() int64 {
	return atomic.LoadInt64(&w.p.len)
}

func newPipe() *pipe {
	p := new(pipe)
	p.rwait.L = &p.l
	p.wwait.L = &p.l
	return p
}

func AutoPipe(threads int) (r *autoPipeReader, w *autoPipeWriter) {
	ap := new(autoPipe)
	ap.threads = make(chan bool, threads)
	ap.rwait.L = &ap.l
	ap.rb.cond.L = &ap.l

	r = &autoPipeReader{ap}
	w = &autoPipeWriter{ap}
	return
}
