package rewrite

import (
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
}

type Filter struct {
	Config
	UserAgentEnabled bool
	UserAgentValue   string
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
	f := &Filter{
		Config:           *config,
		UserAgentEnabled: config.UserAgent.Enabled,
		UserAgentValue:   config.UserAgent.Value,
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) Request(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Request, error) {
	if f.UserAgentEnabled {
		glog.V(3).Infof("REWRITE %#v User-Agent=%#v", req.URL.String(), f.UserAgentValue)
		req.Header.Set("User-Agent", f.UserAgentValue)
	}

	return ctx, req, nil
}

func (f *Filter) Response(ctx *filters.Context, resp *http.Response) (*filters.Context, *http.Response, error) {
	return ctx, resp, nil
}
