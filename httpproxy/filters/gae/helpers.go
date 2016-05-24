package gae

import (
	"net/http"
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
