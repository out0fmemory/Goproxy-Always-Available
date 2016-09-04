package helpers

import (
	"net"
	"net/http"
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
		f := func(conn net.Conn, idle bool) bool {
			return true
		}
		t.CloseConnections(f)
		return true
	}
	return false
}

func CloseConnectionByRemoteAddr(tr http.RoundTripper, addr string) bool {
	if t, ok := tr.(*http.Transport); ok {
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

func GetHostName(req *http.Request) string {
	if host, _, err := net.SplitHostPort(req.Host); err == nil {
		return host
	} else {
		return req.Host
	}
}
