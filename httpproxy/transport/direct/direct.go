package direct

import (
	"net"
	"net/http"
)

type Tranport struct {
	http.Transport
	dialer Dialer
}

func (t *Tranport) Dial(network, address string) (conn net.Conn, err error) {
	return t.dialer.Dial(network, address)
}
