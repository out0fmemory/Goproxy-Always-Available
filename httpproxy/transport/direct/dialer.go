package direct

import (
	"fmt"
	"net"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
)

const (
	DefaultRetryTimes     int           = 2
	DefaultRetryDelay     time.Duration = 100 * time.Millisecond
	DefaultDNSCacheExpiry time.Duration = time.Hour
	DefaultDNSCacheSize   uint          = 8 * 1024
)

type Dialer struct {
	net.Dialer

	RetryTimes     int
	RetryDelay     time.Duration
	DNSCache       lrucache.Cache
	DNSCacheExpiry time.Duration
	LoopbackAddrs  map[string]struct{}
	Level          int
}

func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	glog.V(3).Infof("Dail(%#v, %#v)", network, address)

	switch network {
	case "tcp", "tcp4", "tcp6":
		if d.DNSCache != nil {
			if addr, ok := d.DNSCache.Get(address); ok {
				address = addr.(string)
			} else {
				if host, port, err := net.SplitHostPort(address); err == nil {
					if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
						ip := ips[0].String()
						if d.LoopbackAddrs != nil {
							if _, ok := d.LoopbackAddrs[ip]; ok {
								return nil, net.InvalidAddrError(fmt.Sprintf("Invaid DNS Record: %s(%s)", host, ip))
							}
						}
						addr := net.JoinHostPort(ip, port)
						expiry := d.DNSCacheExpiry
						if expiry == 0 {
							expiry = DefaultDNSCacheExpiry
						}
						d.DNSCache.Set(address, addr, time.Now().Add(expiry))
						glog.V(3).Infof("direct Dial cache dns %#v=%#v", address, addr)
						address = addr
					}
				}
			}
		}
	default:
		break
	}

	if d.Level <= 1 {
		retry := d.RetryTimes
		if retry == 0 {
			retry = DefaultRetryTimes
		}

		for i := 0; i < retry; i++ {
			conn, err = d.Dialer.Dial(network, address)
			if err == nil || i == retry-1 {
				break
			}
			retryDelay := d.RetryDelay
			if retryDelay == 0 {
				retryDelay = DefaultRetryDelay
			}
			time.Sleep(retryDelay)
		}
		return conn, err
	} else {
		type racer struct {
			c net.Conn
			e error
		}

		lane := make(chan racer, d.Level)
		retry := (d.RetryTimes + d.Level - 1) / d.Level
		for i := 0; i < retry; i++ {
			for j := 0; j < d.Level; j++ {
				go func(addr string, c chan<- racer) {
					conn, err := d.Dialer.Dial(network, addr)
					lane <- racer{conn, err}
				}(address, lane)
			}

			var r racer
			for k := 0; k < d.Level; k++ {
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
					}(d.Level - 1 - k)
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
