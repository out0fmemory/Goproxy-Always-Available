package ssh2

import (
	"net"
	"runtime"
	"strconv"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"golang.org/x/crypto/ssh"
)

type Server struct {
	Address      string
	ClientConfig *ssh.ClientConfig
}

type Servers struct {
	servers    []Server
	sshClients lrucache.Cache
}

func (ss *Servers) Dial(network, address string) (net.Conn, error) {
	var err error

	i := 0
	c, ok := ss.sshClients.Get(strconv.Itoa(i))
	if !ok {
		c, err = ssh.Dial(network, ss.servers[i].Address, ss.servers[i].ClientConfig)
		if err != nil {
			return nil, err
		}
		runtime.SetFinalizer(c, func(c *ssh.Client) { c.Close() })
		ss.sshClients.Set(strconv.Itoa(i), c, time.Time{})
	}

	conn := c.(*ssh.Client)

	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}

	raddr := &net.TCPAddr{IP: ips[0], Port: port}

	return conn.DialTCP(network, nil, raddr)
}
