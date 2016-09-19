package rewrite

import (
	"context"
	"net/http"

	"github.com/phuslu/glog"

	"../../filters"
	"../../storage"
)

const (
	filterName string = "rewrite"
)

type Config struct {
	UserAgent struct {
		Enabled bool
		Value   string
	}
	Host struct {
		Enabled   bool
		RewriteBy string
	}
}

type Filter struct {
	Config
	UserAgentEnabled bool
	UserAgentValue   string
	HostEnabled      bool
	HostRewriteBy    string
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
	f := &Filter{
		Config:           *config,
		UserAgentEnabled: config.UserAgent.Enabled,
		UserAgentValue:   config.UserAgent.Value,
		HostEnabled:      config.Host.Enabled,
		HostRewriteBy:    config.Host.RewriteBy,
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) Request(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	if f.UserAgentEnabled {
		glog.V(3).Infof("REWRITE %#v User-Agent=%#v", req.URL.String(), f.UserAgentValue)
		req.Header.Set("User-Agent", f.UserAgentValue)
	}

	if f.HostEnabled {
		if host := req.Header.Get(f.HostRewriteBy); host != "" {
			glog.V(3).Infof("REWRITE %#v Host=%#v", req.URL.String(), host)
			req.Host = host
			req.Header.Set("Host", req.Host)
			req.Header.Del(f.HostRewriteBy)
		}
	}

	return ctx, req, nil
}

func (f *Filter) Response(ctx context.Context, resp *http.Response) (context.Context, *http.Response, error) {
	return ctx, resp, nil
}
