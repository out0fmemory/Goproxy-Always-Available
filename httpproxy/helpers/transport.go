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

func alwaysClose(raddr net.Addr, laddr net.Addr, idle bool) bool {
	return true
}

func TryCloseConnections(tr http.RoundTripper) bool {
	type closer1 interface {
		CloseConnections(func(raddr net.Addr, laddr net.Addr, idle bool) bool)
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
