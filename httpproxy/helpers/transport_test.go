package helpers

import (
	"net"
	"net/http"
	"testing"

	"github.com/phuslu/net/http2"
	"github.com/phuslu/quic-go/h2quic"
)

func TestCloseConnections(t *testing.T) {
	tansports := []http.RoundTripper{
		http.DefaultTransport,
		&http2.Transport{},
		&h2quic.RoundTripper{},
	}

	type RoundTripperCloser interface {
		CloseConnection(f func(raddr net.Addr) bool)
	}

	for _, tr := range tansports {
		_, ok := tr.(RoundTripperCloser)
		if !ok {
			t.Errorf("%T(%v) CloseConnection()", tr, tr)
		}
		CloseConnections(tr)
	}
}
