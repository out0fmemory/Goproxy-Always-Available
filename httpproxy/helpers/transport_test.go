package helpers

import (
	"net/http"
	"runtime"
	"testing"
)

func TestCloseConnections(t *testing.T) {
	if !CloseConnections(http.DefaultTransport) {
		t.Errorf("go %+v net/http does not support CloseConnections()", runtime.Version())
	}
}
