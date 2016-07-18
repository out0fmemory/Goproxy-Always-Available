package helpers

import (
	"net"
	"net/http"
	"net/url"

	"../proxy"
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

func alwaysClose(conn net.Conn, idle bool) bool {
	return true
}

func TryCloseConnections(tr http.RoundTripper) bool {
	type closer1 interface {
		CloseConnections(func(conn net.Conn, idle bool) bool)
	}

	type closer2 interface {
		CloseIdleConnections()
	}

	if t, ok := tr.(closer1); ok {
		t.CloseConnections(alwaysClose)
		return true
	}

	if t, ok := tr.(closer2); ok {
		t.CloseIdleConnections()
	}

	return false
}

func TryCloseConnectionByRemoteAddr(tr http.RoundTripper, addr string) bool {
	type closer1 interface {
		CloseConnections(func(conn net.Conn, idle bool) bool)
	}
	if t, ok := tr.(closer1); ok {
		f := func(conn net.Conn, idle bool) bool {
			return conn != nil && conn.RemoteAddr().String() == addr
		}
		t.CloseConnections(f)
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

func ConfigureProxy(t *http.Transport, fixedURL *url.URL, forward proxy.Dialer, resolver proxy.Resolver) error {
	switch fixedURL.Scheme {
	case "socks", "socks5", "sock4":
		if forward == nil {
			forward = proxy.Direct
		}

		dialer, err := proxy.FromURL(fixedURL, forward, resolver)
		if err != nil {
			return err
		}

		t.Dial = dialer.Dial
		t.DialTLS = nil
		t.Proxy = nil
	default:
		t.Dial = nil
		t.DialTLS = nil
		t.Proxy = http.ProxyURL(fixedURL)
	}

	return nil
}
