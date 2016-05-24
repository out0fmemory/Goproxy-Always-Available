package helpers

import (
	"net/http"
	"runtime"
	"testing"
)

func TestTryCloseConnections(t *testing.T) {
	if !TryCloseConnections(http.DefaultTransport) {
		t.Errorf("go %v net/http does not support CloseConnections()", runtime.Version)
	}
}
