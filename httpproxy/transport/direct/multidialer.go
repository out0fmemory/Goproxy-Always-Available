package direct

import (
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
	"github.com/miekg/dns"

	"../../../httpproxy"
)

type MultiDialer struct {
	net.Dialer
	TLSConfig            *tls.Config
	DialConcurrentNumber int
	Blacklist            map[string]struct{}
	Matcher              *httpproxy.HostMatcher
	HostMap              struct {
		lists      map[string][]string
		dnsservers []string
		blacklist  *httpproxy.HostMatcher
		dnsCache   lrucache.Cache
		dualStack  bool
	}
	connTCPDuration    lrucache.Cache
	connTLSDuration    lrucache.Cache
	connExpireDuration time.Duration
}

func (d *MultiDialer) lookupHost(name string) (hosts []string, err error) {
	hs, err := net.LookupHost(name)
	if err != nil {
		return hs, err
	}

	hosts = make([]string, 0)
	for _, h := range hs {
		if !d.HostMap.dualStack && strings.Contains(h, ":") {
			continue
		}
		if !d.HostMap.blacklist.Match(h) {
			hosts = append(hosts, h)
		}
	}

	return hosts, nil
}

func (d *MultiDialer) lookupHost2(name string, dnsserver string) (hosts []string, err error) {
	m := &dns.Msg{}
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)

	r, err := dns.Exchange(m, dnsserver+":53")
	if err != nil {
		return nil, err
	}

	if len(r.Answer) < 1 {
		return nil, errors.New("no Answer")
	}

	hosts = []string{}

	for _, rr := range r.Answer {
		if a, ok := rr.(*dns.A); ok {
			ip := a.A.String()
			hosts = append(hosts, ip)
		}
	}

	return hosts, nil
}

func (d *MultiDialer) Lookup(name string) (hosts []string, err error) {
	list, ok := d.HostMap.lists[name]
	if !ok {
		return nil, fmt.Errorf("iplist %#v not exists", name)
	}

	hostSet := make(map[string]struct{}, 0)
	expire := time.Now().Add(24 * time.Hour)
	for _, addr := range list {
		var hs []string
		if hs0, ok := d.HostMap.dnsCache.Get(addr); ok {
			hs = hs0.([]string)
		} else {
			hs, err = d.lookupHost(addr)
			if err != nil {
				glog.Warningf("lookupHost(%#v) error: %s", addr, err)
				continue
			}
			glog.V(2).Infof("Lookup(%#v) return %v", addr, hs)
			d.HostMap.dnsCache.Set(addr, hs, expire)
		}
		for _, h := range hs {
			hostSet[h] = struct{}{}
		}
	}

	if len(hostSet) == 0 {
		return nil, err
	}

	hosts = make([]string, 0)
	for h, _ := range hostSet {
		hosts = append(hosts, h)
	}

	return hosts, nil
}

func (d *MultiDialer) ExpandList(name string) error {
	list, ok := d.HostMap.lists[name]
	if !ok {
		return fmt.Errorf("iplist %#v not exists", name)
	}

	expire := time.Now().Add(24 * time.Hour)
	for _, addr := range list {
		if regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).MatchString(addr) {
			continue
		}

		hostSet := make(map[string]struct{}, 0)
		for _, ds := range d.HostMap.dnsservers {
			hs, err := d.lookupHost2(addr, ds)
			if err != nil {
				glog.V(2).Infof("lookupHost2(%#v) error: %s", addr, err)
				continue
			}
			glog.V(2).Infof("ExpandList(%#v) %#v return %v", addr, ds, hs)
			for _, h := range hs {
				hostSet[h] = struct{}{}
			}
		}

		if len(hostSet) == 0 {
			continue
		}

		if hs, ok := d.HostMap.dnsCache.Get(addr); ok {
			hs1 := hs.([]string)
			for _, h := range hs1 {
				hostSet[h] = struct{}{}
			}
		}

		hosts := make([]string, 0)
		for h, _ := range hostSet {
			hosts = append(hosts, h)
		}

		d.HostMap.dnsCache.Set(addr, hosts, expire)
	}

	return nil
}

func (d *MultiDialer) Dial(network, address string) (net.Conn, error) {
	glog.V(2).Infof("Dail(%#v, %#v)...", network, address)
	switch network {
	case "tcp", "tcp4", "tcp6":
		if host, port, err := net.SplitHostPort(address); err == nil {
			if alias0, ok := d.Matcher.Lookup(host); ok {
				alias := alias0.(string)
				if hosts, err := d.Lookup(alias); err == nil {
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

func (d *MultiDialer) DialTLS(network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		if host, port, err := net.SplitHostPort(address); err == nil {
			if alias0, ok := d.Matcher.Lookup(host); ok {
				alias := alias0.(string)
				if hosts, err := d.Lookup(alias); err == nil {
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

func (d *MultiDialer) dialMulti(network string, addrs []string) (net.Conn, error) {
	type racer struct {
		conn net.Conn
		err  error
	}

	length := len(addrs)
	if d.DialConcurrentNumber < length {
		length = d.DialConcurrentNumber
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

func (d *MultiDialer) dialMultiTLS(network string, addrs []string, config *tls.Config) (net.Conn, error) {
	type racer struct {
		conn net.Conn
		err  error
	}

	length := len(addrs)
	if d.DialConcurrentNumber < length {
		length = d.DialConcurrentNumber
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
