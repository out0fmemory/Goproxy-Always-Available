package gae

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/net/http2"
	quic "github.com/phuslu/quic-go"
	"github.com/phuslu/quic-go/h2quic"

	"../../filters"
	"../../helpers"
	"../../proxy"
	"../../storage"
)

const (
	filterName string = "gae"
)

type Config struct {
	AppIDs          []string
	Password        string
	SSLVerify       bool
	DisableIPv6     bool
	ForceIPv6       bool
	DisableHTTP2    bool
	ForceHTTP2      bool
	EnableQuic      bool
	EnableDeadProbe bool
	EnableRemoteDNS bool
	SiteToAlias     map[string]string
	Site2Alias      map[string]string
	HostMap         map[string][]string
	TLSConfig       struct {
		Version                string
		SSLVerify              bool
		ClientSessionCacheSize int
		Ciphers                []string
		ServerName             []string
	}
	GoogleG2PKP string
	ForceGAE    []string
	FakeOptions map[string][]string
	DNSServers  []string
	IPBlackList []string
	Transport   struct {
		Dialer struct {
			DNSCacheExpiry   int
			DNSCacheSize     uint
			SocketReadBuffer int
			DualStack        bool
			KeepAlive        int
			Level            int
			Timeout          int
		}
		Proxy struct {
			Enabled bool
			URL     string
		}
		DisableCompression    bool
		DisableKeepAlives     bool
		IdleConnTimeout       int
		MaxIdleConnsPerHost   int
		ResponseHeaderTimeout int
		RetryDelay            float32
		RetryTimes            int
	}
}

type Filter struct {
	Config
	GAETransport       *GAETransport
	Transport          *Transport
	ForceHTTPSMatcher  *helpers.HostMatcher
	ForceGAEStrings    []string
	ForceGAESuffixs    []string
	ForceGAEMatcher    *helpers.HostMatcher
	FakeOptionsMatcher *helpers.HostMatcher
	SiteMatcher        *helpers.HostMatcher
	DirectSiteMatcher  *helpers.HostMatcher
}

func init() {
	filters.Register(filterName, func() (filters.Filter, error) {
		filename := filterName + ".json"
		config := new(Config)
		err := storage.LookupStoreByFilterName(filterName).UnmarshallJson(filename, config)
		if err != nil {
			glog.Fatalf("storage.ReadJsonConfig(%#v) failed: %s", filename, err)
		}
		return NewFilter(config)
	})
}

