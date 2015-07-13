package auth

import (
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
	"github.com/phuslu/goproxy/httpproxy/filters"
)

const (
	filterName   string = "auth"
	authHeader   string = filterName + "/header"
	bypassHeader string = filterName + "/bypass"
)

type Filter struct {
	ByPassHeaders lrucache.Cache
	Basic         map[string]string
	WhiteList     map[string]struct{}
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
	f := &Filter{
		ByPassHeaders: lrucache.NewMultiLRUCache(4, uint(config.CacheSize)),
		Basic:         make(map[string]string),
		WhiteList:     make(map[string]struct{}),
	}

	for _, v := range config.Basic {
		f.Basic[v.Username] = v.Password
	}

	for _, v := range config.WhiteList {
		f.WhiteList[v] = struct{}{}
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) Request(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Request, error) {
	if auth := req.Header.Get("Proxy-Authorization"); auth != "" {
		req.Header.Del("Proxy-Authorization")
		ctx.SetString(authHeader, auth)
	}
	return ctx, req, nil
}

func (f *Filter) RoundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {

	if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		if _, ok := f.WhiteList[ip]; ok {
			return ctx, nil, nil
		}
	}

	if auth, err := ctx.GetString(authHeader); err == nil {
		if _, ok := f.ByPassHeaders.Get(auth); ok {
			glog.V(3).Infof("auth filter hit bypass cache %#v", auth)
			return ctx, nil, nil
		}
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "Basic":
				if userpass, err := base64.StdEncoding.DecodeString(parts[1]); err == nil {
					parts := strings.Split(string(userpass), ":")
					user := parts[0]
					pass := parts[1]
					pass1, ok := f.Basic[user]
					if ok && pass == pass1 {
						f.ByPassHeaders.Set(auth, struct{}{}, time.Now().Add(time.Hour))
						return ctx, nil, nil
					}
				}
			default:
				glog.Errorf("Unrecognized auth type: %#v", parts[0])
				break
			}
		}
	}

	glog.V(1).Infof("UnAuthenticated URL %v from %#v", req.URL.String(), req.RemoteAddr)

	noAuthResponse := &http.Response{
		Status:        "407 Proxy authentication required",
		StatusCode:    407,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: -1,
	}

	return ctx, noAuthResponse, nil
}
