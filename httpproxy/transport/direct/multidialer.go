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
	IPv6Only        bool
	TLSConfig       *tls.Config
	Site2Alias      *httpproxy.HostMatcher
	IPBlackList     *httpproxy.HostMatcher
	HostMap         map[string][]string
	DNSServers      []net.IP
	DNSCache        lrucache.Cache
	DNSCacheExpiry  time.Duration
	TCPConnDuration lrucache.Cache
	TLSConnDuration lrucache.Cache
	ConnExpiry      time.Duration
	Level           int
}

func (d *MultiDialer) LookupHost(name string) (addrs []string, err error) {
	hs, err := net.LookupHost(name)
	if err != nil {
		return hs, err
	}

	addrs = make([]string, 0)
	for _, h := range hs {
		if d.IPBlackList.Match(h) {
			continue
		}

		if strings.Contains(h, ":") {
			if d.IPv6Only {
				addrs = append(addrs, h)
			}
		} else {
			addrs = append(addrs, h)
		}
	}

	return addrs, nil
}

func (d *MultiDialer) LookupHost2(name string, dnsserver net.IP) (addrs []string, err error) {
	m := &dns.Msg{}

	if d.IPv6Only {
		m.SetQuestion(dns.Fqdn(name), dns.TypeAAAA)
	} else {
		m.SetQuestion(dns.Fqdn(name), dns.TypeANY)
	}

	r, err := dns.Exchange(m, dnsserver.String()+":53")
	if err != nil {
		return nil, err
	}

	if len(r.Answer) < 1 {
		return nil, errors.New("no Answer")
	}

	addrs = []string{}

	for _, rr := range r.Answer {
		if d.IPv6Only {
			if aaaa, ok := rr.(*dns.AAAA); ok {
				addrs = append(addrs, aaaa.AAAA.String())
			}
		} else {
			if a, ok := rr.(*dns.A); ok {
				addrs = append(addrs, a.A.String())
			}
		}
	}

	return addrs, nil
}

func (d *MultiDialer) LookupAlias(alias string) (addrs []string, err error) {
	names, ok := d.HostMap[alias]
	if !ok {
		return nil, fmt.Errorf("alias %#v not exists", alias)
	}

	seen := make(map[string]struct{}, 0)
	expiry := time.Now().Add(d.DNSCacheExpiry)
	for _, name := range names {
		var addrs0 []string
		if addrs1, ok := d.DNSCache.Get(name); ok {
			addrs0 = addrs1.([]string)
		} else {
			if regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).MatchString(name) || strings.Contains(name, ":") {
				addrs0 = []string{name}
			} else {
				addrs0, err = d.LookupHost2(name, d.DNSServers[0])
				if err != nil {
					glog.Warningf("LookupHost(%#v) error: %s", name, err)
					continue
				}
			}

			glog.V(2).Infof("LookupHost(%#v) return %v", name, addrs0)
			d.DNSCache.Set(name, addrs0, expiry)
		}
		for _, addr := range addrs0 {
			seen[addr] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil, err
	}

	addrs = make([]string, 0)
	for addr, _ := range seen {
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

func (d *MultiDialer) ExpandAlias(alias string) error {
	names, ok := d.HostMap[alias]
	if !ok {
		return fmt.Errorf("alias %#v not exists", alias)
	}

	expire := time.Now().Add(24 * time.Hour)
	for _, name := range names {
		if regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).MatchString(name) {
			continue
		}

		seen := make(map[string]struct{}, 0)
		for _, dnsserver := range d.DNSServers {
			addrs, err := d.LookupHost2(name, dnsserver)
			if err != nil {
				glog.V(2).Infof("LookupHost2(%#v) error: %s", name, err)
				continue
			}
			glog.V(2).Infof("ExpandList(%#v) %#v return %v", name, dnsserver, addrs)
			for _, addr := range addrs {
				seen[addr] = struct{}{}
			}
		}

		if len(seen) == 0 {
			continue
		}

		if addrs, ok := d.DNSCache.Get(name); ok {
			addrs1 := addrs.([]string)
			for _, addr := range addrs1 {
				seen[addr] = struct{}{}
			}
		}

		addrs := make([]string, 0)
		for addr, _ := range seen {
			addrs = append(addrs, addr)
		}

		d.DNSCache.Set(name, addrs, expire)
	}

	return nil
}

func (d *MultiDialer) Dial(network, address string) (net.Conn, error) {
	glog.V(3).Infof("Dail(%#v, %#v)...", network, address)
	switch network {
	case "tcp", "tcp4", "tcp6":
		if host, port, err := net.SplitHostPort(address); err == nil {
			if alias0, ok := d.Site2Alias.Lookup(host); ok {
				alias := alias0.(string)
				if hosts, err := d.LookupAlias(alias); err == nil {
					addrs := make([]string, len(hosts))
					for i, host := range hosts {
						addrs[i] = net.JoinHostPort(host, port)
					}
					if d.IPv6Only {
						network = "tcp6"
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
	glog.V(3).Infof("DialTLS(%#v, %#v)...", network, address)
	switch network {
	case "tcp", "tcp4", "tcp6":
		if host, port, err := net.SplitHostPort(address); err == nil {
			if alias0, ok := d.Site2Alias.Lookup(host); ok {
				alias := alias0.(string)
				if hosts, err := d.LookupAlias(alias); err == nil {
					config := &tls.Config{
						InsecureSkipVerify: true,
						ServerName:         address,
					}
					if strings.Contains(address, ".appspot.com") ||
						strings.Contains(address, ".google") ||
						strings.Contains(address, ".gstatic.com") ||
						strings.Contains(address, ".ggpht.com") {
						config = defaultTLSConfigForGoogle
					}

					addrs := make([]string, len(hosts))
					for i, host := range hosts {
						addrs[i] = net.JoinHostPort(host, port)
					}
					if d.IPv6Only {
						network = "tcp6"
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
	glog.V(3).Infof("dialMulti(%v, %v)", network, addrs)
	type racer struct {
		c net.Conn
		e error
	}

	length := len(addrs)
	if d.Level < length {
		length = d.Level
	}

	addrs = pickupAddrs(addrs, length, d.TCPConnDuration)
	lane := make(chan racer, length)

	for _, addr := range addrs {
		go func(addr string, c chan<- racer) {
			start := time.Now()
			conn, err := d.Dialer.Dial(network, addr)
			end := time.Now()
			if err == nil {
				d.TCPConnDuration.Set(addr, end.Sub(start), end.Add(d.ConnExpiry))
			} else {
				d.TCPConnDuration.Del(addr)
			}
			lane <- racer{conn, err}
		}(addr, lane)
	}

	var r racer
	for i := 0; i < length; i++ {
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
			}(length - 1 - i)
			return r.c, nil
		}
	}
	return nil, r.e
}

func (d *MultiDialer) dialMultiTLS(network string, addrs []string, config *tls.Config) (net.Conn, error) {
	glog.V(3).Infof("dialMultiTLS(%v, %v, %#v)", network, addrs, config)
	type racer struct {
		c net.Conn
		e error
	}

	length := len(addrs)
	if d.Level < length {
		length = d.Level
	}

	addrs = pickupAddrs(addrs, length, d.TLSConnDuration)
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
				d.TLSConnDuration.Set(addr, end.Sub(start), end.Add(d.ConnExpiry))
			} else {
				d.TLSConnDuration.Del(addr)
			}

			lane <- racer{tlsConn, err}
		}(addr, lane)
	}

	var r racer
	for i := 0; i < length; i++ {
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
			}(length - 1 - i)
			return r.c, nil
		}
	}
	return nil, r.e
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
