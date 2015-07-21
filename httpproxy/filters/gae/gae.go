package gae

import (
	"fmt"
	"math/rand"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/phuslu/goproxy/httpproxy"
	"github.com/phuslu/goproxy/httpproxy/filters"
)

const (
	filterName string = "gae"
)

type Filter struct {
	FetchServers   []*FetchServer
	muFetchServers sync.Mutex
	Transport      filters.RoundTripFilter
	Sites          *httpproxy.HostMatcher
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
	ctx, resp, err := f.roundTrip(ctx, req)

	if err != nil || resp == nil {
		return ctx, resp, err
	}

	if resp.StatusCode == 206 {
		return ctx, resp, err
	}

	return ctx, resp, err
}

func (f *Filter) roundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {
	i := 0
	if strings.HasPrefix(mime.TypeByExtension(path.Ext(req.URL.Path)), "image/") {
		i += rand.Intn(len(f.FetchServers) - i)
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
		return ctx, resp, err
	} else {
		glog.Infof("%s \"GAE %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}

	switch resp.StatusCode {
	case 503:
		if len(f.FetchServers) == 1 {
			break
		}
		glog.Warningf("%s over qouta, switch to next appid.", fetchServer.URL.String())
		f.muFetchServers.Lock()
		if fetchServer == f.FetchServers[0] {
			for i := 1; i < len(f.FetchServers)-1; i++ {
				f.FetchServers[i] = f.FetchServers[i+1]
			}
			f.FetchServers[len(f.FetchServers)-1] = fetchServer
		}
		f.muFetchServers.Unlock()
		resp := &http.Response{
			Status:     "302 Moved Temporarily",
			StatusCode: 302,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header: http.Header{
				"Location": []string{req.URL.String()},
			},
			Request:       req,
			Close:         false,
			ContentLength: 0,
		}
		return ctx, resp, nil
	default:
		break
	}

	resp1, err := fetchServer.decodeResponse(resp)
	if resp1 != nil {
		resp1.Request = req
	}

	return ctx, resp1, err
}
