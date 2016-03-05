package direct

import (
	"net"
	"net/http"
)

type Tranport struct {
	http.Transport
	dailer Dailer
}

func (t *Tranport) Dial(network, address string) (conn net.Conn, err error) {
	return t.dailer.Dial(network, address)
}