func NewFilter(config *Config) (filters.Filter, error) {
	dnsServers := make([]net.IP, 0)
	for _, s := range config.DNSServers {
		if ip := net.ParseIP(s); ip != nil {
			dnsServers = append(dnsServers, ip)
		}
	}

	GoogleG2PKP, err := base64.StdEncoding.DecodeString(config.GoogleG2PKP)
	if err != nil {
		return nil, err
	}

	googleTLSConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		ServerName:         "www.microsoft.com",
		ClientSessionCache: tls.NewLRUClientSessionCache(config.TLSConfig.ClientSessionCacheSize),
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
	}
	switch config.TLSConfig.Version {
	case "TLSv13", "TLSv1.3":
		googleTLSConfig.MinVersion = tls.VersionTLS13
	default:
		googleTLSConfig.MinVersion = tls.VersionTLS12
	}
	pickupCiphers := func(names []string) []uint16 {
		ciphers := make([]uint16, 0)
		for _, name := range names {
			cipher := helpers.Cipher(name)
			if cipher == 0 {
				glog.Fatalf("GAE: cipher %#v is not supported.", name)
			}
			ciphers = append(ciphers, cipher)
		}
		helpers.ShuffleUint16s(ciphers)
		return ciphers
	}
	googleTLSConfig.CipherSuites = pickupCiphers(config.TLSConfig.Ciphers)
	if len(config.TLSConfig.ServerName) > 0 {
		googleTLSConfig.ServerName = config.TLSConfig.ServerName[rand.Intn(len(config.TLSConfig.ServerName))]
	}
	if !config.DisableHTTP2 {
		googleTLSConfig.NextProtos = []string{"h2", "http/1.1"}
	}

	if config.Site2Alias == nil {
		config.Site2Alias = make(map[string]string)
	}
	for key, value := range config.SiteToAlias {
		config.Site2Alias[key] = value
	}
	config.SiteToAlias = config.Site2Alias

	hostmap := map[string][]string{}
	for key, value := range config.HostMap {
		hosts := helpers.UniqueStrings(value)
		helpers.ShuffleStrings(hosts)
		hostmap[key] = hosts
	}

	r := &helpers.Resolver{
		LRUCache:    lrucache.NewLRUCache(config.Transport.Dialer.DNSCacheSize),
		DNSExpiry:   time.Duration(config.Transport.Dialer.DNSCacheExpiry) * time.Second,
		DisableIPv6: config.DisableIPv6,
		ForceIPv6:   config.ForceIPv6,
	}

	if config.EnableRemoteDNS {
		r.DNSServer = net.ParseIP(config.DNSServers[0])
		if r.DNSServer == nil {
			glog.Fatalf("net.ParseIP(%+v) failed: %s", config.DNSServers[0], err)
		}
	}

	md := &helpers.MultiDialer{
		KeepAlive:         time.Duration(config.Transport.Dialer.KeepAlive) * time.Second,
		Timeout:           time.Duration(config.Transport.Dialer.Timeout) * time.Second,
		DualStack:         config.Transport.Dialer.DualStack,
		Resolver:          r,
		SSLVerify:         config.SSLVerify,
		LogToStderr:       flag.Lookup("logtostderr") != nil,
		TLSConfig:         nil,
		SiteToAlias:       helpers.NewHostMatcherWithString(config.SiteToAlias),
		IPBlackList:       lrucache.NewLRUCache(1024),
		HostMap:           hostmap,
		GoogleTLSConfig:   googleTLSConfig,
		GoogleG2PKP:       GoogleG2PKP,
		TLSConnDuration:   lrucache.NewLRUCache(8192),
		TLSConnError:      lrucache.NewLRUCache(8192),
		TLSConnReadBuffer: config.Transport.Dialer.SocketReadBuffer,
		GoodConnExpiry:    5 * time.Minute,
		ErrorConnExpiry:   30 * time.Minute,
		Level:             config.Transport.Dialer.Level,
	}

	for _, ip := range config.IPBlackList {
		md.IPBlackList.Set(ip, struct{}{}, time.Time{})
	}

	GetHostnameCacheKey := func(addr string) string {
		var host, port string
		var err error
		host, port, err = net.SplitHostPort(addr)
		if err != nil {
			host = addr
			port = "443"
		}

		if alias, ok := md.SiteToAlias.Lookup(host); ok {
			addr = net.JoinHostPort(alias.(string), port)
		}

		return addr
	}

	tr := &Transport{
		MultiDialer: md,
		RetryTimes:  2,
	}

	t1 := &http.Transport{
		DialTLS:               md.DialTLS,
		DisableKeepAlives:     config.Transport.DisableKeepAlives,
		DisableCompression:    config.Transport.DisableCompression,
		ResponseHeaderTimeout: time.Duration(config.Transport.ResponseHeaderTimeout) * time.Second,
		IdleConnTimeout:       time.Duration(config.Transport.IdleConnTimeout) * time.Second,
		MaxIdleConnsPerHost:   config.Transport.MaxIdleConnsPerHost,
		GetConnectMethodAddr:  GetHostnameCacheKey,
	}

	if config.Transport.Proxy.Enabled {
		if config.EnableQuic {
			glog.Fatalf("EnableQuic is conflict with Proxy setting!")
		}
		fixedURL, err := url.Parse(config.Transport.Proxy.URL)
		if err != nil {
			glog.Fatalf("url.Parse(%#v) error: %s", config.Transport.Proxy.URL, err)
		}

		dialer0 := &net.Dialer{
			KeepAlive: md.KeepAlive,
			Timeout:   md.Timeout,
			DualStack: md.DualStack,
		}

		dialer, err := proxy.FromURL(fixedURL, dialer0, &helpers.MultiResolver{md})
		if err != nil {
			glog.Fatalf("proxy.FromURL(%#v) error: %s", fixedURL.String(), err)
		}

		t1.Dial = dialer.Dial
		t1.DialTLS = nil
		t1.Proxy = nil
		t1.TLSClientConfig = md.GoogleTLSConfig
	}

	switch {
	case config.EnableQuic:
		tr.RoundTripper = &h2quic.RoundTripper{
			DisableCompression: true,
			TLSClientConfig:    md.GoogleTLSConfig,
			QuicConfig: &quic.Config{
				HandshakeTimeout:              md.Timeout,
				IdleTimeout:                   md.Timeout,
				RequestConnectionIDTruncation: true,
				KeepAlive:                     true,
			},
			DialAddr:     md.DialQuic,
			GetClientKey: GetHostnameCacheKey,
		}
	case config.DisableHTTP2 && config.ForceHTTP2:
		glog.Fatalf("GAE: DisableHTTP2=%v and ForceHTTP2=%v is conflict!", config.DisableHTTP2, config.ForceHTTP2)
	case config.Transport.Proxy.Enabled && config.ForceHTTP2:
		glog.Fatalf("GAE: Proxy.Enabled=%v and ForceHTTP2=%v is conflict!", config.Transport.Proxy.Enabled, config.ForceHTTP2)
	case config.ForceHTTP2:
		tr.RoundTripper = &http2.Transport{
			DialTLS:            md.DialTLS2,
			TLSClientConfig:    md.GoogleTLSConfig,
			DisableCompression: config.Transport.DisableCompression,
		}
	case !config.DisableHTTP2:
		err := http2.ConfigureTransport(t1)
		if err != nil {
			glog.Warningf("GAE: Error enabling Transport HTTP/2 support: %v", err)
		}
		tr.RoundTripper = t1
	default:
		tr.RoundTripper = t1
	}

	forceHTTPSMatcherStrings := make([]string, 0)
	for key, value := range config.SiteToAlias {
		if strings.HasPrefix(value, "google_") {
			forceHTTPSMatcherStrings = append(forceHTTPSMatcherStrings, key)
		}
	}

	forceGAEStrings := make([]string, 0)
	forceGAESuffixs := make([]string, 0)
	forceGAEMatcherStrings := make([]string, 0)
	for _, s := range config.ForceGAE {
		if strings.Contains(s, "/") {
			if strings.HasSuffix(s, "$") {
				forceGAESuffixs = append(forceGAESuffixs, strings.TrimRight(s, "$"))
			} else {
				forceGAEStrings = append(forceGAEStrings, s)
			}
		} else {
			forceGAEMatcherStrings = append(forceGAEMatcherStrings, s)
		}
	}

	if config.EnableDeadProbe && !config.Transport.Proxy.Enabled {
		isNetAvailable := func() bool {
			c, err := net.DialTimeout("tcp", net.JoinHostPort(config.DNSServers[0], "53"), 300*time.Millisecond)
			if err != nil {
				glog.V(3).Infof("GAE EnableDeadProbe connect DNSServer(%#v) failed: %+v", config.DNSServers[0], err)
				return false
			}
			c.Close()
			return true
		}

		probeTLS := func() {
			if !isNetAvailable() {
				return
			}

			req, _ := http.NewRequest(http.MethodGet, "https://clients3.google.com/generate_204", nil)
			ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
			defer cancel()
			req = req.WithContext(ctx)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				glog.V(2).Infof("GAE EnableDeadProbe \"%s %s\" error: %v", req.Method, req.URL.String(), err)
				s := strings.ToLower(err.Error())
				if strings.HasPrefix(s, "net/http: request canceled") || strings.Contains(s, "timeout") {
					helpers.CloseConnections(tr.RoundTripper)
				}
			}
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
		}

		probeQuic := func() {
			if !isNetAvailable() {
				return
			}

			c := make(chan error)

			go func(c chan<- error) {
				req, _ := http.NewRequest(http.MethodGet, "https://clients3.google.com/generate_204", nil)
				resp, err := tr.RoundTrip(req)
				c <- err
				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}
			}(c)

			select {
			case err := <-c:
				if ne, ok := err.(*net.OpError); ok && err != nil {
					glog.V(2).Infof("GAE EnableDeadProbe probeQuic error: %v", ne)
					helpers.CloseConnections(tr.RoundTripper)
				}
			case <-time.After(3 * time.Second):
				glog.V(2).Infof("GAE EnableDeadProbe probeQuic timed out. Close all quic connections")
				helpers.CloseConnections(tr.RoundTripper)
			}

		}

		go func() {
			time.Sleep(1 * time.Minute)
			for {
				time.Sleep(time.Duration(2+rand.Intn(4)) * time.Second)
				if config.EnableQuic {
					probeQuic()
				} else {
					probeTLS()
				}
			}
		}()
	}

	helpers.ShuffleStrings(config.AppIDs)

	f := &Filter{
		Config: *config,
		GAETransport: &GAETransport{
			RoundTripper: tr,
			MultiDialer:  md,
			Servers:      NewServers(config.AppIDs, config.Password, config.SSLVerify),
			Deadline:     time.Duration(config.Transport.ResponseHeaderTimeout-2) * time.Second,
			RetryDelay:   time.Duration(config.Transport.RetryDelay*1000) * time.Millisecond,
			RetryTimes:   config.Transport.RetryTimes,
		},
		Transport:          tr,
		ForceHTTPSMatcher:  helpers.NewHostMatcher(forceHTTPSMatcherStrings),
		ForceGAEMatcher:    helpers.NewHostMatcher(forceGAEMatcherStrings),
		ForceGAEStrings:    forceGAEStrings,
		ForceGAESuffixs:    forceGAESuffixs,
		FakeOptionsMatcher: helpers.NewHostMatcherWithStrings(config.FakeOptions),
		DirectSiteMatcher:  helpers.NewHostMatcherWithString(config.Site2Alias),
	}

	if config.Transport.Proxy.Enabled {
		f.GAETransport.MultiDialer = nil
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	var tr http.RoundTripper = f.GAETransport

	if req.URL.Scheme == "http" && f.ForceHTTPSMatcher.Match(req.Host) && req.URL.Path != "/ocsp" {
		if !strings.HasPrefix(req.Header.Get("Referer"), "https://") {
			u := strings.Replace(req.URL.String(), "http://", "https://", 1)
			glog.V(2).Infof("GAE FORCEHTTPS get raw url=%v, redirect to %v", req.URL.String(), u)
			resp := &http.Response{
				StatusCode: http.StatusMovedPermanently,
				Header: http.Header{
					"Location": []string{u},
				},
				Request:       req,
				Close:         true,
				ContentLength: -1,
			}
			glog.V(2).Infof("%s \"GAE FORCEHTTPS %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
			return ctx, resp, nil
		}
	}

	if f.DirectSiteMatcher.Match(req.Host) {
		switch req.URL.Path {
		case "/url":
			if rawurl := req.URL.Query().Get("url"); rawurl != "" {
				if u, err := url.Parse(rawurl); err == nil {
					if u.Scheme == "http" && f.ForceHTTPSMatcher.Match(u.Host) {
						rawurl = strings.Replace(rawurl, "http://", "https://", 1)
					}
				}
				glog.V(2).Infof("%s \"GAE REDIRECT %s %s %s\" - -", req.RemoteAddr, req.Method, rawurl, req.Proto)
				return ctx, &http.Response{
					StatusCode: http.StatusFound,
					Header: http.Header{
						"Location": []string{rawurl},
					},
					Request:       req,
					Close:         true,
					ContentLength: -1,
				}, nil
			}
		case "/books":
			if req.URL.Host == "books.google.cn" {
				rawurl := strings.Replace(req.URL.String(), "books.google.cn", "books.google.com", 1)
				glog.V(2).Infof("%s \"GAE REDIRECT %s %s %s\" - -", req.RemoteAddr, req.Method, rawurl, req.Proto)
				return ctx, &http.Response{
					StatusCode: http.StatusFound,
					Header: http.Header{
						"Location": []string{rawurl},
					},
					Request:       req,
					Close:         true,
					ContentLength: -1,
				}, nil
			}
		}

		if req.URL.Scheme != "http" && !f.shouldForceGAE(req) {
			tr = f.Transport
			if s := req.Header.Get("Connection"); s != "" {
				if s1 := strings.ToLower(s); s != s1 {
					req.Header.Set("Connection", s1)
				}
			}
		}
	}

	if tr != f.Transport && req.Method == http.MethodOptions {
		if v, ok := f.FakeOptionsMatcher.Lookup(req.Host); ok {
			resp := &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{},
				Request:       req,
				Close:         false,
				ContentLength: -1,
			}
			for _, s := range v.([]string) {
				parts := strings.SplitN(s, ":", 2)
				if len(parts) == 2 {
					resp.Header.Add(parts[0], strings.TrimSpace(parts[1]))
				}
			}
			if origin := req.Header.Get("Origin"); origin != "" {
				resp.Header.Set("Access-Control-Allow-Origin", origin)
			}
			if headers := req.Header.Get("Access-Control-Request-Headers"); headers != "" {
				resp.Header.Set("Access-Control-Allow-Headers", headers)
			}
			glog.V(2).Infof("%s \"GAE FAKEOPTIONS %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
			return ctx, resp, nil
		}
	}

	if tr == f.GAETransport {
		switch strings.ToLower(req.Header.Get("Connection")) {
		case "upgrade":
			resp := &http.Response{
				StatusCode: http.StatusForbidden,
				Header: http.Header{
					"X-WebSocket-Reject-Reason": []string{"Unsupported"},
				},
				Request:       req,
				Close:         true,
				ContentLength: 0,
			}
			glog.V(2).Infof("%s \"GAE FAKEWEBSOCKET %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
			return ctx, resp, nil
		}
	}

	prefix := "FETCH"
	if tr == f.Transport {
		prefix = "DIRECT"
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		glog.Warningf("%s \"GAE %s %s %s %s\" error: %T(%v)", req.RemoteAddr, prefix, req.Method, req.URL.String(), req.Proto, err, err)
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return ctx, nil, err
	}

	if resp != nil && resp.Header != nil {
		resp.Header.Del("Alt-Svc")
		resp.Header.Del("Alternate-Protocol")
	}

	glog.V(2).Infof("%s \"GAE %s %s %s %s\" %d %s", req.RemoteAddr, prefix, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	return ctx, resp, err
}

func (f *Filter) shouldForceGAE(req *http.Request) bool {
	if f.ForceGAEMatcher.Match(req.Host) {
		return true
	}

	u := req.URL.String()

	if len(f.ForceGAESuffixs) > 0 {
		for _, s := range f.ForceGAESuffixs {
			if strings.HasSuffix(u, s) {
				return true
			}
		}
	}

	if len(f.ForceGAEStrings) > 0 {
		for _, s := range f.ForceGAEStrings {
			if strings.Contains(u, s) {
				return true
			}
		}
	}

	return false
}
