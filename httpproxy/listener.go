package httpproxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
)

const (
	backlog           = 1000
	ENV_PREFIX string = "LISTEN_FD,"
)

type filer interface {
	File() (*os.File, error)
}

type Listener interface {
	net.Listener
	filer

	Add(net.Conn) error
	Wait()
}

type racer struct {
	conn net.Conn
	err  error
	ref  bool
}

type listener struct {
	ln              net.Listener
	lane            chan racer
	keepAlivePeriod time.Duration
	stopped         bool
	once            *sync.Once
	mu              *sync.Mutex
	wg              *sync.WaitGroup
}

type conn struct {
	net.Conn
	wg *sync.WaitGroup
}

func (c *conn) Close() error {
	err := c.Conn.Close()
	if err == nil {
		c.wg.Done()
	}
	return err
}

type ListenOptions struct {
	TLSConfig       *tls.Config
	KeepAlivePeriod time.Duration
}

func ListenTCP(network, addr string, opts *ListenOptions) (Listener, error) {
	key := ENV_PREFIX + network + "://" + addr
	if s := os.Getenv(key); s != "" {
		fd, err := strconv.Atoi(s)
		if err != nil {
			return nil, err
		}

		fl, err := net.FileListener(os.NewFile(uintptr(fd), key))
		if err != nil {
			return nil, err
		}

		var ln net.Listener
		if opts != nil && opts.TLSConfig != nil {
			ln = tls.NewListener(fl, opts.TLSConfig)
		} else {
			ln = fl
		}

		var keepAlivePeriod time.Duration
		if opts != nil && opts.KeepAlivePeriod > 0 {
			keepAlivePeriod = opts.KeepAlivePeriod
		}

		l := &listener{
			ln:              ln,
			lane:            make(chan racer, backlog),
			stopped:         false,
			keepAlivePeriod: keepAlivePeriod,
			once:            new(sync.Once),
			mu:              new(sync.Mutex),
			wg:              new(sync.WaitGroup),
		}

		go l.startListen()

		return l, nil
	} else {
		laddr, err := net.ResolveTCPAddr(network, addr)
		if err != nil {
			return nil, err
		}

		ln0, err := net.ListenTCP(network, laddr)
		if err != nil {
			return nil, err
		}

		if f, err := ln0.File(); err == nil {
			os.Setenv(key, strconv.FormatUint(uint64(f.Fd()), 10))
		}

		var ln net.Listener
		if opts != nil && opts.TLSConfig != nil {
			ln = tls.NewListener(ln0, opts.TLSConfig)
		} else {
			ln = ln0
		}

		var keepAlivePeriod time.Duration
		if opts != nil && opts.KeepAlivePeriod > 0 {
			keepAlivePeriod = opts.KeepAlivePeriod
		}

		l := &listener{
			ln:              ln,
			lane:            make(chan racer, backlog),
			stopped:         false,
			keepAlivePeriod: keepAlivePeriod,
			once:            new(sync.Once),
			mu:              new(sync.Mutex),
			wg:              new(sync.WaitGroup),
		}

		go l.startListen()

		return l, nil
	}
}

func StartProcess() (*os.Process, error) {
	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return nil, err
	}

	files := make([]*os.File, 0)
	files = append(files, os.Stdin)
	files = append(files, os.Stdout)
	files = append(files, os.Stderr)

	for _, key := range os.Environ() {
		if strings.HasPrefix(key, ENV_PREFIX) {
			parts := strings.SplitN(key, "=", 2)
			if fd, err := strconv.Atoi(parts[1]); err == nil {
				if err = noCloseOnExec(uintptr(fd)); err != nil {
					files = append(files, os.NewFile(uintptr(fd), key))
				}
			}
		}
	}

	return os.StartProcess(argv0, os.Args, &os.ProcAttr{
		Dir:   path.Dir(argv0),
		Env:   os.Environ(),
		Files: files,
		Sys:   &syscall.SysProcAttr{},
	})
}

func (l *listener) startListen() {
	var tempDelay time.Duration
	for {
		conn, err := l.ln.Accept()
		l.lane <- racer{conn, err, true}
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
				glog.Infof("http: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return
		}
	}
}

func (l *listener) Accept() (c net.Conn, err error) {
	r := <-l.lane
	if r.err != nil {
		return r.conn, r.err
	}

	if l.keepAlivePeriod > 0 {
		if tc, ok := r.conn.(*net.TCPConn); ok {
			tc.SetKeepAlive(true)
			tc.SetKeepAlivePeriod(l.keepAlivePeriod)
		}
	}

	if tc, ok := r.conn.(*tls.Conn); ok && !r.ref {
		return tc, nil
	}

	l.wg.Add(1)
	return &conn{r.conn, l.wg}, nil
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

func (l *listener) File() (*os.File, error) {
	if f, ok := l.ln.(filer); ok {
		return f.File()
	}
	return nil, fmt.Errorf("%T does not has func File()", l.ln)
}

func (l *listener) Add(conn net.Conn) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.stopped {
		return fmt.Errorf("%#v already closed", l)
	}

	l.lane <- racer{conn, nil, false}

	return nil
}

func (l *listener) Wait() {
	l.wg.Wait()
}
