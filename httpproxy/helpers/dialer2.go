package helpers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	quic "github.com/phuslu/quic-go"
)

type MultiDialer struct {
	KeepAlive         time.Duration
	Timeout           time.Duration
	DualStack         bool
	Resolver          *Resolver
	SSLVerify         bool
	LogToStderr       bool
	TLSConfig         *tls.Config
	SiteToAlias       *HostMatcher
	GoogleTLSConfig   *tls.Config
	GoogleG2PKP       []byte
	IPBlackList       lrucache.Cache
	HostMap           map[string][]string
	TLSConnDuration   lrucache.Cache
	TLSConnError      lrucache.Cache
	TLSConnReadBuffer int
	GoodConnExpiry    time.Duration
	ErrorConnExpiry   time.Duration
	Level             int
}

func (d *MultiDialer) ClearCache() {
	// d.DNSCache.Clear()
	d.TLSConnDuration.Clear()
	d.TLSConnError.Clear()
}

func (d *MultiDialer) LookupAlias(alias string) (hosts []string, err error) {
	names, ok := d.HostMap[alias]
	if !ok {
		return nil, fmt.Errorf("alias %#v not exists", alias)
	}

	seen := make(map[string]struct{}, 0)
	for _, name := range names {
		var hosts0 []string
		if net.ParseIP(name) != nil {
			hosts0 = []string{name}
		} else {
			hosts0, err = d.Resolver.LookupHost(name)
			if err != nil {
				glog.Warningf("LookupHost(%#v) error: %s", name, err)
				hosts0 = []string{}
			}
		}
		for _, host := range hosts0 {
			seen[host] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil, err
	}

	hosts = make([]string, 0)
	for host := range seen {
		if _, ok := d.IPBlackList.GetQuiet(host); ok {
			continue
		}
		hosts = append(hosts, host)
	}

	if len(hosts) == 0 {
		glog.Errorf("MULTIDIALER: LookupAlias(%#v) have no good ips", alias)
		return nil, fmt.Errorf("MULTIDIALER: LookupAlias(%#v) have no good ips", alias)
	}

	return hosts, nil
}

func (d *MultiDialer) DialTLS(network, address string) (net.Conn, error) {
	return d.DialTLS2(network, address, nil)
}

func (d *MultiDialer) DialTLS2(network, address string, cfg *tls.Config) (net.Conn, error) {
	if d.LogToStderr {
		SetConsoleTextColorGreen()
	}
	glog.V(2).Infof("MULTIDIALER DialTLS2(%#v, %#v) with good_addrs=%d, bad_addrs=%d", network, address, d.TLSConnDuration.Len(), d.TLSConnError.Len())
	if d.LogToStderr {
		SetConsoleTextColorReset()
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

					switch {
					case d.Resolver.ForceIPv6:
						network = "tcp6"
					case d.Resolver.DisableIPv6:
						network = "tcp4"
					}
					conn, err := d.dialMultiTLS(network, hosts, port, config)
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
							glog.V(3).Infof("MULTIDIALER DialTLS2(%#v, %#v) verify cert=%v", network, address, cert.Subject)
							switch {
							case d.GoogleG2PKP != nil:
								pkp := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
								if bytes.Equal(pkp[:], d.GoogleG2PKP) {
									break
								}
								fallthrough
							case !strings.HasPrefix(cert.Subject.CommonName, "Google "):
								err := fmt.Errorf("Wrong certificate of %s: Issuer=%v, SubjectKeyId=%#v", conn.RemoteAddr(), cert.Subject, cert.SubjectKeyId)
								glog.Warningf("MultiDailer: %v", err)
								if ip, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil {
									d.IPBlackList.Set(ip, struct{}{}, time.Time{})
								}
								conn.Close()
								return nil, err
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

	dialer := &net.Dialer{
		KeepAlive: d.KeepAlive,
		Timeout:   d.Timeout,
		DualStack: d.DualStack,
	}
	return tls.DialWithDialer(dialer, network, address, d.TLSConfig)
}

func (d *MultiDialer) dialMultiTLS(network string, hosts []string, port string, config *tls.Config) (net.Conn, error) {
	glog.V(3).Infof("dialMultiTLS(%v, %v, %#v)", network, hosts, config)
	type connWithError struct {
		c net.Conn
		e error
	}

	hosts = d.pickupTLSHosts(hosts, d.Level)
	lane := make(chan connWithError, len(hosts))

	for _, host := range hosts {
		go func(host string, c chan<- connWithError) {
			// start := time.Now()
			raddr, err := net.ResolveTCPAddr(network, net.JoinHostPort(host, port))
			if err != nil {
				glog.Warningf("net.ResolveTCPAddr(%#v, %+v) err=%+v", network, host, err)
				lane <- connWithError{nil, err}
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), d.Timeout)
			defer cancel()

			conn, err := net.DialTCPContext(ctx, network, nil, raddr)
			if err != nil {
				d.TLSConnDuration.Del(host)
				d.TLSConnError.Set(host, err, time.Now().Add(d.ErrorConnExpiry))
				lane <- connWithError{nil, err}
				return
			}

			if d.KeepAlive > 0 {
				conn.SetKeepAlive(true)
				conn.SetKeepAlivePeriod(d.KeepAlive)
			}

			if d.TLSConnReadBuffer > 0 {
				conn.SetReadBuffer(d.TLSConnReadBuffer)
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
				d.TLSConnDuration.Del(host)
				d.TLSConnError.Set(host, err, end.Add(d.ErrorConnExpiry))
			} else {
				d.TLSConnDuration.Set(host, end.Sub(start), end.Add(d.GoodConnExpiry))
			}

			lane <- connWithError{tlsConn, err}
		}(host, lane)
	}

	var r connWithError
	for i := range hosts {
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
			}(len(hosts) - 1 - i)
			return r.c, nil
		}
	}
	return nil, r.e
}

