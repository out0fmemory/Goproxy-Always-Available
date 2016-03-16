package gae

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

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
			Timeout         int
			KeepAlive       int
			DualStack       bool
			RetryTimes      int
			RetryDelay      float32
			DNSCacheExpires int
			DNSCacheSize    uint
		}
		DisableKeepAlives   bool
		DisableCompression  bool
		TLSHandshakeTimeout int
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
	d := &direct.Dialer{
		Dialer: net.Dialer{
			KeepAlive: time.Duration(config.Transport.Dialer.KeepAlive) * time.Second,
			Timeout:   time.Duration(config.Transport.Dialer.Timeout) * time.Second,
			DualStack: config.Transport.Dialer.DualStack,
		},
		RetryTimes:      config.Transport.Dialer.RetryTimes,
		RetryDelay:      time.Duration(config.Transport.Dialer.RetryDelay*1000) * time.Second,
		DNSCacheExpires: time.Duration(config.Transport.Dialer.DNSCacheExpires) * time.Second,
		DNSCacheSize:    config.Transport.Dialer.DNSCacheSize,
		Level:           2,
	}

	tr := &http.Transport{
		Dial: d.Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		},
		TLSHandshakeTimeout: time.Duration(config.Transport.TLSHandshakeTimeout) * time.Second,
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
