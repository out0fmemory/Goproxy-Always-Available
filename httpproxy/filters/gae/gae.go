package gae

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/net/http2"

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
	GAETransport       *Transport
	DirectTransport    http.RoundTripper
	ForceHTTPSMatcher  *helpers.HostMatcher
	ForceGAEStrings    []string
	ForceGAESuffixs    []string
	ForceGAEMatcher    *helpers.HostMatcher
	FakeOptionsMatcher *helpers.HostMatcher
	SiteMatcher        *helpers.HostMatcher
	DirectSiteMatcher  *helpers.HostMatcher
}

func init() {
	filename := filterName + ".json"
	config := new(Config)
	err := storage.LookupStoreByFilterName(filterName).UnmarshallJson(filename, config)
	if err != nil {
		glog.Fatalf("storage.ReadJsonConfig(%#v) failed: %s", filename, err)
	}

	err = filters.Register(filterName, &filters.RegisteredFilter{
		New: func() (filters.Filter, error) {
			return NewFilter(config)
		},
	})

	if err != nil {
		glog.Fatalf("Register(%#v) error: %s", filterName, err)
	}
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
	case "TLSv12", "TLSv1.2":
		googleTLSConfig.MinVersion = tls.VersionTLS12
	case "TLSv11", "TLSv1.1":
		googleTLSConfig.MinVersion = tls.VersionTLS11
	default:
		googleTLSConfig.MinVersion = tls.VersionTLS10
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
		ciphers = ciphers[:1+rand.Intn(len(ciphers))]
		ciphers1 := []uint16{}
		for _, name := range []string{
			"TLS_RSA_WITH_AES_256_CBC_SHA256",
			"TLS_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		} {
			if !helpers.ContainsString(names, name) {
				if c := helpers.Cipher(name); c != 0 {
					ciphers1 = append(ciphers1, c)
				}
			}
		}
		helpers.ShuffleUint16s(ciphers1)
		ciphers1 = ciphers1[:rand.Intn(len(ciphers1))]
		ciphers = append(ciphers, ciphers1...)
		helpers.ShuffleUint16s(ciphers)
		return ciphers
	}
	googleTLSConfig.CipherSuites = pickupCiphers(config.TLSConfig.Ciphers)
	if len(config.TLSConfig.ServerName) > 0 {
		googleTLSConfig.ServerName = config.TLSConfig.ServerName[rand.Intn(len(config.TLSConfig.ServerName))]
	}
	if !config.DisableHTTP2 {
		googleTLSConfig.NextProtos = []string{"h2", "h2-14", "http/1.1"}
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
		hostmap[key] = helpers.UniqueStrings(value)
	}

	d := &net.Dialer{
		KeepAlive: time.Duration(config.Transport.Dialer.KeepAlive) * time.Second,
		Timeout:   time.Duration(config.Transport.Dialer.Timeout) * time.Second,
		DualStack: config.Transport.Dialer.DualStack,
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
		Dialer:            d,
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
		ConnExpiry:        5 * time.Minute,
		Level:             config.Transport.Dialer.Level,
	}

	for _, ip := range config.IPBlackList {
		md.IPBlackList.Set(ip, struct{}{}, time.Time{})
	}

	var tr http.RoundTripper

	t1 := &http.Transport{
		Dial:                  md.Dial,
		DialTLS:               md.DialTLS,
		DisableKeepAlives:     config.Transport.DisableKeepAlives,
		DisableCompression:    config.Transport.DisableCompression,
		ResponseHeaderTimeout: time.Duration(config.Transport.ResponseHeaderTimeout) * time.Second,
		IdleConnTimeout:       time.Duration(config.Transport.IdleConnTimeout) * time.Second,
		MaxIdleConnsPerHost:   config.Transport.MaxIdleConnsPerHost,
	}

	if config.Transport.Proxy.Enabled {
		fixedURL, err := url.Parse(config.Transport.Proxy.URL)
		if err != nil {
			glog.Fatalf("url.Parse(%#v) error: %s", config.Transport.Proxy.URL, err)
		}

		dialer, err := proxy.FromURL(fixedURL, d, &helpers.MultiResolver{md})
		if err != nil {
			glog.Fatalf("proxy.FromURL(%#v) error: %s", fixedURL.String(), err)
		}

		t1.Dial = dialer.Dial
		t1.DialTLS = nil
		t1.Proxy = nil
		t1.TLSClientConfig = md.GoogleTLSConfig
	}

	switch {
	case config.DisableHTTP2 && config.ForceHTTP2:
		glog.Fatalf("GAE: DisableHTTP2=%v and ForceHTTP2=%v is conflict!", config.DisableHTTP2, config.ForceHTTP2)
	case config.Transport.Proxy.Enabled && config.ForceHTTP2:
		glog.Fatalf("GAE: Proxy.Enabled=%v and ForceHTTP2=%v is conflict!", config.Transport.Proxy.Enabled, config.ForceHTTP2)
	case config.ForceHTTP2:
		tr = &http2.Transport{
			DialTLS:            md.DialTLS2,
			TLSClientConfig:    md.GoogleTLSConfig,
			DisableCompression: config.Transport.DisableCompression,
		}
	case !config.DisableHTTP2:
		err := http2.ConfigureTransport(t1)
		if err != nil {
			glog.Warningf("GAE: Error enabling Transport HTTP/2 support: %v", err)
		}
		tr = t1
	default:
		tr = t1
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
		go func() {
			probe := func() {
				req, _ := http.NewRequest(http.MethodGet, "https://clients3.google.com/generate_204", nil)
				ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
				defer cancel()
				req = req.WithContext(ctx)
				resp, err := tr.RoundTrip(req)
				if resp != nil && resp.Body != nil {
					glog.V(3).Infof("GAE EnableDeadProbe \"%s %s\" %d -", req.Method, req.URL.String(), resp.StatusCode)
					defer resp.Body.Close()
					if resp.StatusCode == http.StatusBadGateway {
						if ip, err := helpers.ReflectRemoteIPFromResponse(resp); err == nil && ip != nil {
							duration := 1 * time.Hour
							glog.Warningf("GAE EnableDeadProbe: %s StatusCode is %d, not a gws/gvs ip, add to blacklist for %v", ip, resp.StatusCode, duration)
							md.IPBlackList.Set(ip.String(), struct{}{}, time.Now().Add(duration))
						}
					}
				}
				if err != nil {
					glog.V(2).Infof("GAE EnableDeadProbe \"%s %s\" error: %v", req.Method, req.URL.String(), err)
					s := strings.ToLower(err.Error())
					if strings.HasPrefix(s, "net/http: request canceled") || strings.Contains(s, "timeout") {
						helpers.CloseConnections(tr)
					}
				}
			}

			for {
				time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
				probe()
			}
		}()
	}

	helpers.ShuffleStrings(config.AppIDs)

	f := &Filter{
		Config: *config,
		GAETransport: &Transport{
			RoundTripper: tr,
			MultiDialer:  md,
			Servers:      NewServers(config.AppIDs, config.Password, config.SSLVerify),
			Deadline:     time.Duration(config.Transport.ResponseHeaderTimeout-2) * time.Second,
			RetryDelay:   time.Duration(config.Transport.RetryDelay*1000) * time.Second,
			RetryTimes:   config.Transport.RetryTimes,
		},
		DirectTransport:    tr,
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

	if req.URL.Scheme == "http" && f.ForceHTTPSMatcher.Match(req.Host) {
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
			tr = f.DirectTransport
			if s := req.Header.Get("Connection"); s != "" {
				if s1 := strings.ToLower(s); s != s1 {
					req.Header.Set("Connection", s1)
				}
			}
		}
	}

	if tr != f.DirectTransport && req.Method == http.MethodOptions {
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
	if tr == f.DirectTransport {
		prefix = "DIRECT"
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		glog.Warningf("%s \"GAE %s %s %s %s\" error: %T(%v)", req.RemoteAddr, prefix, req.Method, req.URL.String(), req.Proto, err, err)
		if tr == f.DirectTransport {
			if ne, ok := err.(interface {
				Timeout() bool
			}); ok && ne.Timeout() {
				// f.MultiDialer.ClearCache()
				helpers.CloseConnections(tr)
			}
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return ctx, nil, err
	}

	if tr == f.DirectTransport && resp.StatusCode >= http.StatusBadRequest {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return ctx, nil, err
		}
		if addr, err := helpers.ReflectRemoteAddrFromResponse(resp); err == nil {
			if ip, _, err := net.SplitHostPort(addr); err == nil {
				var duration time.Duration
				switch {
				case resp.StatusCode == http.StatusBadGateway && bytes.Contains(body, []byte("Please try again in 30 seconds.")):
					duration = 1 * time.Hour
				case resp.StatusCode == http.StatusNotFound && bytes.Contains(body, []byte("<ins>That’s all we know.</ins>")):
					server := resp.Header.Get("Server")
					if server != "gws" && !strings.HasPrefix(server, "gvs") {
						if md := f.GAETransport.MultiDialer; md != nil && md.TLSConnDuration.Len() > 10 {
							duration = 5 * time.Minute
						}
					}
				}

				if duration > 0 && f.GAETransport.MultiDialer != nil {
					glog.Warningf("GAE: %s StatusCode is %d, not a gws/gvs ip, add to blacklist for %v", ip, resp.StatusCode, duration)
					f.GAETransport.MultiDialer.IPBlackList.Set(ip, struct{}{}, time.Now().Add(duration))

					if !helpers.CloseConnectionByRemoteAddr(tr, addr) {
						glog.Warningf("GAE: CloseConnectionByRemoteAddr(%T, %#v) failed.", tr, addr)
					}
				}
			}
		}
		resp.Body.Close()
		resp.Body = ioutil.NopCloser(bytes.NewReader(body))
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
