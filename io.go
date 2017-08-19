package main

import (
	"io"
	"net"
	"net/http"
	"time"
)

type FlushWriter struct {
	w io.Writer
}

func (fw FlushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

type TCPListener struct {
	*net.TCPListener
}

func (ln TCPListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	tc.SetReadBuffer(32 * 1024)
	tc.SetWriteBuffer(32 * 1024)
	return tc, nil
}

type ConnWithData struct {
	net.Conn
	Data []byte
}

func (c *ConnWithData) Read(b []byte) (int, error) {
	if c.Data == nil {
		return c.Conn.Read(b)
	} else {
		n := copy(b, c.Data)
		if n < len(c.Data) {
			c.Data = c.Data[n:]
		} else {
			c.Data = nil
		}
		return n, nil
	}
}
