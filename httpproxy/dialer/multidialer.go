package dialer

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/miekg/dns"
	"github.com/phuslu/glog"

	"../helpers"
)

type MultiDialer struct {
	Dialer interface {
		Dial(network, addr string) (net.Conn, error)
	}
	DisableIPv6       bool
	ForceIPv6         bool
	SSLVerify         bool
	EnableRemoteDNS   bool
	LogToStderr       bool
	TLSConfig         *tls.Config
	SiteToAlias       *helpers.HostMatcher
	GoogleTLSConfig   *tls.Config
	GoogleG2PKP       []byte
	IPBlackList       lrucache.Cache
	HostMap           map[string][]string
	DNSServers        []net.IP
	DNSCache          lrucache.Cache
	DNSCacheExpiry    time.Duration
	TLSConnDuration   lrucache.Cache
	TLSConnError      lrucache.Cache
	TLSConnReadBuffer int
	ConnExpiry        time.Duration
	Level             int
}

func (d *MultiDialer) ClearCache() {
	// d.DNSCache.Clear()
	d.TLSConnDuration.Clear()
	d.TLSConnError.Clear()
}

func (d *MultiDialer) lookupHost1(name string) (addrs []string, err error) {
	ips, err := helpers.LookupIP(name)
	if err != nil {
		return nil, err
	}

	addrs = make([]string, 0)
	for _, ip := range ips {
		h := ip.String()
		if _, ok := d.IPBlackList.GetQuiet(h); ok {
			continue
		}

		if strings.Contains(h, ":") {
			if d.ForceIPv6 || !d.DisableIPv6 {
				addrs = append(addrs, h)
			}
		} else {
			if !d.ForceIPv6 {
				addrs = append(addrs, h)
			}
		}
	}

	return addrs, nil
}

func (d *MultiDialer) lookupHost2(name string, dnsserver net.IP) (addrs []string, err error) {
	m := &dns.Msg{}

	switch {
	case d.ForceIPv6:
		m.SetQuestion(dns.Fqdn(name), dns.TypeAAAA)
	case d.DisableIPv6:
		m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	default:
		m.SetQuestion(dns.Fqdn(name), dns.TypeANY)
	}

	r, err := dns.Exchange(m, net.JoinHostPort(dnsserver.String(), "53"))
	if err != nil {
		return nil, err
	}

	if len(r.Answer) < 1 {
		return nil, errors.New("no Answer")
	}

	addrs = []string{}

	for _, rr := range r.Answer {
		var addr string

		if aaaa, ok := rr.(*dns.AAAA); ok {
			addr = aaaa.AAAA.String()
		}
		if a, ok := rr.(*dns.A); ok {
			addr = a.A.String()
		}

		if addr == "" {
			continue
		}

		if _, ok := d.IPBlackList.GetQuiet(addr); ok {
			continue
		}

		addrs = append(addrs, addr)
	}

	return addrs, nil
}

