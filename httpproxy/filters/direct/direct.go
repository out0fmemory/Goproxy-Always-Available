package direct

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"

	"../../filters"
	"../../helpers"
	"../../proxy"
	"../../storage"
)

const (
	filterName string = "direct"
)

type Config struct {
	Transport struct {
		Dialer struct {
			Timeout        int
			KeepAlive      int
			DualStack      bool
			DNSCacheExpiry int
			DNSCacheSize   uint
		}
		Proxy struct {
			Enabled bool
			URL     string
		}
		TLSClientConfig struct {
			InsecureSkipVerify     bool
			ClientSessionCacheSize int
		}
		DisableKeepAlives   bool
		DisableCompression  bool
		TLSHandshakeTimeout int
		MaxIdleConnsPerHost int
	}
}

type Filter struct {
	Config
	filters.RoundTripFilter
	transport *http.Transport
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
	d := &helpers.Dialer{
		Dialer: &net.Dialer{
			KeepAlive: time.Duration(config.Transport.Dialer.KeepAlive) * time.Second,
			Timeout:   time.Duration(config.Transport.Dialer.Timeout) * time.Second,
			DualStack: config.Transport.Dialer.DualStack,
		},
		Resolver: &helpers.Resolver{
			LRUCache:  lrucache.NewLRUCache(config.Transport.Dialer.DNSCacheSize),
			DNSExpiry: time.Duration(config.Transport.Dialer.DNSCacheExpiry) * time.Second,
			BlackList: lrucache.NewLRUCache(1024),
		},
	}

	if ips, err := helpers.LocalIPv4s(); err == nil {
		for _, ip := range ips {
			d.Resolver.BlackList.Set(ip.String(), struct{}{}, time.Time{})
		}
		for _, s := range []string{"127.0.0.1", "::1"} {
			d.Resolver.BlackList.Set(s, struct{}{}, time.Time{})
		}
	}

	tr := &http.Transport{
		Dial: d.Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.Transport.TLSClientConfig.InsecureSkipVerify,
			ClientSessionCache: tls.NewLRUClientSessionCache(config.Transport.TLSClientConfig.ClientSessionCacheSize),
		},
		TLSHandshakeTimeout: time.Duration(config.Transport.TLSHandshakeTimeout) * time.Second,
		MaxIdleConnsPerHost: config.Transport.MaxIdleConnsPerHost,
		DisableCompression:  config.Transport.DisableCompression,
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

	return &Filter{
		Config:    *config,
		transport: tr,
	}, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	switch req.Method {
	case "CONNECT":
		glog.V(2).Infof("%s \"DIRECT %s %s %s\" - -", req.RemoteAddr, req.Method, req.Host, req.Proto)
		rconn, err := f.transport.Dial("tcp", req.Host)
		if err != nil {
			return ctx, nil, err
		}

		rw := filters.GetResponseWriter(ctx)

		hijacker, ok := rw.(http.Hijacker)
		if !ok {
			return ctx, nil, fmt.Errorf("http.ResponseWriter(%#v) does not implments http.Hijacker", rw)
		}

		flusher, ok := rw.(http.Flusher)
		if !ok {
			return ctx, nil, fmt.Errorf("http.ResponseWriter(%#v) does not implments http.Flusher", rw)
		}

		rw.WriteHeader(http.StatusOK)
		flusher.Flush()

		lconn, _, err := hijacker.Hijack()
		if err != nil {
			return ctx, nil, fmt.Errorf("%#v.Hijack() error: %v", hijacker, err)
		}
		defer lconn.Close()

		go helpers.IOCopy(rconn, lconn)
		helpers.IOCopy(lconn, rconn)

		return ctx, filters.DummyResponse, nil
	default:
		helpers.FixRequestURL(req)
		resp, err := f.transport.RoundTrip(req)

		if err != nil {
			return ctx, nil, err
		}

		if req.RemoteAddr != "" {
			glog.V(2).Infof("%s \"DIRECT %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
		}

		return ctx, resp, err
	}
}
