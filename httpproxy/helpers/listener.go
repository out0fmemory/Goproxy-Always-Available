package helpers

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/phuslu/glog"
)

const (
	backlog = 1024
)

type Listener interface {
	net.Listener

	Add(net.Conn) error
}

type connRacer struct {
	conn net.Conn
	err  error
}

type listener struct {
	ln              net.Listener
	lane            chan connRacer
	keepAlivePeriod time.Duration
	readBufferSize  int
	writeBufferSize int
	stopped         bool
	once            sync.Once
	mu              sync.Mutex
}

type ListenOptions struct {
	TLSConfig       *tls.Config
	KeepAlivePeriod time.Duration
	ReadBufferSize  int
	WriteBufferSize int
}

func ListenTCP(network, addr string, opts *ListenOptions) (Listener, error) {
	laddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, err
	}

	ln0, err := net.ListenTCP(network, laddr)
	if err != nil {
		return nil, err
	}

	var ln net.Listener
	if opts != nil && opts.TLSConfig != nil {
		ln = tls.NewListener(ln0, opts.TLSConfig)
	} else {
		ln = ln0
	}

	var keepAlivePeriod time.Duration
	var readBufferSize, writeBufferSize int
	if opts != nil {
		if opts.KeepAlivePeriod > 0 {
			keepAlivePeriod = opts.KeepAlivePeriod
		}
		if opts.ReadBufferSize > 0 {
			readBufferSize = opts.ReadBufferSize
		}
		if opts.WriteBufferSize > 0 {
			writeBufferSize = opts.WriteBufferSize
		}
	}

	l := &listener{
		ln:              ln,
		lane:            make(chan connRacer, backlog),
		stopped:         false,
		keepAlivePeriod: keepAlivePeriod,
		readBufferSize:  readBufferSize,
		writeBufferSize: writeBufferSize,
	}

	return l, nil

}

func (l *listener) Accept() (c net.Conn, err error) {
	l.once.Do(func() {
		go func() {
			var tempDelay time.Duration
			for {
				conn, err := l.ln.Accept()
				l.lane <- connRacer{conn, err}
				if err != nil {
					if ne, ok := err.(net.Error); ok && ne.Temporary() {
						if tempDelay == 0 {
							tempDelay = 5 * time.Millisecond
						} else {
							tempDelay *= 2
						}
						if max := 1 * time.Second; tempDelay > max {
							tempDelay = max
						}
						glog.Warningf("httpproxy.Listener: Accept error: %v; retrying in %v", err, tempDelay)
						time.Sleep(tempDelay)
						continue
					}
					return
				}
			}
		}()
	})

	r := <-l.lane
	if r.err != nil {
		return r.conn, r.err
	}

	if l.keepAlivePeriod > 0 || l.readBufferSize > 0 || l.writeBufferSize > 0 {
		if tc, ok := r.conn.(*net.TCPConn); ok {
			if l.keepAlivePeriod > 0 {
				tc.SetKeepAlive(true)
				tc.SetKeepAlivePeriod(l.keepAlivePeriod)
			}
			if l.readBufferSize > 0 {
				tc.SetReadBuffer(l.readBufferSize)
			}
			if l.writeBufferSize > 0 {
				tc.SetWriteBuffer(l.writeBufferSize)
			}
		}
	}

	return r.conn, nil
}

func (l *listener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		return nil
	}
	l.stopped = true
	close(l.lane)
	return l.ln.Close()
}

func (l *listener) Addr() net.Addr {
	return l.ln.Addr()
}

func (l *listener) Add(conn net.Conn) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.stopped {
		return fmt.Errorf("%#v already closed", l)
	}

	l.lane <- connRacer{conn, nil}

	return nil
}
