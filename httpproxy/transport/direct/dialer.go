package direct

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
)

const (
	DefaultRetryTimes      int           = 2
	DefaultRetryDelay      time.Duration = 100 * time.Millisecond
	DefaultDNSCacheExpires time.Duration = time.Hour
	DefaultDNSCacheSize    uint          = 8 * 1024
)

type Dialer struct {
	net.Dialer

	RetryTimes           int
	RetryDelay           time.Duration
	DNSCacheExpires      time.Duration
	DNSCacheSize         uint
	DialConcurrentNumber int

	dnsCache lrucache.Cache
	loAddrs  map[string]struct{}
	once     sync.Once
}

func (d *Dialer) init() {
	d.once.Do(func() {
		if d.RetryTimes == 0 {
			d.RetryTimes = DefaultRetryTimes
		}
		if d.RetryDelay == 0 {
			d.RetryDelay = DefaultRetryDelay
		}

		if d.DNSCacheExpires == 0 {
			d.DNSCacheExpires = DefaultDNSCacheExpires
		}
		if d.DNSCacheSize == 0 {
			d.DNSCacheSize = DefaultDNSCacheSize
		}
		d.dnsCache = lrucache.NewLRUCache(d.DNSCacheSize)

		if d.DialConcurrentNumber == 0 {
			d.DialConcurrentNumber = 1
		}

		d.loAddrs = make(map[string]struct{})
		// d.LoopbackAddrs["127.0.0.1"] = struct{}{}
		d.loAddrs["::1"] = struct{}{}
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, addr := range addrs {
				switch addr.Network() {
				case "ip":
					d.loAddrs[addr.String()] = struct{}{}
				}
			}
		}
	})
}

func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	d.init()

	glog.V(3).Infof("Dail(%#v, %#v)", network, address)

	switch network {
	case "tcp", "tcp4", "tcp6":
		if addr, ok := d.dnsCache.Get(address); ok {
			address = addr.(string)
		} else {
			if host, port, err := net.SplitHostPort(address); err == nil {
				if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
					ip := ips[0].String()
					if d.loAddrs != nil {
						if _, ok := d.loAddrs[ip]; ok {
							return nil, net.InvalidAddrError(fmt.Sprintf("Invaid DNS Record: %s(%s)", host, ip))
						}
					}
					addr := net.JoinHostPort(ip, port)
					d.dnsCache.Set(address, addr, time.Now().Add(d.DNSCacheExpires))
					glog.V(3).Infof("direct Dial cache dns %#v=%#v", address, addr)
					address = addr
				}
			}
		}
	default:
		break
	}

	if d.DialConcurrentNumber <= 1 {
		for i := 0; i < d.RetryTimes; i++ {
			conn, err = d.Dialer.Dial(network, address)
			if err == nil || i == d.RetryTimes-1 {
				break
			}
			time.Sleep(d.RetryDelay)
		}
		return conn, err
	} else {
		type racer struct {
			c net.Conn
			e error
		}

		lane := make(chan racer, d.DialConcurrentNumber)
		retry := (d.RetryTimes + d.DialConcurrentNumber - 1) / d.DialConcurrentNumber
		for i := 0; i < retry; i++ {
			for j := 0; j < d.DialConcurrentNumber; j++ {
				go func(addr string, c chan<- racer) {
					conn, err := d.Dialer.Dial(network, addr)
					lane <- racer{conn, err}
				}(address, lane)
			}

			var r racer
			for k := 0; k < d.DialConcurrentNumber; k++ {
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
					}(d.DialConcurrentNumber - 1 - k)
					return r.c, nil
				}
			}

			if i == retry-1 {
				return nil, r.e
			}
		}
	}

	return nil, net.UnknownNetworkError("Unkown transport/direct error")
}
