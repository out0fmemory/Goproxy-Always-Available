package helpers

import (
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/phuslu/glog"
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

func CloseConnections(tr http.RoundTripper) {
	f := func(_ net.Addr) bool { return true }

	switch tr.(type) {
	case *http.Transport:
		tr.(*http.Transport).CloseConnection(f)
	case *http2.Transport:
		tr.(*http2.Transport).CloseConnection(f)
	case *h2quic.RoundTripper:
		tr.(*h2quic.RoundTripper).CloseConnection(f)
	default:
		glog.Errorf("%T(%v) has not implement CloseConnection method", tr, tr)
	}
}

func CloseConnectionByRemoteHost(tr http.RoundTripper, host string) {
	if host1, _, err := net.SplitHostPort(host); err == nil {
		host = host1
	}

	f := func(raddr net.Addr) bool {
		if host1, _, err := net.SplitHostPort(raddr.String()); err == nil {
			return host == host1
		}
		return false
	}

	switch tr.(type) {
	case *http.Transport:
		tr.(*http.Transport).CloseConnection(f)
	case *http2.Transport:
		tr.(*http2.Transport).CloseConnection(f)
	case *h2quic.RoundTripper:
		tr.(*h2quic.RoundTripper).CloseConnection(f)
	default:
		glog.Errorf("%T(%v) has not implement CloseConnection method", tr, tr)
	}
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

func IsStaticRequest(req *http.Request) bool {
	switch path.Ext(req.URL.Path) {
	case "bmp", "gif", "ico", "jpeg", "jpg", "png", "tif", "tiff",
		"3gp", "3gpp", "avi", "f4v", "flv", "m4p", "mkv", "mp4",
		"mp4v", "mpv4", "rmvb", ".webp", ".js", ".css":
		return true
	case "":
		name := path.Base(req.URL.Path)
		if strings.Contains(name, "play") ||
			strings.Contains(name, "video") {
			return true
		}
	default:
		if req.Header.Get("Range") != "" ||
			strings.Contains(req.Host, "img.") ||
			strings.Contains(req.Host, "cache.") ||
			strings.Contains(req.Host, "video.") ||
			strings.Contains(req.Host, "static.") ||
			strings.HasPrefix(req.Host, "img") ||
			strings.HasPrefix(req.URL.Path, "/static") ||
			strings.HasPrefix(req.URL.Path, "/asset") ||
			strings.Contains(req.URL.Path, "static") ||
			strings.Contains(req.URL.Path, "asset") ||
			strings.Contains(req.URL.Path, "/cache/") {
			return true
		}
	}
	return false
}
