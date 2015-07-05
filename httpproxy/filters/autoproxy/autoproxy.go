package autoproxy

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"

	"../../../httpproxy"
	"../../../storage"
	"../../filters"

	"github.com/golang/glog"
)

const (
	filterName      string = "autoproxy"
	placeholderPath string = "/proxy.pac"
)

type GFWList struct {
	URL      *url.URL
	Filename string
	Encoding string
}

type Filter struct {
	Store         storage.Store
	Sites         *httpproxy.HostMatcher
	GFWList       *GFWList
	AutoProxy2Pac *AutoProxy2Pac
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

func NewFilter(config *Config) (_ filters.Filter, err error) {
	var gfwlist GFWList

	gfwlist.Encoding = config.GFWList.Encoding
	gfwlist.Filename = config.GFWList.File
	gfwlist.URL, err = url.Parse(config.GFWList.URL)
	if err != nil {
		return nil, err
	}

	store, err := storage.OpenURI(filters.LookupConfigStoreURI(filterName))
	if err != nil {
		return nil, err
	}

	if _, err := store.HeadObject(gfwlist.Filename); err != nil {
		return nil, err
	}

	autoproxy2pac := &AutoProxy2Pac{
		Sites: config.Sites,
	}

	object, err := store.GetObject(gfwlist.Filename, -1, -1)
	if err != nil {
		return nil, err
	}

	rc := object.Body()
	defer rc.Close()

	err = autoproxy2pac.Read(rc)
	if err != nil {
		return nil, err
	}

	f := &Filter{
		Store:         store,
		Sites:         httpproxy.NewHostMatcher(config.Sites),
		GFWList:       &gfwlist,
		AutoProxy2Pac: autoproxy2pac,
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {

	if req.Method != "GET" {
		return ctx, nil, nil
	}

	if req.RequestURI != placeholderPath {
		return ctx, nil, nil
	}

	data := f.AutoProxy2Pac.GeneratePac(req)

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: int64(len(data)),
		Body:          ioutil.NopCloser(bytes.NewReader([]byte(data))),
	}

	glog.Infof("%s \"AUTOPROXY %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.RequestURI, req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))

	return ctx, resp, nil
}
