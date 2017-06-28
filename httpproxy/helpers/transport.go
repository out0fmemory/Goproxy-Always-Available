package helpers

import (
	"net"
	"net/http"
	"strconv"

	"github.com/phuslu/net/http2"
	"github.com/phuslu/quic-go/h2quic"
)

var (
	ReqWriteExcludeHeader = map[string]bool{
		"Vary":                true,
		"Via":                 true,
		"X-Forwarded-For":     true,
		"Proxy-Authorization": true,
		"Proxy-Connection":    true,
		"Upgrade":             true,
		"X-Chrome-Variations": true,
		"Connection":          true,
		"Cache-Control":       true,
	}
)

func CloseConnections(tr http.RoundTripper) bool {
	if t, ok := tr.(*http.Transport); ok {
		f := func(addr net.Addr, idle bool) bool {
			return true
		}
		t.CloseConnections(f)
		return true
	}
	if t, ok := tr.(*http2.Transport); ok {
		f := func(addr net.Addr, idle bool) bool {
			return true
		}
		t.CloseConnections(f)
		return true
	}
	if t, ok := tr.(*h2quic.RoundTripper); ok {
		f := func(addr net.Addr, idle bool) bool {
			return true
		}
		t.CloseConnections(f)
		return true
	}
	return false
}

func CloseConnectionByRemoteHost(tr http.RoundTripper, host string) bool {
	f := func(addr net.Addr, idle bool) bool {
		if host1, _, err := net.SplitHostPort(addr.String()); err == nil {
			return host == host1
		}
		return false
	}
	switch tr.(type) {
	case *http.Transport:
		tr.(*http.Transport).CloseConnections(f)
		return true
	case *http2.Transport:
		tr.(*http2.Transport).CloseConnections(f)
		return true
	case *h2quic.RoundTripper:
		tr.(*h2quic.RoundTripper).CloseConnections(f)
		return true
	}
	return false
}

func FixRequestURL(req *http.Request) {
	if req.URL.Host == "" {
		switch {
		case req.Host != "":
			req.URL.Host = req.Host
		case req.TLS != nil:
			req.URL.Host = req.TLS.ServerName
		}
	}
}

func FixRequestHeader(req *http.Request) {
	if req.ContentLength > 0 {
		if req.Header.Get("Content-Length") == "" {
			req.Header.Set("Content-Length", strconv.FormatInt(req.ContentLength, 10))
		}
	}
}

// CloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func CloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

func GetHostName(req *http.Request) string {
	if host, _, err := net.SplitHostPort(req.Host); err == nil {
		return host
	} else {
		return req.Host
	}
}
