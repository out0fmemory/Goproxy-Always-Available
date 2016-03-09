package direct

import (
	"net"
	"net/http"
)

type Transport struct {
	http.Transport
	Dialer
}

func (t *Transport) Dial(network, address string) (conn net.Conn, err error) {
	return t.Dialer.Dial(network, address)
}