func (d *MultiDialer) LookupHost(name string) (addrs []string, err error) {
	if d.EnableRemoteDNS {
		return d.lookupHost2(name, d.DNSServers[0])
	} else {
		return d.lookupHost1(name)
	}
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
		if net.ParseIP(name) != nil {
			addrs0 = []string{name}
			expiry = time.Time{}
		} else if addrs1, ok := d.DNSCache.Get(name); ok {
			addrs0 = addrs1.([]string)
		} else {
			addrs0, err = d.LookupHost(name)
			if err != nil {
				glog.Warningf("LookupHost(%#v) error: %s", name, err)
				addrs0 = []string{}
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
		if _, ok := d.IPBlackList.GetQuiet(addr); ok {
			continue
		}
		addrs = append(addrs, addr)
	}

	if len(addrs) == 0 {
		glog.Errorf("MULTIDIALER: LookupAlias(%#v) have no good ip addrs", alias)
		return nil, fmt.Errorf("MULTIDIALER: LookupAlias(%#v) have no good ip addrs", alias)
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
		seen := make(map[string]struct{}, 0)
		for _, dnsserver := range d.DNSServers {
			var addrs []string
			var err error
			if net.ParseIP(name) != nil {
				addrs = []string{name}
				expire = time.Time{}
			} else if addrs, err = d.lookupHost2(name, dnsserver); err != nil {
				glog.V(2).Infof("lookupHost2(%#v) error: %s", name, err)
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
	return d.Dialer.Dial(network, address)
}

func (d *MultiDialer) DialTLS(network, address string) (net.Conn, error) {
	return d.DialTLS2(network, address, nil)
}

func (d *MultiDialer) DialTLS2(network, address string, cfg *tls.Config) (net.Conn, error) {
	if d.LogToStderr {
		helpers.SetConsoleTextColorGreen()
	}
	glog.V(2).Infof("MULTIDIALER DialTLS2(%#v, %#v) with good_addrs=%d, bad_addrs=%d", network, address, d.TLSConnDuration.Len(), d.TLSConnError.Len())
	if d.LogToStderr {
		helpers.SetConsoleTextColorReset()
	}

	if cfg == nil {
		cfg = d.TLSConfig
	}

	switch network {
	case "tcp", "tcp4", "tcp6":
		if host, port, err := net.SplitHostPort(address); err == nil {
			if alias0, ok := d.SiteToAlias.Lookup(host); ok {
				alias := alias0.(string)
				if hosts, err := d.LookupAlias(alias); err == nil {
					var config *tls.Config

					isGoogleAddr := false
					switch {
					case strings.HasPrefix(alias, "google_"):
						config = d.GoogleTLSConfig
						isGoogleAddr = true
					case cfg == nil:
						config = &tls.Config{
							InsecureSkipVerify: !d.SSLVerify,
							ServerName:         address,
						}
					default:
						config = cfg
					}
					glog.V(3).Infof("DialTLS2(%#v, %#v) alais=%#v set tls.Config=%#v", network, address, alias, config)

					addrs := make([]string, len(hosts))
					for i, host := range hosts {
						addrs[i] = net.JoinHostPort(host, port)
					}
					switch {
					case d.ForceIPv6:
						network = "tcp6"
					case d.DisableIPv6:
						network = "tcp4"
					}
					conn, err := d.dialMultiTLS(network, addrs, config)
					if err != nil {
						return nil, err
					}
					if d.SSLVerify && isGoogleAddr {
						if tc, ok := conn.(*tls.Conn); ok {
							certs := tc.ConnectionState().PeerCertificates
							if len(tc.ConnectionState().PeerCertificates) <= 1 {
								return nil, fmt.Errorf("Wrong certificate of %s: PeerCertificates=%#v", conn.RemoteAddr(), certs)
							}
							cert := certs[1]
							glog.V(3).Infof("MULTIDIALER DialTLS(%#v, %#v) verify cert=%v", network, address, cert.Subject)
							if d.GoogleG2PKP != nil {
								pkp := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
								if bytes.Compare(pkp[:], d.GoogleG2PKP) != 0 {
									defer conn.Close()
									return nil, fmt.Errorf("Wrong certificate of %s: Issuer=%v, SubjectKeyId=%#v", conn.RemoteAddr(), cert.Subject, cert.SubjectKeyId)
								}
							} else {
								if !strings.HasPrefix(cert.Subject.CommonName, "Google ") {
									defer conn.Close()
									return nil, fmt.Errorf("Wrong certificate of %s: Issuer=%v, SubjectKeyId=%#v", conn.RemoteAddr(), cert.Subject, cert.SubjectKeyId)
								}
							}
						}
					}
					return conn, nil
				}
			}
		}
	default:
		break
	}

	if dialer, ok := d.Dialer.(*net.Dialer); ok {
		return tls.DialWithDialer(dialer, network, address, d.TLSConfig)
	} else {
		return tls.Dial(network, address, d.TLSConfig)
	}
}

func (d *MultiDialer) dialMultiTLS(network string, addrs []string, config *tls.Config) (net.Conn, error) {
	glog.V(3).Infof("dialMultiTLS(%v, %v, %#v)", network, addrs, config)
	type connWithError struct {
		c net.Conn
		e error
	}

	addrs = d.pickupTLSAddrs(addrs, d.Level)
	lane := make(chan connWithError, len(addrs))

	for _, addr := range addrs {
		go func(addr string, c chan<- connWithError) {
			// start := time.Now()
			conn, err := d.Dialer.Dial(network, addr)
			if err != nil {
				d.TLSConnDuration.Del(addr)
				d.TLSConnError.Set(addr, err, time.Now().Add(d.ConnExpiry))
				lane <- connWithError{conn, err}
				return
			}

			if d.TLSConnReadBuffer > 0 {
				if tc, ok := conn.(*net.TCPConn); ok {
					tc.SetReadBuffer(d.TLSConnReadBuffer)
				}
			}

			if config == nil {
				config = &tls.Config{
					InsecureSkipVerify: true,
				}
			}

			start := time.Now()
			tlsConn := tls.Client(conn, config)
			err = tlsConn.Handshake()

			end := time.Now()
			if err != nil {
				d.TLSConnDuration.Del(addr)
				d.TLSConnError.Set(addr, err, end.Add(d.ConnExpiry))
			} else {
				d.TLSConnDuration.Set(addr, end.Sub(start), end.Add(d.ConnExpiry))
			}

			lane <- connWithError{tlsConn, err}
		}(addr, lane)
	}

	var r connWithError
	for i := range addrs {
		r = <-lane
		if r.e == nil {
			go func(count int) {
				var r1 connWithError
				for ; count > 0; count-- {
					r1 = <-lane
					if r1.c != nil {
						r1.c.Close()
					}
				}
			}(len(addrs) - 1 - i)
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

func (r racers) Len() int           { return len(r) }
func (r racers) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r racers) Less(i, j int) bool { return r[i].duration < r[j].duration }

func (d *MultiDialer) pickupTLSAddrs(addrs []string, n int) []string {
	if len(addrs) <= n {
		return addrs
	}

	goodAddrs := make([]racer, 0)
	unknownAddrs := make([]string, 0)
	badAddrs := make([]string, 0)

	for _, addr := range addrs {
		if duration, ok := d.TLSConnDuration.GetQuiet(addr); ok {
			if d, ok := duration.(time.Duration); !ok {
				glog.Errorf("%#v for %#v is not a time.Duration", duration, addr)
			} else {
				goodAddrs = append(goodAddrs, racer{addr, d})
			}
		} else if e, ok := d.TLSConnError.GetQuiet(addr); ok {
			if _, ok := e.(error); !ok {
				glog.Errorf("%#v for %#v is not a error", e, addr)
			} else {
				badAddrs = append(badAddrs, addr)
			}
		} else {
			unknownAddrs = append(unknownAddrs, addr)
		}
	}

	addrs1 := make([]string, 0, n)

	sort.Sort(racers(goodAddrs))
	if len(goodAddrs) > n/2 {
		goodAddrs = goodAddrs[:n/2]
	}
	for _, r := range goodAddrs {
		addrs1 = append(addrs1, r.addr)
	}

	for _, addrs2 := range [][]string{unknownAddrs, badAddrs} {
		if len(addrs1) < n && len(addrs2) > 0 {
			m := n - len(addrs1)
			if len(addrs2) > m {
				helpers.ShuffleStringsN(addrs2, m)
				addrs2 = addrs2[:m]
			}
			addrs1 = append(addrs1, addrs2...)
		}
	}

	return addrs1
}
