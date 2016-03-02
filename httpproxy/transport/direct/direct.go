package direct

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
)

const (
	maxDialTries = 2
)

var (
	ErrLoopbackAddr = errors.New("dial to loopback addr")
)

type Dailer struct {
	net.Dialer
	DNSCache        lrucache.Cache
	DNSCacheExpires time.Duration
	LoopbackAddrs   map[string]struct{}
}

func (d *Dailer) Dial(network, address string) (conn net.Conn, err error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if d.DNSCache != nil {
			if addr, ok := d.DNSCache.Get(address); ok {
				address = addr.(string)
			} else {
				if host, port, err := net.SplitHostPort(address); err == nil {
					if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
						ip := ips[0].String()
						if _, ok := d.LoopbackAddrs[ip]; ok {
							return nil, net.InvalidAddrError(fmt.Sprintf("Invaid DNS Record: %s(%s)", host, ip))
						}
						addr := net.JoinHostPort(ip, port)
						d.DNSCache.Set(address, addr, time.Now().Add(d.DNSCacheExpires))
						glog.V(3).Infof("direct Dial cache dns %#v=%#v", address, addr)
						address = addr
					}
				}
			}
		}
	default:
		break
	}
	for i := 0; i < maxDialTries; i++ {
		conn, err = d.Dialer.Dial(network, address)
		if err == nil || i == maxDialTries-1 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return conn, err
}

func NewTransport() http.RoundTripper {
	d := &Dailer{}
	d.Timeout = time.Duration(10) * time.Second
	d.KeepAlive = time.Duration(60) * time.Second
	d.DNSCache = lrucache.NewMultiLRUCache(4, 8192)
	d.DNSCacheExpires = 1 * time.Hour
	d.LoopbackAddrs = make(map[string]struct{})

	// d.LoopbackAddrs["127.0.0.1"] = struct{}{}
	d.LoopbackAddrs["::1"] = struct{}{}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			switch addr.Network() {
			case "ip":
				d.LoopbackAddrs[addr.String()] = struct{}{}
			}
		}
	}

	return &http.Transport{
		Dial: d.Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		},
		TLSHandshakeTimeout: 2 * time.Second,
		MaxIdleConnsPerHost: 16,
		DisableCompression:  false,
	}
}
