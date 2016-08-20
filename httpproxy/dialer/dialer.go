package dialer

import (
	"fmt"
	"net"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
)

const (
	DefaultRetryTimes     int           = 2
	DefaultRetryDelay     time.Duration = 100 * time.Millisecond
	DefaultDNSCacheExpiry time.Duration = time.Hour
	DefaultDNSCacheSize   uint          = 8 * 1024
)

type Dialer struct {
	Dialer interface {
		Dial(network, addr string) (net.Conn, error)
	}

	RetryTimes     int
	RetryDelay     time.Duration
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
			if addr, ok := d.DNSCache.Get(address); ok {
				address = addr.(string)
			} else {
				if host, port, err := net.SplitHostPort(address); err == nil {
					if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
						var ip string
						if d.BlackList != nil {
							for _, ip1 := range ips {
								if _, ok := d.BlackList.Get(ip1.String()); !ok {
									ip = ip1.String()
								}
							}
						} else {
							ip = ips[0].String()
						}

						if ip == "" {
							return nil, net.InvalidAddrError(fmt.Sprintf("Invaid DNS Record: %s(%+v)", host, ips))
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
			if ne, ok := err.(interface {
				Timeout() bool
			}); ok && ne.Timeout() {
				d.BlackList.Set(address, struct{}{}, time.Now().Add(5*time.Hour))
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
