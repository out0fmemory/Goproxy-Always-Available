package iplist

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"../../../httpproxy"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
	"github.com/miekg/dns"
)

type Iplist struct {
	lists     map[string][]string
	blacklist *httpproxy.HostMatcher
	dnsCache  lrucache.Cache
	dualStack bool
}

func NewIplist(lists map[string][]string, blacklist []string, dualStack bool) (*Iplist, error) {
	iplist := &Iplist{
		lists:     lists,
		blacklist: httpproxy.NewHostMatcher(blacklist),
		dnsCache:  lrucache.NewMultiLRUCache(4, 10240),
		dualStack: dualStack,
	}

	return iplist, nil
}

func (i *Iplist) lookupOne(name string) (hosts []string, err error) {
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

func (i *Iplist) lookupOneRemote(name string, dnsserver string) (hosts []string, err error) {
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion(name+".", dns.TypeA)

	r, _, err := c.Exchange(m, net.JoinHostPort(dnsserver, "53"))
	if err != nil {
		return nil, err
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("lookupOneRemote(%#v, %#v) return %#v", name, dnsserver, r.Rcode)
	}

	hosts = make([]string, 0)
	for _, a := range r.Answer {
		h := a.String()
		if !i.blacklist.Match(h) {
			hosts = append(hosts, h)
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
			hs, err = i.lookupOne(addr)
			// hs, err = i.lookupOneRemote(addr, "114.114.114.114")
			if err != nil {
				glog.Warningf("net.ResolveIPAddr(\"tcp\", %#v) error: %s", addr, err)
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
					return d.DialMulti(network, hosts, port)
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
					return d.DialMultiTLS(network, ips, port)
				}
			}
		}
	default:
		break
	}
	return tls.DialWithDialer(&d.Dialer, network, address, d.TLSConfig)
}

func (d *Dialer) DialMulti(network string, hosts []string, port string) (net.Conn, error) {
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

	for _, h := range hosts {
		go func(h string, c chan<- racer) {
			addr := net.JoinHostPort(h, port)
			conn, err := d.Dialer.Dial(network, addr)
			c <- racer{conn, err}
		}(h, lane)
	}

	var r racer
	for i := 0; i < length; i++ {
		r = <-lane
		if r.err == nil {
			go func(count int) {
				var r1 racer
				for i := 0; i < count; i++ {
					r1 = <-lane
					if r1.err == nil {
						r1.conn.Close()
					}
				}
			}(length - i - 1)
			return r.conn, nil
		}
	}
	return nil, r.err
}

func (d *Dialer) DialMultiTLS(network string, hosts []string, port string) (net.Conn, error) {
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

	for _, h := range hosts {
		go func(h string, c chan<- racer) {
			config := d.TLSConfig
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
			addr := net.JoinHostPort(h, port)
			conn, err := tls.DialWithDialer(&d.Dialer, network, addr, config)
			lane <- racer{conn, err}
		}(h, lane)
	}

	var r racer
	for i := 0; i < length; i++ {
		r = <-lane
		if r.err == nil {
			go func(count int) {
				var r1 racer
				for i := 0; i < count; i++ {
					r1 = <-lane
					if r1.err == nil {
						r1.conn.Close()
					}
				}
			}(length - i - 1)
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
