package helpers

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/miekg/dns"
	"github.com/phuslu/glog"
)

type Resolver struct {
	LRUCache    lrucache.Cache
	BlackList   lrucache.Cache
	DNSServer   net.IP
	DNSExpiry   time.Duration
	DisableIPv6 bool
	ForceIPv6   bool
}

func (r *Resolver) LookupHost(name string) ([]string, error) {
	ips, err := r.LookupIP(name)
	if err != nil {
		return nil, err
	}

	addrs := make([]string, len(ips))
	for i, ip := range ips {
		addrs[i] = ip.String()
	}

	return addrs, nil
}

func (r *Resolver) LookupIP(name string) ([]net.IP, error) {
	if r.LRUCache != nil {
		if v, ok := r.LRUCache.Get(name); ok {
			switch v.(type) {
			case []net.IP:
				return v.([]net.IP), nil
			case string:
				return r.LookupIP(v.(string))
			default:
				return nil, fmt.Errorf("LookupIP: cannot convert %T(%+v) to []net.IP", v, v)
			}
		}
	}

	if ip := net.ParseIP(name); ip != nil {
		ips := []net.IP{ip}
		if r.LRUCache != nil {
			r.LRUCache.Set(name, ips, time.Time{})
		}
		return ips, nil
	}

	lookupIP := r.lookupIP1
	if r.DNSServer != nil {
		lookupIP = r.lookupIP2
	}

	ips, err := lookupIP(name)
	if err == nil {
		if r.BlackList != nil {
			ips1 := ips[:0]
			for _, ip := range ips {
				if _, ok := r.BlackList.GetQuiet(ip.String()); !ok {
					ips1 = append(ips1, ip)
				}
			}
			ips = ips1
		}

		if r.LRUCache != nil && len(ips) > 0 {
			if r.DNSExpiry == 0 {
				r.LRUCache.Set(name, ips, time.Time{})
			} else {
				r.LRUCache.Set(name, ips, time.Now().Add(r.DNSExpiry))
			}
		}
	}

	glog.V(2).Infof("LookupIP(%#v) return %+v, err=%+v", name, ips, err)
	return ips, err
}

func (r *Resolver) lookupIP1(name string) ([]net.IP, error) {
	ips, err := LookupIP(name)
	if err != nil {
		return nil, err
	}

	ips1 := ips[:0]
	for _, ip := range ips {
		if strings.Contains(ip.String(), ":") {
			if r.ForceIPv6 || !r.DisableIPv6 {
				ips1 = append(ips1, ip)
			}
		} else {
			if !r.ForceIPv6 {
				ips1 = append(ips1, ip)
			}
		}
	}

	return ips1, nil
}

func (r *Resolver) lookupIP2(name string) ([]net.IP, error) {
	m := &dns.Msg{}

	switch {
	case r.ForceIPv6:
		m.SetQuestion(dns.Fqdn(name), dns.TypeAAAA)
	case r.DisableIPv6:
		m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	default:
		m.SetQuestion(dns.Fqdn(name), dns.TypeANY)
	}

	reply, err := dns.Exchange(m, net.JoinHostPort(r.DNSServer.String(), "53"))
	if err != nil {
		return nil, err
	}

	if len(reply.Answer) < 1 {
		return nil, fmt.Errorf("no Answer from dns server %+v", r.DNSServer.String())
	}

	ips := make([]net.IP, 0, 4)

	for _, rr := range reply.Answer {
		var ip net.IP

		switch rr.(type) {
		case *dns.AAAA:
			ip = rr.(*dns.AAAA).AAAA
		case *dns.A:
			ip = rr.(*dns.A).A
		}

		ips = append(ips, ip)
	}

	return ips, nil
}
