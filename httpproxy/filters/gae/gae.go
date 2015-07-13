package gae

import (
	"fmt"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/golang/glog"
	"github.com/phuslu/goproxy/httpproxy"
	"github.com/phuslu/goproxy/httpproxy/filters"
)

const (
	filterName string = "gae"
)

type Filter struct {
	FetchServers []*FetchServer
	Transport    filters.RoundTripFilter
	Sites        *httpproxy.HostMatcher
}

func init() {
	filename := filterName + ".json"
	config, err := NewConfig(filters.LookupConfigStoreURI(filterName), filename)
	if err != nil {
		glog.Fatalf("NewConfig(%#v) failed: %s", filename, err)
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
	f1, err := filters.GetFilter(config.Transport)
	if err != nil {
		return nil, err
	}

	f2, ok := f1.(filters.RoundTripFilter)
	if !ok {
		return nil, fmt.Errorf("%#v was not a filters.RoundTripFilter", f1)
	}

	fetchServers := make([]*FetchServer, 0)
	for _, appid := range config.AppIds {
		u, err := url.Parse(fmt.Sprintf("%s://%s.%s%s", config.Scheme, appid, config.Domain, config.Path))
		if err != nil {
			return nil, err
		}

		fs := &FetchServer{
			URL:       u,
			Password:  config.Password,
			SSLVerify: config.SSLVerify,
		}

		fetchServers = append(fetchServers, fs)
	}

	return &Filter{
		FetchServers: fetchServers,
		Transport:    f2,
		Sites:        httpproxy.NewHostMatcher(config.Sites),
	}, nil
}

func (p *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {
	i := 0
	if strings.HasPrefix(mime.TypeByExtension(path.Ext(req.URL.Path)), "image/") {
		i = rand.Intn(len(f.FetchServers))
	}

	fetchServer := f.FetchServers[i]

	req1, err := fetchServer.encodeRequest(req)
	if err != nil {
		return ctx, nil, fmt.Errorf("GAE encodeRequest: %s", err.Error())
	}
	req1.Header = req.Header

	ctx, resp, err := f.Transport.RoundTrip(ctx, req1)
	if err != nil || resp == nil {
		glog.Errorf("%s \"GAE %s %s %s\" %#v %v", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp, err)
		return ctx, nil, err
	} else {
		glog.Infof("%s \"GAE %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}

	resp1, err := fetchServer.decodeResponse(resp)
	return ctx, resp1, err
}
