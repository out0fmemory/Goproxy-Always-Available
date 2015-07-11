package direct

import (
	"fmt"
	"net"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
)

type Dailer struct {
	net.Dialer
	DNSCache        lrucache.Cache
	DNSCacheExpires time.Duration
	Blacklist       map[string]struct{}
}

func (d *Dailer) Dial(network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if d.DNSCache != nil {
			if addr, ok := d.DNSCache.Get(address); ok {
				address = addr.(string)
			} else {
				if host, port, err := net.SplitHostPort(address); err == nil {
					if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
						ip := ips[0].String()
						if _, ok := d.Blacklist[ip]; ok {
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
	return d.Dialer.Dial(network, address)
}
