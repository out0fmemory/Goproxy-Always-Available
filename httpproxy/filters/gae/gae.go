package gae

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

const (
	DefaultGAEScheme string = "https"
	DefaultGAEDomain string = "appspot.com"
	DefaultGAEPath   string = "/_gh/"
)

type Config struct {
	AppIDs          []string
	Scheme          string
	Domain          string
	Path            string
	Password        string
	SSLVerify       bool
	IPv6Only        bool
	DisableHTTP2    bool
	ForceHTTP2      bool
	Sites           []string
	Site2Alias      map[string]string
	HostMap         map[string][]string
	FakeServerNames []string
	ForceHTTPS      []string
	ForceGAE        []string
	FakeOptions     map[string][]string
	DNSServers      []string
	IPBlackList     []string
	Transport       struct {
		Dialer struct {
			DNSCacheExpiry int
			DNSCacheSize   uint
			DualStack      bool
			KeepAlive      int
			Level          int
			Timeout        int
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
	FakeOptionsMatcher *helpers.HostMatcher
	SiteMatcher        *helpers.HostMatcher
	DirectSiteMatcher  *helpers.HostMatcher
}

func init() {
	filename := filterName + ".json"
	config := new(Config)
	err := storage.ReadJsonConfig(storage.LookupConfigStoreURI(filterName), filename, config)
	if err != nil {
		glog.Fatalf("storage.ReadJsonConfig(%#v) failed: %s", filename, err)
	}

	if config.Scheme == "" {
		config.Scheme = DefaultGAEScheme
	}

	if config.Domain == "" {
		config.Domain = DefaultGAEDomain
	}

	if config.Path == "" {
		config.Path = DefaultGAEPath
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

	d := &dialer.MultiDialer{
		Dialer: net.Dialer{
			KeepAlive: time.Duration(config.Transport.Dialer.KeepAlive) * time.Second,
			Timeout:   time.Duration(config.Transport.Dialer.Timeout) * time.Second,
			DualStack: config.Transport.Dialer.DualStack,
		},
		IPv6Only:        config.IPv6Only,
		TLSConfig:       nil,
		Site2Alias:      helpers.NewHostMatcherWithString(config.Site2Alias),
		IPBlackList:     lrucache.NewLRUCache(8192),
		HostMap:         config.HostMap,
		FakeServerNames: config.FakeServerNames,
		DNSServers:      dnsServers,
		DNSCache:        lrucache.NewLRUCache(config.Transport.Dialer.DNSCacheSize),
		DNSCacheExpiry:  time.Duration(config.Transport.Dialer.DNSCacheExpiry) * time.Second,
		TCPConnDuration: lrucache.NewLRUCache(8192),
		TCPConnError:    lrucache.NewLRUCache(8192),
		TLSConnDuration: lrucache.NewLRUCache(8192),
		TLSConnError:    lrucache.NewLRUCache(8192),
		ConnExpiry:      5 * time.Minute,
		Level:           config.Transport.Dialer.Level,
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

	switch {
	case config.DisableHTTP2 && config.ForceHTTP2:
		glog.Fatalf("GAE: DisableHTTP2=%v and ForceHTTPS=%v is conflict!", config.DisableHTTP2, config.ForceHTTP2)
	case config.ForceHTTP2:
		_ = dialer.GetDefaultTLSConfigForGoogle(config.FakeServerNames)
		tr = &http2.Transport{
			DialTLS:            d.DialTLS2,
			TLSClientConfig:    dialer.GetDefaultTLSConfigForGoogle(config.FakeServerNames),
			DisableCompression: config.Transport.DisableCompression,
		}
	case config.DisableHTTP2:
		tr = t1
	default:
		err := http2.ConfigureTransport(t1)
		if err != nil {
			glog.Warningf("GAE: Error enabling Transport HTTP/2 support: %v", err)
		}
		tr = t1
	}

	servers := make([]Server, 0)
	for _, appid := range config.AppIDs {
		var rawurl string
		switch strings.Count(appid, ".") {
		case 0, 1:
			rawurl = fmt.Sprintf("%s://%s.%s%s", config.Scheme, appid, config.Domain, config.Path)
		default:
			rawurl = fmt.Sprintf("%s://%s.%s%s", config.Scheme, appid, config.Path)
		}
		u, err := url.Parse(rawurl)
		if err != nil {
			return nil, err
		}

		server := Server{
			URL:       u,
			Password:  config.Password,
			SSLVerify: config.SSLVerify,
			Deadline:  time.Duration(config.Transport.ResponseHeaderTimeout-4) * time.Second,
		}

		servers = append(servers, server)
	}

	return &Filter{
		Config: *config,
		GAETransport: &Transport{
			RoundTripper: tr,
			MultiDialer:  d,
			Servers:      servers,
			RetryDelay:   time.Duration(config.Transport.RetryDelay*1000) * time.Second,
			RetryTimes:   config.Transport.RetryTimes,
		},
		DirectTransport:    tr,
		ForceHTTPSMatcher:  helpers.NewHostMatcher(config.ForceHTTPS),
		ForceGAEStrings:    config.ForceGAE,
		FakeOptionsMatcher: helpers.NewHostMatcherWithStrings(config.FakeOptions),
		SiteMatcher:        helpers.NewHostMatcher(config.Sites),
		DirectSiteMatcher:  helpers.NewHostMatcherWithString(config.Site2Alias),
	}, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	if !f.SiteMatcher.Match(req.Host) {
		return ctx, nil, nil
	}

	if req.Method == http.MethodOptions {
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

	var tr http.RoundTripper = f.GAETransport
	prefix := "FETCH"

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
			prefix = "DIRECT"
		}
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		glog.Warningf("%s \"GAE %s %s %s %s\" error: %T(%v)", req.RemoteAddr, prefix, req.Method, req.URL.String(), req.Proto, err, err)
		if prefix == "DIRECT" {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				if t1, ok := tr.(*http.Transport); ok {
					t1.CloseIdleConnections()
				} else if t2, ok := tr.(*http2.Transport); ok {
					t2.CloseConnections()
				}
			}
		}
	} else {
		glog.V(2).Infof("%s \"GAE %s %s %s %s\" %d %s", req.RemoteAddr, prefix, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}

	return ctx, resp, nil
}

func (f *Filter) shouldForceGAE(req *http.Request) bool {
	if len(f.ForceGAEStrings) > 0 {
		for _, s := range f.ForceGAEStrings {
			if strings.Contains(req.URL.String(), s) {
				return true
			}
		}
	}
	return false
}
