package iplist

import (
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"strings"
	"time"

	"../../../httpproxy"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
	"github.com/miekg/dns"
)

type Iplist struct {
	lists      map[string][]string
	dnsservers []string
	blacklist  *httpproxy.HostMatcher
	dnsCache   lrucache.Cache
	dualStack  bool
}

func NewIplist(lists map[string][]string, dnsservers []string, blacklist []string, dualStack bool) (*Iplist, error) {
	iplist := &Iplist{
		lists:      lists,
		dnsservers: dnsservers,
		blacklist:  httpproxy.NewHostMatcher(blacklist),
		dnsCache:   lrucache.NewMultiLRUCache(4, 10240),
		dualStack:  dualStack,
	}

	return iplist, nil
}

func (i *Iplist) lookupHost(name string) (hosts []string, err error) {
	hs, err := net.LookupHost(name)
	if err != nil {
		return hs, err
	}

	hosts = make([]string, 0)
	for _, h := range hs {
		if !i.dualStack && strings.Contains(h, ":") {
			continue
		}
		if !i.blacklist.Match(h) {
			hosts = append(hosts, h)
		}
	}

	return hosts, nil
}

func (i *Iplist) lookupHost2(name string, dnsserver string) (hosts []string, err error) {
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

func (i *Iplist) Lookup(name string) (hosts []string, err error) {
	list, ok := i.lists[name]
	if !ok {
		return nil, fmt.Errorf("iplist %#v not exists", name)
	}

	hostSet := make(map[string]struct{}, 0)
	expire := time.Now().Add(24 * time.Hour)
	for _, addr := range list {
		var hs []string
		if hs0, ok := i.dnsCache.Get(addr); ok {
			hs = hs0.([]string)
		} else {
			hs, err = i.lookupHost(addr)
			if err != nil {
				glog.Warningf("lookupHost(%#v) error: %s", addr, err)
				continue
			}
			glog.V(2).Infof("Lookup(%#v) return %v", addr, hs)
			i.dnsCache.Set(addr, hs, expire)
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

func (i *Iplist) ExpandList(name string) error {
	list, ok := i.lists[name]
	if !ok {
		return fmt.Errorf("iplist %#v not exists", name)
	}

	expire := time.Now().Add(24 * time.Hour)
	for _, addr := range list {
		if regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).MatchString(addr) {
			continue
		}

		hostSet := make(map[string]struct{}, 0)
		for _, ds := range i.dnsservers {
			hs, err := i.lookupHost2(addr, ds)
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

		if hs, ok := i.dnsCache.Get(addr); ok {
			hs1 := hs.([]string)
			for _, h := range hs1 {
				hostSet[h] = struct{}{}
			}
		}

		hosts := make([]string, 0)
		for h, _ := range hostSet {
			hosts = append(hosts, h)
		}

		i.dnsCache.Set(addr, hosts, expire)
	}

	return nil
}

type Hosts struct {
	hosts       map[string]string
	suffixHosts map[string]string
}

func NewHosts(hosts map[string]string) (*Hosts, error) {
	h := &Hosts{
		hosts:       make(map[string]string),
		suffixHosts: make(map[string]string),
	}

	for k, v := range hosts {
		if strings.HasPrefix(k, ".") {
			h.suffixHosts[k] = v
		} else {
			h.hosts[k] = v
		}
	}

	return h, nil
}

func (h *Hosts) Lookup(name string) (alias string) {
	s, ok := h.hosts[name]
	if ok {
		return s
	}

	for k, v := range h.suffixHosts {
		if strings.HasSuffix(name, k) {
			return v
		}
	}

	return ""
}

type Dialer struct {
	net.Dialer
	TLSConfig *tls.Config
	Window    int
	Blacklist map[string]struct{}
	hosts     *Hosts
	iplist    *Iplist
}

func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	glog.V(2).Infof("Dail(%#v, %#v)...", network, address)
	switch network {
	case "tcp", "tcp4", "tcp6":
		if host, port, err := net.SplitHostPort(address); err == nil {
			if alias := d.hosts.Lookup(host); alias != "" {
				if hosts, err := d.iplist.Lookup(alias); err == nil {
					return d.dialMulti(network, hosts, port)
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
			if alias := d.hosts.Lookup(host); alias != "" {
				if ips, err := d.iplist.Lookup(alias); err == nil {
					config := &tls.Config{
						InsecureSkipVerify: true,
						ServerName:         address,
					}
					if strings.Contains(address, ".google") || strings.Contains(address, ".appspot.com") {
						config.ServerName = "www.bing.com"
						config.CipherSuites = []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA}
					}
					// glog.V(2).Infof("dialMultiTLS(%#v, %#v) with %#v for %#v", ips, port, config, address)
					return d.dialMultiTLS(network, ips, port, config)
				}
			}
		}
	default:
		break
	}
	return tls.DialWithDialer(&d.Dialer, network, address, d.TLSConfig)
}

func (d *Dialer) dialMulti(network string, hosts []string, port string) (net.Conn, error) {
	type racer struct {
		conn net.Conn
		err  error
	}

	shuffle(hosts)

	length := len(hosts)
	if d.Window < length {
		length = d.Window
	}

	lane := make(chan racer, length)

	for _, h := range hosts[:length] {
		go func(h string, c chan<- racer) {
			conn, err := d.Dialer.Dial(network, net.JoinHostPort(h, port))
			lane <- racer{conn, err}
		}(h, lane)
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

func (d *Dialer) dialMultiTLS(network string, hosts []string, port string, config *tls.Config) (net.Conn, error) {
	type racer struct {
		conn net.Conn
		err  error
	}

	shuffle(hosts)

	length := len(hosts)
	if d.Window < length {
		length = d.Window
	}

	lane := make(chan racer, length)

	for _, h := range hosts[:length] {
		go func(h string, c chan<- racer) {
			conn, err := d.Dialer.Dial(network, net.JoinHostPort(h, port))
			if err != nil {
				lane <- racer{conn, err}
				return
			}

			if config == nil {
				config = &tls.Config{
					InsecureSkipVerify: true,
				}
			}
			if config.ServerName == "" {
				c := *config
				c.ServerName = h
				config = &c
			}

			tlsConn := tls.Client(conn, config)
			err = tlsConn.Handshake()
			lane <- racer{tlsConn, err}
		}(h, lane)
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

func shuffle(ips []string) {
	for i := len(ips) - 1; i >= 0; i-- {
		j := rand.Intn(i + 1)
		ips[i], ips[j] = ips[j], ips[i]
	}
}
