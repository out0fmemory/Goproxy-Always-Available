package autorange

import (
	"net/http"

	"github.com/golang/glog"

	"../../../httpproxy"
	"../../../storage"
	"../../filters"
)

const (
	filterName string = "autorange"
)

type Config struct {
	Sites   []string
	Suffixs []string
	MaxSize int
	BufSize int
	Threads int
}

type Filter struct {
	SiteMatcher *httpproxy.HostMatcher
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
	f := &Filter{
		SiteMatcher: httpproxy.NewHostMatcher(config.Sites),
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) Request(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Request, error) {
	return ctx, req, nil
}

func (f *Filter) Response(ctx *filters.Context, resp *http.Response) (*filters.Context, *http.Response, error) {
	if !f.SiteMatcher.Match(resp.Request.Host) {
		return ctx, resp, nil
	}

	if resp.StatusCode != http.StatusPartialContent {
		return ctx, resp, nil
	}

	f1 := ctx.GetRoundTripFilter()
	if f1 == nil {
		return ctx, resp, nil
	}

	glog.V(2).Infof("AUTORANGE")

	return ctx, resp, nil
}