func (d *MultiDialer) DialQuic(address string, tlsConfig *tls.Config, cfg *quic.Config) (quic.Session, error) {
	if d.LogToStderr {
		SetConsoleTextColorGreen()
	}
	glog.V(2).Infof("MULTIDIALER DialQuic(%#v) with good_addrs=%d, bad_addrs=%d", address, d.TLSConnDuration.Len(), d.TLSConnError.Len())
	if d.LogToStderr {
		SetConsoleTextColorReset()
	}

	if tlsConfig == nil {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: !d.SSLVerify,
			ServerName:         address,
		}
	}

	if cfg == nil {
		cfg = &quic.Config{
			HandshakeTimeout:              d.Timeout,
			IdleTimeout:                   d.Timeout,
			RequestConnectionIDTruncation: true,
			KeepAlive:                     true,
		}
	}

	if host, port, err := net.SplitHostPort(address); err == nil {
		if alias0, ok := d.SiteToAlias.Lookup(host); ok {
			alias := alias0.(string)
			if hosts, err := d.LookupAlias(alias); err == nil {
				var config *quic.Config

				isGoogleAddr := false
				switch {
				case strings.HasPrefix(alias, "google_"):
					config = &quic.Config{
						HandshakeTimeout:              d.Timeout,
						IdleTimeout:                   d.Timeout,
						RequestConnectionIDTruncation: true,
						KeepAlive:                     true,
					}
					isGoogleAddr = true
				case cfg == nil:
					config = &quic.Config{
						HandshakeTimeout:              d.Timeout,
						IdleTimeout:                   d.Timeout,
						RequestConnectionIDTruncation: true,
						KeepAlive:                     true,
					}
				default:
					config = cfg
				}
				glog.V(3).Infof("DialQuic(%#v) alais=%#v set quic.Config=%#v", address, alias, config)

				sess, err := d.dialMultiQuic(hosts, port, tlsConfig, config)
				if err != nil {
					return nil, err
				}
				if d.SSLVerify && isGoogleAddr {
					// TODO: verify google certificates
				}
				return sess, nil
			}
		}
	}

	return quic.DialAddr(address, tlsConfig, cfg)
}

