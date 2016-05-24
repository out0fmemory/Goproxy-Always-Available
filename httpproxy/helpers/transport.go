package helpers

import (
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

func TryCloseConnections(tr http.RoundTripper) bool {
	type closer1 interface {
		CloseConnections()
	}

	type closer2 interface {
		CloseIdleConnections()
	}

	if t, ok := tr.(closer1); ok {
		t.CloseConnections()
		return true
	}

	if t, ok := tr.(closer2); ok {
		t.CloseIdleConnections()
	}

	return false
}
