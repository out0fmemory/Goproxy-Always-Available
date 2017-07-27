package vps

import (
	"context"
	// "fmt"
	"math/rand"
	"net/http"
	"net/url"

	"github.com/phuslu/glog"
	"github.com/phuslu/net/http2"

	"../../filters"
	"../../helpers"
	"../../storage"
)

const (
	filterName string = "vps"
)

type Config struct {
	Servers []struct {
		URL       string
		Username  string
		Password  string
		SSLVerify bool
	}
}

type Filter struct {
	Servers []*Server
	Sites   *helpers.HostMatcher
}

func init() {
	filters.Register(filterName, func() (filters.Filter, error) {
		filename := filterName + ".json"
		config := new(Config)
		err := storage.LookupStoreByFilterName(filterName).UnmarshallJson(filename, config)
		if err != nil {
			glog.Fatalf("storage.ReadJsonConfig(%#v) failed: %s", filename, err)
		}
		return NewFilter(config)
	})
}

func NewFilter(config *Config) (filters.Filter, error) {
	servers := make([]*Server, 0)
	for _, fs := range config.Servers {
		u, err := url.Parse(fs.URL)
		if err != nil {
			return nil, err
		}

		transport := &http2.Transport{}

		fs := &Server{
			URL:       u,
			Username:  fs.Username,
			Password:  fs.Password,
			SSLVerify: fs.SSLVerify,
			Transport: transport,
		}

		servers = append(servers, fs)
	}

	return &Filter{
		Servers: servers,
	}, nil
}

func (p *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	i := 0
	if helpers.IsStaticRequest(req) {
		i = rand.Intn(len(f.Servers))
	}

	server := f.Servers[i]

	// if req.Method == "CONNECT" {
	// 	rconn, err := server.Transport.Connect(req)
	// 	if err != nil {
	// 		return ctx, nil, err
	// 	}
	// 	defer rconn.Close()

	// 	rw := ctx.GetResponseWriter()

	// 	hijacker, ok := rw.(http.Hijacker)
	// 	if !ok {
	// 		return ctx, nil, fmt.Errorf("http.ResponseWriter(%#v) does not implments http.Hijacker", rw)
	// 	}

	// 	flusher, ok := rw.(http.Flusher)
	// 	if !ok {
	// 		return ctx, nil, fmt.Errorf("http.ResponseWriter(%#v) does not implments http.Flusher", rw)
	// 	}

	// 	rw.WriteHeader(http.StatusOK)
	// 	flusher.Flush()

	// 	lconn, _, err := hijacker.Hijack()
	// 	if err != nil {
	// 		return ctx, nil, fmt.Errorf("%#v.Hijack() error: %v", hijacker, err)
	// 	}
	// 	defer lconn.Close()

	// 	go helpers.IOCopy(rconn, lconn)
	// 	helpers.IOCopy(lconn, rconn)

	// 	ctx.Hijack(true)
	// 	return ctx, nil, nil
	// }
	resp, err := server.RoundTrip(req)
	if err != nil {
		return ctx, nil, err
	} else {
		glog.V(2).Infof("%s \"VPS %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}
	return ctx, resp, err
}