func (d *MultiDialer) dialMultiQuic(hosts []string, port string, tlsConfig *tls.Config, config *quic.Config) (quic.Session, error) {
	glog.V(3).Infof("dialMultiQuic( %v, %#v)", hosts, config)
	type sessWithError struct {
		s quic.Session
		e error
	}

	hosts = d.pickupTLSHosts(hosts, d.Level)
	lane := make(chan sessWithError, len(hosts))

	for _, host := range hosts {
		go func(host string, c chan<- sessWithError) {
			addr := net.JoinHostPort(host, port)

			start := time.Now()
			sess, err := quic.DialAddr(addr, tlsConfig, config)
			end := time.Now()

			if err != nil {
				d.TLSConnDuration.Del(host)
				d.TLSConnError.Set(host, err, end.Add(d.ErrorConnExpiry))
			} else {
				d.TLSConnDuration.Set(host, end.Sub(start), end.Add(d.GoodConnExpiry))
			}

			lane <- sessWithError{sess, err}
		}(host, lane)
	}

	var r sessWithError
	for i := range hosts {
		r = <-lane
		if r.e == nil {
			go func(count int) {
				var r1 sessWithError
				for ; count > 0; count-- {
					r1 = <-lane
					if r1.s != nil {
						r1.s.Close(nil)
					}
				}
			}(len(hosts) - 1 - i)
			return r.s, nil
		}
	}
	return nil, r.e
}

func (d *MultiDialer) pickupTLSHosts(hosts []string, n int) []string {
	if len(hosts) <= n {
		return hosts
	}

	type racer struct {
		host     string
		duration time.Duration
	}

	goods := make([]racer, 0)
	unknowns := make([]string, 0)
	bads := make([]string, 0)

	for _, host := range hosts {
		if duration, ok := d.TLSConnDuration.GetNotStale(host); ok {
			if d, ok := duration.(time.Duration); !ok {
				glog.Errorf("%#v for %#v is not a time.Duration", duration, host)
			} else {
				goods = append(goods, racer{host, d})
			}
		} else if e, ok := d.TLSConnError.GetNotStale(host); ok {
			if _, ok := e.(error); !ok {
				glog.Errorf("%#v for %#v is not a error", e, host)
			} else {
				bads = append(bads, host)
			}
		} else {
			unknowns = append(unknowns, host)
		}
	}

	sort.Slice(goods, func(i, j int) bool { return goods[i].duration < goods[j].duration })

	m := n / 2
	if len(bads) > 16*len(goods) {
		n += m
	}
	if len(goods) > m {
		goods = goods[:m]
	}

	hosts1 := make([]string, 0, n)
	for _, r := range goods {
		hosts1 = append(hosts1, r.host)
	}

	if len(goods) == 0 {
		ShuffleStrings(unknowns)
		ShuffleStrings(bads)
	}

	for _, hosts2 := range [][]string{unknowns, bads} {
		if len(hosts1) < n && len(hosts2) > 0 {
			m := n - len(hosts1)
			if len(hosts2) > m {
				ShuffleStringsN(hosts2, m)
				hosts2 = hosts2[:m]
			}
			hosts1 = append(hosts1, hosts2...)
		}
	}

	return hosts1
}

type MultiResolver struct {
	*MultiDialer
}

func (r *MultiResolver) LookupHost(host string) ([]string, error) {
	if alias0, ok := r.MultiDialer.SiteToAlias.Lookup(host); ok {
		alias := alias0.(string)
		if hosts, err := r.MultiDialer.LookupAlias(alias); err == nil && len(hosts) > 0 {
			ShuffleStrings(hosts)
			return hosts, nil
		}
	}
	return []string{host}, nil
}
