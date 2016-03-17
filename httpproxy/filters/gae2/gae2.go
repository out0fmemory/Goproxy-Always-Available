package gae

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"

	"../../../httpproxy"
	"../../../storage"
	"../../filters"
	"../../transport/direct"
	"../../transport/gae"
)

const (
	filterName string = "gae2"
)

type Config struct {
	AppIDs      []string
	Scheme      string
	Domain      string
	Path        string
	Password    string
	SSLVerify   bool
	Sites       []string
	Site2Alias  map[string]string
	HostMap     map[string][]string
	DNSServers  []string
	IPBlackList []string
	Transport   struct {
		Dialer struct {
			Timeout        int
			KeepAlive      int
			DualStack      bool
			RetryTimes     int
			RetryDelay     float32
			DNSCacheExpiry int
			DNSCacheSize   uint
			Level          int
		}
		DisableKeepAlives   bool
		DisableCompression  bool
		MaxIdleConnsPerHost int
	}
}

type Filter struct {
	GAETransport      *gae.Transport
	DirectTransport   *http.Transport
	SiteMatcher       *httpproxy.HostMatcher
	DirectSiteMatcher *httpproxy.HostMatcher
}

func init() {
	filename := filterName + ".json"
	config := new(Config)
	err := storage.ReadJsonConfig(filters.LookupConfigStoreURI(filterName), filename, config)
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

	d := &direct.MultiDialer{
		Dialer: net.Dialer{
			KeepAlive: time.Duration(config.Transport.Dialer.KeepAlive) * time.Second,
			Timeout:   time.Duration(config.Transport.Dialer.Timeout) * time.Second,
			DualStack: config.Transport.Dialer.DualStack,
		},
		TLSConfig:       nil,
		Site2Alias:      httpproxy.NewHostMatcherWithString(config.Site2Alias),
		IPBlackList:     httpproxy.NewHostMatcher(config.IPBlackList),
		HostMap:         config.HostMap,
		DNSServers:      dnsServers,
		DNSCache:        lrucache.NewLRUCache(config.Transport.Dialer.DNSCacheSize),
		DNSCacheExpiry:  time.Duration(config.Transport.Dialer.DNSCacheExpiry) * time.Second,
		TCPConnDuration: lrucache.NewLRUCache(8192),
		TLSConnDuration: lrucache.NewLRUCache(8192),
		ConnExpiry:      5 * time.Minute,
		Level:           config.Transport.Dialer.Level,
	}

	tr := &http.Transport{
		Dial:                d.Dial,
		DialTLS:             d.DialTLS,
		MaxIdleConnsPerHost: config.Transport.MaxIdleConnsPerHost,
	}

	servers := make([]gae.Server, 0)
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

		server := gae.Server{
			URL:       u,
			Password:  config.Password,
			SSLVerify: config.SSLVerify,
		}

		servers = append(servers, server)
	}

	return &Filter{
		GAETransport: &gae.Transport{
			RoundTripper: tr,
			Servers:      servers,
		},
		DirectTransport:   tr,
		SiteMatcher:       httpproxy.NewHostMatcher(config.Sites),
		DirectSiteMatcher: httpproxy.NewHostMatcherWithString(config.Site2Alias),
	}, nil
}

func (p *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {
	if !f.SiteMatcher.Match(req.Host) {
		return ctx, nil, nil
	}

	var tr http.RoundTripper = f.GAETransport
	prefix := "FETCH"

	if f.DirectSiteMatcher.Match(req.Host) {
		tr = f.DirectTransport
		prefix = "DIRECT"
	}

	resp, err := tr.RoundTrip(req)

	if err != nil {
		return ctx, nil, err
	} else {
		glog.Infof("%s \"GAE %s %s %s %s\" %d %s", req.RemoteAddr, prefix, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}

	return ctx, resp, nil
}
