package gae

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/net/http2"

	"../../dialer"
	"../../filters"
	"../../helpers"
	"../../storage"
)

const (
	filterName string = "gae"
)

type Config struct {
	AppIDs          []string
	Password        string
	SSLVerify       bool
	ForceIPv6       bool
	DisableHTTP2    bool
	ForceHTTP2      bool
	EnableDeadProbe bool
	Sites           []string
	Site2Alias      map[string]string
	HostMap         map[string][]string
	TLSConfig       struct {
		Version                string
		SSLVerify              bool
		ClientSessionCacheSize int
		Ciphers                []string
		ServerName             []string
	}
	GoogleG2KeyID string
	ForceHTTPS    []string
	ForceGAE      []string
	ForceDeflate  []string
	FakeOptions   map[string][]string
	DNSServers    []string
	IPBlackList   []string
	Transport     struct {
		Dialer struct {
			DNSCacheExpiry   int
			DNSCacheSize     uint
			SocketReadBuffer int
			DualStack        bool
			KeepAlive        int
			Level            int
			Timeout          int
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
	MultiDialer         *dialer.MultiDialer
	GAETransport        *Transport
	DirectTransport     http.RoundTripper
	ForceHTTPSMatcher   *helpers.HostMatcher
	ForceGAEStrings     []string
	ForceGAEMatcher     *helpers.HostMatcher
	ForceDeflateMatcher *helpers.HostMatcher
	FakeOptionsMatcher  *helpers.HostMatcher
	SiteMatcher         *helpers.HostMatcher
	DirectSiteMatcher   *helpers.HostMatcher
}

func init() {
	filename := filterName + ".json"
	config := new(Config)
	err := storage.ReadJsonConfig(storage.LookupConfigStoreURI(filterName), filename, config)
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

	// curl -s https://pki.google.com/GIAG2.crt | openssl x509 -inform der -pubkey -text
	googleG2KeyID, err := hex.DecodeString(config.GoogleG2KeyID)
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

	d := &dialer.MultiDialer{
		Dialer: net.Dialer{
			KeepAlive: time.Duration(config.Transport.Dialer.KeepAlive) * time.Second,
			Timeout:   time.Duration(config.Transport.Dialer.Timeout) * time.Second,
			DualStack: config.Transport.Dialer.DualStack,
		},
		ForceIPv6:         config.ForceIPv6,
		SSLVerify:         config.SSLVerify,
		TLSConfig:         nil,
		Site2Alias:        helpers.NewHostMatcherWithString(config.Site2Alias),
		IPBlackList:       lrucache.NewLRUCache(8192),
		HostMap:           config.HostMap,
		GoogleTLSConfig:   googleTLSConfig,
		GoogleG2KeyID:     googleG2KeyID,
		DNSServers:        dnsServers,
		DNSCache:          lrucache.NewLRUCache(config.Transport.Dialer.DNSCacheSize),
		DNSCacheExpiry:    time.Duration(config.Transport.Dialer.DNSCacheExpiry) * time.Second,
		TCPConnDuration:   lrucache.NewLRUCache(8192),
		TCPConnError:      lrucache.NewLRUCache(8192),
		TLSConnDuration:   lrucache.NewLRUCache(8192),
		TLSConnError:      lrucache.NewLRUCache(8192),
		TCPConnReadBuffer: config.Transport.Dialer.SocketReadBuffer,
		TLSConnReadBuffer: config.Transport.Dialer.SocketReadBuffer,
		ConnExpiry:        5 * time.Minute,
		Level:             config.Transport.Dialer.Level,
	}

	for _, ip := range config.IPBlackList {
		d.IPBlackList.Set(ip, struct{}{}, time.Time{})
	}

	var tr http.RoundTripper

	t1 := &http.Transport{
		Dial:                  d.Dial,
		DialTLS:               d.DialTLS,
		DisableKeepAlives:     config.Transport.DisableKeepAlives,
		DisableCompression:    config.Transport.DisableCompression,
		ResponseHeaderTimeout: time.Duration(config.Transport.ResponseHeaderTimeout) * time.Second,
		IdleConnTimeout:       time.Duration(config.Transport.IdleConnTimeout) * time.Second,
		MaxIdleConnsPerHost:   config.Transport.MaxIdleConnsPerHost,
	}

	t2 := &http2.Transport{
		DialTLS:            d.DialTLS2,
		TLSClientConfig:    d.GoogleTLSConfig,
		DisableCompression: config.Transport.DisableCompression,
	}

	switch {
	case config.DisableHTTP2 && config.ForceHTTP2:
		glog.Fatalf("GAE: DisableHTTP2=%v and ForceHTTPS=%v is conflict!", config.DisableHTTP2, config.ForceHTTP2)
	case config.ForceHTTP2:
		tr = t2
	case !config.DisableHTTP2:
		err := http2.ConfigureTransport(t1)
		if err != nil {
			glog.Warningf("GAE: Error enabling Transport HTTP/2 support: %v", err)
		}
		tr = t1
	default:
		tr = t1
	}

	forceGAEStrings := make([]string, 0)
	forceGAEMatcherStrings := make([]string, 0)
	for _, s := range config.ForceGAE {
		if strings.Contains(s, "/") {
			forceGAEStrings = append(forceGAEStrings, s)
		} else {
			forceGAEMatcherStrings = append(forceGAEMatcherStrings, s)
		}
	}

	if config.EnableDeadProbe {
		go func() {
			probe := func() {
				req, _ := http.NewRequest(http.MethodGet, "https://clients3.google.com/generate_204", nil)
				ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
				defer cancel()
				req = req.WithContext(ctx)
				resp, err := tr.RoundTrip(req)
				if resp != nil && resp.Body != nil {
					glog.V(3).Infof("GAE EnableDeadProbe \"%s %s\" %d -", req.Method, req.URL.String(), resp.StatusCode)
					resp.Body.Close()
				}
				if err != nil {
					glog.V(2).Infof("GAE EnableDeadProbe \"%s %s\" error: %v", req.Method, req.URL.String(), err)
					s := strings.ToLower(err.Error())
					if strings.HasPrefix(s, "net/http: request canceled") || strings.Contains(s, "timeout") {
						helpers.TryCloseConnections(tr)
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

	return &Filter{
		Config:      *config,
		MultiDialer: d,
		GAETransport: &Transport{
			RoundTripper: tr,
			MultiDialer:  d,
			Servers: NewServers(config.AppIDs,
				config.Password,
				config.SSLVerify,
				time.Duration(config.Transport.ResponseHeaderTimeout-4)*time.Second),
			RetryDelay: time.Duration(config.Transport.RetryDelay*1000) * time.Second,
			RetryTimes: config.Transport.RetryTimes,
		},
		DirectTransport:     tr,
		ForceHTTPSMatcher:   helpers.NewHostMatcher(config.ForceHTTPS),
		ForceGAEMatcher:     helpers.NewHostMatcher(forceGAEMatcherStrings),
		ForceGAEStrings:     forceGAEStrings,
		ForceDeflateMatcher: helpers.NewHostMatcher(config.ForceDeflate),
		FakeOptionsMatcher:  helpers.NewHostMatcherWithStrings(config.FakeOptions),
		SiteMatcher:         helpers.NewHostMatcher(config.Sites),
		DirectSiteMatcher:   helpers.NewHostMatcherWithString(config.Site2Alias),
	}, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	if !f.SiteMatcher.Match(req.Host) {
		return ctx, nil, nil
	}

	var tr http.RoundTripper = f.GAETransport

	if req.URL.Scheme == "http" && f.ForceHTTPSMatcher.Match(req.Host) {
		if !strings.HasPrefix(req.Header.Get("Referer"), "https://") {
			u := strings.Replace(req.URL.String(), "http://", "https://", 1)
			glog.V(2).Infof("GAE FORCEHTTPS get raw url=%v, redirect to %v", req.URL.String(), u)
			resp := &http.Response{
				Status:     "301 Moved Permanently",
				StatusCode: http.StatusMovedPermanently,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
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
		if req.URL.Path == "/url" {
			if u := req.URL.Query().Get("url"); u != "" {
				glog.V(2).Infof("GAE REDIRECT get raw url=%v, redirect to %v", req.URL.String(), u)
				return ctx, &http.Response{
					Status:     "302 Found",
					StatusCode: http.StatusFound,
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					Header: http.Header{
						"Location": []string{u},
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
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
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
			glog.V(2).Infof("%s \"GAE FAKEOPTIONS %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
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
				helpers.TryCloseConnections(tr)
			}
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return ctx, nil, err
	}

	if f.ForceDeflateMatcher.Match(req.Host) &&
		resp.Header.Get("Content-Encoding") == "" &&
		strings.HasPrefix(resp.Header.Get("Content-Type"), "text/") {
		buf := make([]byte, 1024)
		n, err := resp.Body.Read(buf)
		if err != nil {
			defer resp.Body.Close()
			return ctx, nil, err
		}
		buf = buf[:n]
		switch {
		case helpers.IsGzip(buf):
			resp.Header.Set("Content-Encoding", "gzip")
		case helpers.IsBinary(buf):
			resp.Header.Set("Content-Encoding", "deflate")
		}
		resp.Body = helpers.NewMultiReadCloser(bytes.NewReader(buf), resp.Body)
	}

	glog.V(2).Infof("%s \"GAE %s %s %s %s\" %d %s", req.RemoteAddr, prefix, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	return ctx, resp, err
}

func (f *Filter) shouldForceGAE(req *http.Request) bool {
	if f.ForceGAEMatcher.Match(req.Host) {
		return true
	}

	if len(f.ForceGAEStrings) > 0 {
		for _, s := range f.ForceGAEStrings {
			if strings.Contains(req.URL.String(), s) {
				return true
			}
		}
	}

	return false
}
