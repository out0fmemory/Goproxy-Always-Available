package iplist

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
	"github.com/miekg/dns"

	"../../../httpproxy"
)

type HostMap struct {
	lists      map[string][]string
	dnsservers []string
	blacklist  *httpproxy.HostMatcher
	dnsCache   lrucache.Cache
	dualStack  bool
}

func NewHostMap(lists map[string][]string, dnsservers []string, blacklist []string, dualStack bool) *HostMap {
	return &HostMap{
		lists:      lists,
		dnsservers: dnsservers,
		blacklist:  httpproxy.NewHostMatcher(blacklist),
		dnsCache:   lrucache.NewMultiLRUCache(4, 10240),
		dualStack:  dualStack,
	}
}

func (hm *HostMap) lookupHost(name string) (hosts []string, err error) {
	hs, err := net.LookupHost(name)
	if err != nil {
		return hs, err
	}

	hosts = make([]string, 0)
	for _, h := range hs {
		if !hm.dualStack && strings.Contains(h, ":") {
			continue
		}
		if !hm.blacklist.Match(h) {
			hosts = append(hosts, h)
		}
	}

	return hosts, nil
}

func (hm *HostMap) lookupHost2(name string, dnsserver string) (hosts []string, err error) {
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

func (hm *HostMap) Lookup(name string) (hosts []string, err error) {
	list, ok := hm.lists[name]
	if !ok {
		return nil, fmt.Errorf("iplist %#v not exists", name)
	}

	hostSet := make(map[string]struct{}, 0)
	expire := time.Now().Add(24 * time.Hour)
	for _, addr := range list {
		var hs []string
		if hs0, ok := hm.dnsCache.Get(addr); ok {
			hs = hs0.([]string)
		} else {
			hs, err = hm.lookupHost(addr)
			if err != nil {
				glog.Warningf("lookupHost(%#v) error: %s", addr, err)
				continue
			}
			glog.V(2).Infof("Lookup(%#v) return %v", addr, hs)
			hm.dnsCache.Set(addr, hs, expire)
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

func (hm *HostMap) ExpandList(name string) error {
	list, ok := hm.lists[name]
	if !ok {
		return fmt.Errorf("iplist %#v not exists", name)
	}

	expire := time.Now().Add(24 * time.Hour)
	for _, addr := range list {
		if regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`).MatchString(addr) {
			continue
		}

		hostSet := make(map[string]struct{}, 0)
		for _, ds := range hm.dnsservers {
			hs, err := hm.lookupHost2(addr, ds)
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

		if hs, ok := hm.dnsCache.Get(addr); ok {
			hs1 := hs.([]string)
			for _, h := range hs1 {
				hostSet[h] = struct{}{}
			}
		}

		hosts := make([]string, 0)
		for h, _ := range hostSet {
			hosts = append(hosts, h)
		}

		hm.dnsCache.Set(addr, hosts, expire)
	}

	return nil
}
