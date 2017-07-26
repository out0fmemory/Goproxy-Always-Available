// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"

	quic "github.com/phuslu/quic-go"
	"github.com/phuslu/quic-go/h2quic"
)

type QuicConn struct {
	quic.Stream
	Source net.Addr
	Addr   net.Addr
}

func (c *QuicConn) LocalAddr() net.Addr {
	return c.Source
}

func (c *QuicConn) RemoteAddr() net.Addr {
	return c.Addr
}

var _ net.Conn = &QuicConn{}

func QUIC(network, addr string, auth *Auth, forward Dialer, resolver Resolver) (Dialer, error) {
	var hostname string

	if host, _, err := net.SplitHostPort(addr); err == nil {
		hostname = host
	} else {
		hostname = addr
		addr = net.JoinHostPort(addr, "443")
	}

	s := &Quic{
		network:  network,
		addr:     addr,
		hostname: hostname,
		forward:  forward,
		resolver: resolver,
		transport: &h2quic.RoundTripper{
			DisableCompression: true,
			DialAddr: func(address string, tlsConfig *tls.Config, cfg *quic.Config) (quic.Session, error) {
				return quic.DialAddr(addr, tlsConfig, cfg)
			},
		},
	}
	if auth != nil {
		s.user = auth.User
		s.password = auth.Password
	}

	return s, nil
}

type Quic struct {
	user, password string
	network, addr  string
	hostname       string
	forward        Dialer
	resolver       Resolver
	transport      *h2quic.RoundTripper
}

// Dial connects to the address addr on the network net via the HTTPS proxy.
func (h *Quic) Dial(network, addr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return nil, errors.New("proxy: no support for QUIC proxy connections of type " + network)
	}

	req := &http.Request{
		Method: http.MethodConnect,
		Host:   addr,
		Header: http.Header{},
		URL: &url.URL{
			Scheme: "https",
			Host:   addr,
		},
	}

	resp, err := h.transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("proxy: failed to read greeting from HTTP proxy at " + h.addr + ": " + resp.Status)
	}

	conn := &QuicConn{
		Stream: resp.Body.(quic.Stream),
	}

	return conn, nil
}
