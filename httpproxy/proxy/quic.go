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
	"time"

	"github.com/phuslu/glog"
	quic "github.com/phuslu/quic-go"
	"github.com/phuslu/quic-go/h2quic"
)

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
			QuicConfig: &quic.Config{
				HandshakeTimeout:              5 * time.Second,
				IdleTimeout:                   10 * time.Second,
				RequestConnectionIDTruncation: true,
				KeepAlive:                     true,
			},
			KeepAliveTimeout:      30 * time.Minute,
			IdleConnTimeout:       5 * time.Minute,
			ResponseHeaderTimeout: 5 * time.Second,
			DialAddr: func(address string, tlsConfig *tls.Config, cfg *quic.Config) (quic.Session, error) {
				return quic.DialAddr(addr, tlsConfig, cfg)
			},
			GetClientKey: func(_ string) string {
				return addr
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

	resp, err := h.transport.RoundTripOpt(req, h2quic.RoundTripOpt{OnlyCachedConn: true})
	if err != nil {
		glog.Warningf("%T.RoundTripOpt(%#v) error: %+v", h.transport, req.URL.String(), err)
		h.transport.Close()
		resp, err = h.transport.RoundTripOpt(req, h2quic.RoundTripOpt{OnlyCachedConn: false})
	}

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("proxy: failed to read greeting from HTTP proxy at " + h.addr + ": " + resp.Status)
	}

	stream, ok := resp.Body.(quic.Stream)
	if !ok || stream == nil {
		return nil, errors.New("proxy: failed to convert resp.Body to a quic.Stream")
	}

	return stream, nil
}
