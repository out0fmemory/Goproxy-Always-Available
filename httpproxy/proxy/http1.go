// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
)

func HTTP1(network, addr string, auth *Auth, forward Dialer, resolver Resolver) (Dialer, error) {
	s := &http1{
		network:  network,
		addr:     addr,
		forward:  forward,
		resolver: resolver,
	}
	if auth != nil {
		s.user = auth.User
		s.password = auth.Password
	}

	return s, nil
}

type http1 struct {
	user, password string
	network, addr  string
	forward        Dialer
	resolver       Resolver
}

// Dial connects to the address addr on the network net via the HTTP1 proxy.
func (h *http1) Dial(network, addr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return nil, errors.New("proxy: no support for HTTP proxy connections of type " + network)
	}

	conn, err := h.forward.Dial(h.network, h.addr)
	if err != nil {
		return nil, err
	}
	closeConn := &conn
	defer func() {
		if closeConn != nil {
			(*closeConn).Close()
		}
	}()

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, errors.New("proxy: failed to parse port number: " + portStr)
	}
	if port < 1 || port > 0xffff {
		return nil, errors.New("proxy: port number out of range: " + portStr)
	}

	if h.resolver != nil {
		hosts, err := h.resolver.LookupHost(host)
		if err == nil && len(hosts) > 0 {
			host = hosts[0]
		}
	}

	b := new(bytes.Buffer)

	fmt.Fprintf(b, "CONNECT %s:%s HTTP/1.1\r\n", host, portStr)
	if h.user != "" {
		fmt.Fprintf(b, "Proxy-Authorization: Basic %s\r\n", base64.StdEncoding.EncodeToString([]byte(h.user+":"+h.password)))
	}
	io.WriteString(b, "\r\n")

	if _, err := conn.Write(b.Bytes()); err != nil {
		return nil, errors.New("proxy: failed to write greeting to HTTP proxy at " + h.addr + ": " + err.Error())
	}

	buf := make([]byte, 0, 2048)
	b1 := make([]byte, 1)

	for {
		_, err := conn.Read(b1)
		if err != nil {
			return nil, err
		}
		buf = append(buf, b1[0])
		if bytes.HasSuffix(buf, []byte("\r\n\r\n")) {
			break
		}
	}

	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(buf)), nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("proxy: failed to read greeting from HTTP proxy at " + h.addr + ": " + resp.Status)
	}

	closeConn = nil
	return conn, nil
}
