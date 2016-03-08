package iplist

import (
	"crypto/tls"
	"math/rand"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"

	"../../../httpproxy"
)

type Dialer struct {
	net.Dialer
	TLSConfig          *tls.Config
	Window             int
	Blacklist          map[string]struct{}
	hosts              *httpproxy.HostMatcher
	hostMap            *HostMap
	connTCPDuration    lrucache.Cache
	connTLSDuration    lrucache.Cache
	connExpireDuration time.Duration
}

func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	glog.V(2).Infof("Dail(%#v, %#v)...", network, address)
	switch network {
	case "tcp", "tcp4", "tcp6":
		if host, port, err := net.SplitHostPort(address); err == nil {
			if alias0, ok := d.hosts.Lookup(host); ok {
				alias := alias0.(string)
				if hosts, err := d.hostMap.Lookup(alias); err == nil {
					addrs := make([]string, len(hosts))
					for i, host := range hosts {
						addrs[i] = net.JoinHostPort(host, port)
					}
					return d.dialMulti(network, addrs)
				}
			}
		}
	default:
		break
	}
	return d.Dialer.Dial(network, address)
}

func (d *Dialer) DialTLS(network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if host, port, err := net.SplitHostPort(address); err == nil {
			if alias0, ok := d.hosts.Lookup(host); ok {
				alias := alias0.(string)
				if hosts, err := d.hostMap.Lookup(alias); err == nil {
					config := &tls.Config{
						InsecureSkipVerify: true,
						ServerName:         address,
					}
					if strings.Contains(address, ".appspot.com") ||
						strings.Contains(address, ".google") ||
						strings.Contains(address, ".gstatic.com") ||
						strings.Contains(address, ".ggpht.com") {
						config.ServerName = "www.bing.com"
						config.CipherSuites = []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA}
					}

					addrs := make([]string, len(hosts))
					for i, host := range hosts {
						addrs[i] = net.JoinHostPort(host, port)
					}
					return d.dialMultiTLS(network, addrs, config)
				}
			}
		}
	default:
		break
	}
	return tls.DialWithDialer(&d.Dialer, network, address, d.TLSConfig)
}

func (d *Dialer) dialMulti(network string, addrs []string) (net.Conn, error) {
	type racer struct {
		conn net.Conn
		err  error
	}

	length := len(addrs)
	if d.Window < length {
		length = d.Window
	}

	addrs = pickupAddrs(addrs, length, d.connTCPDuration)
	lane := make(chan racer, length)

	for _, addr := range addrs {
		go func(addr string, c chan<- racer) {
			start := time.Now()
			conn, err := d.Dialer.Dial(network, addr)
			end := time.Now()
			if err == nil {
				d.connTCPDuration.Set(addr, end.Sub(start), end.Add(d.connExpireDuration))
			} else {
				d.connTCPDuration.Del(addr)
			}
			lane <- racer{conn, err}
		}(addr, lane)
	}

	var r racer
	for i := 0; i < length; i++ {
		r = <-lane
		if r.err == nil {
			go func(count int) {
				var r1 racer
				for ; count > 0; count-- {
					r1 = <-lane
					if r1.conn != nil {
						r1.conn.Close()
					}
				}
			}(length - 1 - i)
			return r.conn, nil
		}
	}
	return nil, r.err
}

func (d *Dialer) dialMultiTLS(network string, addrs []string, config *tls.Config) (net.Conn, error) {
	type racer struct {
		conn net.Conn
		err  error
	}

	length := len(addrs)
	if d.Window < length {
		length = d.Window
	}

	addrs = pickupAddrs(addrs, length, d.connTLSDuration)
	lane := make(chan racer, length)

	for _, addr := range addrs {
		go func(addr string, c chan<- racer) {
			start := time.Now()
			conn, err := d.Dialer.Dial(network, addr)
			if err != nil {
				lane <- racer{conn, err}
				return
			}

			if config == nil {
				config = &tls.Config{
					InsecureSkipVerify: true,
				}
			}

			tlsConn := tls.Client(conn, config)
			err = tlsConn.Handshake()

			end := time.Now()
			if err == nil {
				d.connTLSDuration.Set(addr, end.Sub(start), end.Add(d.connExpireDuration))
			} else {
				d.connTLSDuration.Del(addr)
			}

			lane <- racer{tlsConn, err}
		}(addr, lane)
	}

	var r racer
	for i := 0; i < length; i++ {
		r = <-lane
		if r.err == nil {
			go func(count int) {
				var r1 racer
				for ; count > 0; count-- {
					r1 = <-lane
					if r1.conn != nil {
						r1.conn.Close()
					}
				}
			}(length - 1 - i)
			return r.conn, nil
		}
	}
	return nil, r.err
}

type racer struct {
	addr     string
	duration time.Duration
}

type racers []racer

func (r racers) Len() int {
	return len(r)
}

func (r racers) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r racers) Less(i, j int) bool {
	return r[i].duration < r[j].duration
}

func pickupAddrs(addrs []string, n int, duration lrucache.Cache) []string {
	if len(addrs) <= n {
		return addrs
	}

	goodAddrs := make([]racer, 0)
	unknownAddrs := make([]string, 0)

	for _, addr := range addrs {
		d, ok := duration.GetQuiet(addr)
		if ok {
			d1, ok := d.(time.Duration)
			if !ok {
				glog.Errorf("%#v for %#v is not a time.Duration", d, addr)
			} else {
				goodAddrs = append(goodAddrs, racer{addr, d1})
			}
		} else {
			unknownAddrs = append(unknownAddrs, addr)
		}
	}

	sort.Sort(racers(goodAddrs))

	if len(goodAddrs) > n/2 {
		goodAddrs = goodAddrs[:n/2]
	}

	goodAddrs1 := make([]string, len(goodAddrs), n)
	for i, r := range goodAddrs {
		goodAddrs1[i] = r.addr
	}

	shuffle(unknownAddrs)
	if len(goodAddrs1)+len(unknownAddrs) > n {
		unknownAddrs = unknownAddrs[:n-len(goodAddrs1)]
	}

	return append(goodAddrs1, unknownAddrs...)
}

func shuffle(addrs []string) {
	for i := len(addrs) - 1; i >= 0; i-- {
		j := rand.Intn(i + 1)
		addrs[i], addrs[j] = addrs[j], addrs[i]
	}
}
