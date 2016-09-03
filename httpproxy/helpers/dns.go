package helpers

import (
	"fmt"
	"net"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/miekg/dns"
)

type Resolver struct {
	LRUCache    lrucache.Cache
	DNSServer   net.IP
	DNSExpiry   time.Duration
	DisableIPv6 bool
	ForceIPv6   bool
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

	f := net.LookupIP
	if r.DNSServer != nil {
		f = r.lookupIP
	}

	ips, err := f(name)
	if err == nil {
		if r.DNSExpiry == 0 {
			r.LRUCache.Set(name, ips, time.Time{})
		} else {
			r.LRUCache.Set(name, ips, time.Now().Add(r.DNSExpiry))
		}
	}

	return ips, err
}

func (r *Resolver) lookupIP(name string) ([]net.IP, error) {
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
