package dialer

import (
	"fmt"
	"net"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
)

const (
	DefaultDNSCacheExpiry time.Duration = time.Hour
	DefaultDNSCacheSize   uint          = 8 * 1024
)

type Dialer struct {
	Dialer interface {
		Dial(network, addr string) (net.Conn, error)
	}

	DNSCache       lrucache.Cache
	DNSCacheExpiry time.Duration
	BlackList      lrucache.Cache
	Level          int
}

func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	glog.V(3).Infof("Dail(%#v, %#v)", network, address)

	switch network {
	case "tcp", "tcp4", "tcp6":
		if d.DNSCache != nil {
			if host, port, err := net.SplitHostPort(address); err == nil {
				var ips []net.IP
				if ips0, ok := d.DNSCache.Get(host); ok {
					ips, ok = ips0.([]net.IP)
					if !ok {
						glog.Warningf("DIALER: resolve %#v to %+v is %T, not []net.IP", host, ips0, ips0)
						break
					}
				} else {
					ips0, err := net.LookupIP(host)
					if err != nil {
						return nil, err
					}

					ips = ips[:0]
					for _, ip := range ips0 {
						if !d.inBlackList(ip.String()) {
							ips = append(ips, ip)
						}
					}

					if len(ips) == 0 {
						return nil, net.InvalidAddrError(fmt.Sprintf("Invaid DNS Record: %s(%+v)", address, ips0))
					}

					d.DNSCache.Set(host, ips, time.Now().Add(d.DNSCacheExpiry))
				}
				return d.dialMulti(network, address, ips, port)
			}
		}
	}

	return d.Dialer.Dial(network, address)
}

func (d *Dialer) dialMulti(network, address string, ips []net.IP, port string) (conn net.Conn, err error) {
	if d.Level <= 1 || len(ips) == 1 {
		for i, ip := range ips {
			addr := net.JoinHostPort(ip.String(), port)
			conn, err := d.Dialer.Dial(network, addr)
			if err != nil {
				if i < len(ips)-1 {
					continue
				} else {
					return nil, err
				}
			}
			return conn, nil
		}
	} else {
		type racer struct {
			c net.Conn
			e error
		}

		level := len(ips)
		if level > d.Level {
			level = d.Level
			ips = ips[:level]
		}

		lane := make(chan racer, level)
		for i := 0; i < level; i++ {
			go func(addr string, c chan<- racer) {
				conn, err := d.Dialer.Dial(network, addr)
				lane <- racer{conn, err}
			}(net.JoinHostPort(ips[i].String(), port), lane)
		}

		var r racer
		for j := 0; j < level; j++ {
			r = <-lane
			if r.e == nil {
				go func(count int) {
					var r1 racer
					for ; count > 0; count-- {
						r1 = <-lane
						if r1.c != nil {
							r1.c.Close()
						}
					}
				}(level - 1 - j)
				return r.c, nil
			}
		}
	}

	return nil, net.UnknownNetworkError("Unkown transport/direct error")
}

func (d *Dialer) inBlackList(host string) bool {
	if d.BlackList == nil {
		return false
	}
	_, ok := d.BlackList.Get(host)
	return ok
}
