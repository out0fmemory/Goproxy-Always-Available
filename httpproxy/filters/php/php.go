package php

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/net/http2"

	"../../dialer"
	"../../filters"
	"../../helpers"
	"../../proxy"
	"../../storage"
)

const (
	filterName string = "php"
)

type Config struct {
	Servers []struct {
		URL       string
		Password  string
		SSLVerify bool
		Host      string
	}
	Sites     []string
	Transport struct {
		Dialer struct {
			Timeout        int
			KeepAlive      int
			DualStack      bool
			RetryTimes     int
			RetryDelay     float32
			DNSCacheExpiry int
			DNSCacheSize   uint
		}
		Proxy struct {
			Enabled bool
			URL     string
		}
		DisableKeepAlives   bool
		DisableCompression  bool
		TLSHandshakeTimeout int
		MaxIdleConnsPerHost int
	}
}

type Filter struct {
	Config
	Transport *Transport
	Sites     *helpers.HostMatcher
}

func init() {
	filename := filterName + ".json"
	config := new(Config)
	err := storage.LookupStoreByConfig(filterName).UnmarshallJson(filename, config)
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
	servers := make([]Server, 0)
	for _, s := range config.Servers {
		u, err := url.Parse(s.URL)
		if err != nil {
			return nil, err
		}

		server := Server{
			URL:       u,
			Password:  s.Password,
			SSLVerify: s.SSLVerify,
			Host:      s.Host,
		}

		servers = append(servers, server)
	}

	d0 := &net.Dialer{
		KeepAlive: time.Duration(config.Transport.Dialer.KeepAlive) * time.Second,
		Timeout:   time.Duration(config.Transport.Dialer.Timeout) * time.Second,
		DualStack: config.Transport.Dialer.DualStack,
	}

	d := &dialer.Dialer{
		Dialer:         d0,
		RetryTimes:     config.Transport.Dialer.RetryTimes,
		RetryDelay:     time.Duration(config.Transport.Dialer.RetryDelay*1000) * time.Second,
		DNSCache:       lrucache.NewLRUCache(config.Transport.Dialer.DNSCacheSize),
		DNSCacheExpiry: time.Duration(config.Transport.Dialer.DNSCacheExpiry) * time.Second,
		LoopbackAddrs:  nil,
		Level:          2,
	}

	for _, server := range servers {
		if server.Host != "" {
			host := server.URL.Host
			if _, _, err := net.SplitHostPort(host); err != nil {
				if server.URL.Scheme == "https" {
					host += ":443"
				} else {
					host += ":80"
				}
			}

			host1 := server.Host
			if _, _, err := net.SplitHostPort(host1); err != nil {
				if server.URL.Scheme == "https" {
					host1 += ":443"
				} else {
					host1 += ":80"
				}
			}

			d.DNSCache.Set(host, host1, time.Time{})
		}
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

	if config.Transport.Proxy.Enabled {
		fixedURL, err := url.Parse(config.Transport.Proxy.URL)
		if err != nil {
			glog.Fatalf("url.Parse(%#v) error: %s", config.Transport.Proxy.URL, err)
		}

		switch fixedURL.Scheme {
		case "http", "https":
			tr.Proxy = http.ProxyURL(fixedURL)
			tr.Dial = nil
			tr.DialTLS = nil
		default:
			dialer, err := proxy.FromURL(fixedURL, d, nil)
			if err != nil {
				glog.Fatalf("proxy.FromURL(%#v) error: %s", fixedURL.String(), err)
			}

			tr.Dial = dialer.Dial
			tr.DialTLS = nil
			tr.Proxy = nil
		}
	}

	if tr.TLSClientConfig != nil {
		err := http2.ConfigureTransport(tr)
		if err != nil {
			glog.Warningf("PHP: Error enabling Transport HTTP/2 support: %v", err)
		}
	}

	return &Filter{
		Config: *config,
		Transport: &Transport{
			RoundTripper: tr,
			Servers:      servers,
		},
		Sites: helpers.NewHostMatcher(config.Sites),
	}, nil
}

func (p *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	if !f.Sites.Match(req.Host) {
		return ctx, nil, nil
	}

	resp, err := f.Transport.RoundTrip(req)
	if err != nil {
		return ctx, nil, err
	} else {
		glog.V(2).Infof("%s \"PHP %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}
	return ctx, resp, nil
}
