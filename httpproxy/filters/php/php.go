package php

import (
	"crypto/tls"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/golang/glog"

	"../../../httpproxy"
	"../../../storage"
	"../../filters"
	"../../transport/direct"
	"../../transport/php"
)

const (
	filterName string = "php"
)

type Config struct {
	Servers []struct {
		URL       string
		Password  string
		SSLVerify bool
	}
	Sites     []string
	Transport struct {
		Dialer struct {
			RetryTimes   int
			RetryDelay   int
			DNSCacheSize int
		}
		TLSHandshakeTimeout int
		MaxIdleConnsPerHost int
	}
}

type Filter struct {
	Transports []php.Transport
	Sites      *httpproxy.HostMatcher
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
		Dialer:          net.Dialer{},
		RetryTimes:      config.Transport.Dialer.RetryTimes,
		RetryDelay:      time.Duration(config.Transport.Dialer.RetryDelay) * time.Millisecond,
		DNSCacheExpires: 2 * time.Hour,
		DNSCacheSize:    uint(config.Transport.Dialer.DNSCacheSize),
	}

	tr := &http.Transport{
		Dial: d.Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		},
		TLSHandshakeTimeout: time.Duration(config.Transport.TLSHandshakeTimeout) * time.Millisecond,
		MaxIdleConnsPerHost: config.Transport.MaxIdleConnsPerHost,
	}

	transports := make([]php.Transport, 0)
	for _, fs := range config.Servers {
		u, err := url.Parse(fs.URL)
		if err != nil {
			return nil, err
		}

		tr := php.Transport{
			RoundTripper: tr,
			Server: php.Server{
				URL:       u,
				Password:  fs.Password,
				SSLVerify: fs.SSLVerify,
			},
		}

		transports = append(transports, tr)
	}

	return &Filter{
		Transports: transports,
		Sites:      httpproxy.NewHostMatcher(config.Sites),
	}, nil
}

func (p *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {
	if !f.Sites.Match(req.Host) {
		return ctx, nil, nil
	}

	i := 0
	switch path.Ext(req.URL.Path) {
	case ".jpg", ".png", ".webp", ".bmp", ".gif", ".flv", ".mp4":
		i = rand.Intn(len(f.Transports))
	case "":
		name := path.Base(req.URL.Path)
		if strings.Contains(name, "play") ||
			strings.Contains(name, "video") {
			i = rand.Intn(len(f.Transports))
		}
	default:
		if strings.Contains(req.URL.Host, "img.") ||
			strings.Contains(req.URL.Host, "cache.") ||
			strings.Contains(req.URL.Host, "video.") ||
			strings.Contains(req.URL.Host, "static.") ||
			strings.HasPrefix(req.URL.Host, "img") ||
			strings.HasPrefix(req.URL.Path, "/static") ||
			strings.HasPrefix(req.URL.Path, "/asset") ||
			strings.Contains(req.URL.Path, "min.js") ||
			strings.Contains(req.URL.Path, "static") ||
			strings.Contains(req.URL.Path, "asset") ||
			strings.Contains(req.URL.Path, "/cache/") {
			i = rand.Intn(len(f.Transports))
		}
	}

	tr := f.Transports[i]

	resp, err := tr.RoundTrip(req)
	if err != nil {
		return ctx, nil, err
	} else {
		glog.Infof("%s \"PHP %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}
	return ctx, resp, nil
}
