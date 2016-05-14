package autoproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"

	"../../filters"
	"../../helpers"
	"../../storage"
)

const (
	filterName string = "autoproxy"
)

type Config struct {
	SiteFilters struct {
		Enabled bool
		Rules   map[string]string
	}
	RegionFilters struct {
		Enabled bool
		Rules   map[string]string
	}
	MyProxyPac struct {
		Enabled bool
		File    string
	}
	GFWList struct {
		Enabled  bool
		URL      string
		File     string
		Encoding string
		Duration int
	}
}

var (
	onceUpdater sync.Once
)

type GFWList struct {
	URL      *url.URL
	Filename string
	Encoding string
	Duration time.Duration
}

type Filter struct {
	Config
	Store                storage.Store
	MyProxyPACEnabled    bool
	MyProxyPACFile       string
	GFWListEnabled       bool
	GFWList              *GFWList
	AutoProxy2Pac        *AutoProxy2Pac
	SiteFiltersEnabled   bool
	SiteFiltersRules     *helpers.HostMatcher
	RegionFiltersEnabled bool
	RegionFiltersRules   *helpers.HostMatcher
	RegionDNSCache       lrucache.Cache
	RegionIPCache        lrucache.Cache
	Transport            *http.Transport
	UpdateChan           chan struct{}
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

func NewFilter(config *Config) (_ filters.Filter, err error) {
	var gfwlist GFWList

	gfwlist.Encoding = config.GFWList.Encoding
	gfwlist.Filename = config.GFWList.File
	gfwlist.Duration = time.Duration(config.GFWList.Duration) * time.Second
	gfwlist.URL, err = url.Parse(config.GFWList.URL)
	if err != nil {
		return nil, err
	}

	store, err := storage.OpenURI(storage.LookupConfigStoreURI(filterName))
	if err != nil {
		return nil, err
	}

	if _, err := store.HeadObject(gfwlist.Filename); err != nil {
		return nil, err
	}

	autoproxy2pac := &AutoProxy2Pac{
		Sites: []string{"google.com"},
	}

	object, err := store.GetObject(gfwlist.Filename, -1, -1)
	if err != nil {
		return nil, err
	}

	rc := object.Body()
	defer rc.Close()

	var r io.Reader
	br := bufio.NewReader(rc)
	if data, err := br.Peek(20); err == nil {
		if bytes.HasPrefix(data, []byte("[AutoProxy ")) {
			r = br
		} else {
			r = base64.NewDecoder(base64.StdEncoding, br)
		}
	}

	err = autoproxy2pac.Read(r)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	f := &Filter{
		Config:               *config,
		Store:                store,
		MyProxyPACEnabled:    config.MyProxyPac.Enabled,
		MyProxyPACFile:       config.MyProxyPac.File,
		GFWListEnabled:       config.GFWList.Enabled,
		GFWList:              &gfwlist,
		AutoProxy2Pac:        autoproxy2pac,
		Transport:            transport,
		SiteFiltersEnabled:   config.SiteFilters.Enabled,
		RegionFiltersEnabled: config.RegionFilters.Enabled,
		UpdateChan:           make(chan struct{}),
	}

	if f.SiteFiltersEnabled {
		fm := make(map[string]interface{})
		for host, name := range config.SiteFilters.Rules {
			f, err := filters.GetFilter(name)
			if err != nil {
				glog.Fatalf("AUTOPROXY: filters.GetFilter(%#v) for %#v error: %v", name, host, err)
			}
			if _, ok := f.(filters.RoundTripFilter); !ok {
				glog.Fatalf("AUTOPROXY: filters.GetFilter(%#v) return %T, not a RoundTripFilter", name, f)
			}
			fm[host] = f
		}
		f.SiteFiltersRules = helpers.NewHostMatcherWithValue(fm)
	}

	if f.RegionFiltersEnabled {
		fm := make(map[string]interface{})
		for host, name := range config.SiteFilters.Rules {
			f, err := filters.GetFilter(name)
			if err != nil {
				glog.Fatalf("AUTOPROXY: filters.GetFilter(%#v) for %#v error: %v", name, host, err)
			}
			if _, ok := f.(filters.RoundTripFilter); !ok {
				glog.Fatalf("AUTOPROXY: filters.GetFilter(%#v) return %T, not a RoundTripFilter", name, f)
			}
			fm[host] = f
		}
		f.RegionFiltersRules = helpers.NewHostMatcherWithValue(fm)
	}

	if f.GFWListEnabled {
		go onceUpdater.Do(f.updater)
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	if f.SiteFiltersEnabled {
		if f1, ok := f.SiteFiltersRules.Lookup(req.Host); ok {
			glog.V(2).Infof("%s \"AUTOPROXY SiteFilters %s %s %s\" with %T", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, f1)
			return f1.(filters.RoundTripFilter).RoundTrip(ctx, req)
		}
	}

	if f.RegionFiltersEnabled {
		//TODO
	}

	if f.MyProxyPACEnabled {
		if req.RequestURI[1:len(req.RequestURI)] == f.MyProxyPACFile {
			glog.V(2).Infof("%s \"AUTOPROXY MyProxyPac %s %s %s\" - -", req.RemoteAddr, req.Method, req.RequestURI, req.Proto)
			return f.myProxyPACRoundTrip(ctx, req)
		}
	}

	return ctx, nil, nil
}

func (f *Filter) myProxyPACRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	if strings.Contains(req.URL.Query().Encode(), "flush") {
		f.UpdateChan <- struct{}{}
	}

	data := ""

	obj, err := f.Store.GetObject(f.MyProxyPACFile, -1, -1)
	if os.IsNotExist(err) {
		glog.V(2).Infof("AUTOPROXY MyProxyPac: generate %#v", f.MyProxyPACFile)

		s := fmt.Sprintf(`// User-defined FindProxyForURL
function FindProxyForURL(url, host) {
    if (shExpMatch(host, '*.google*.*') ||
       dnsDomainIs(host, '.ggpht.com') ||
       dnsDomainIs(host, '.gstatic.com') ||
       host == 'goo.gl') {
        return 'PROXY %s';
    }
    return 'DIRECT';
}
`, req.URL.Host)
		f.Store.PutObject(f.MyProxyPACFile, http.Header{}, ioutil.NopCloser(bytes.NewBufferString(s)))
	} else {
		if body := obj.Body(); body != nil {
			body.Close()
		}
	}

	if obj, err := f.Store.GetObject(f.MyProxyPACFile, -1, -1); err == nil {
		body := obj.Body()
		defer body.Close()
		if b, err := ioutil.ReadAll(body); err == nil {
			s := strings.Replace(string(b), "function FindProxyForURL(", "function MyFindProxyForURL(", 1)
			if _, port, err := net.SplitHostPort(req.URL.Host); err == nil {
				for _, localaddr := range []string{"127.0.0.1", "::1", "localhost"} {
					s = strings.Replace(s, net.JoinHostPort(localaddr, port), req.URL.Host, -1)
				}
			}
			data += s
		}
	}

	if f.GFWListEnabled {
		data += f.AutoProxy2Pac.GeneratePac(req)
	} else {
		data += `
function FindProxyForURL(url, host) {
    if (isPlainHostName(host) ||
        host.indexOf('127.') == 0 ||
        host.indexOf('192.168.') == 0 ||
        host.indexOf('10.') == 0 ||
        shExpMatch(host, 'localhost.*')) {
        return 'DIRECT';
    }

    return MyFindProxyForURL(url, host);
}`
	}

	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: int64(len(data)),
		Body:          ioutil.NopCloser(bytes.NewReader([]byte(data))),
	}

	return ctx, resp, nil
}

func (f *Filter) updater() {
	glog.V(2).Infof("start updater for %#v", f.GFWList)

	ticker := time.Tick(10 * time.Minute)

	for {
		needUpdate := false

		select {
		case <-f.UpdateChan:
			glog.V(2).Infof("Begin manual gfwlist(%#v) update...", f.GFWList.URL.String())
			needUpdate = true
		case <-ticker:
			break
		}

		if !needUpdate {
			h, err := f.Store.HeadObject(f.GFWList.Filename)
			if err != nil {
				glog.Warningf("stat gfwlist(%#v) err: %v", f.GFWList.Filename, err)
				continue
			}

			lm := h.Get("Last-Modified")
			if lm == "" {
				glog.Warningf("gfwlist(%#v) header(%#v) does not contains last-modified", f.GFWList.Filename, h)
				continue
			}

			modTime, err := time.Parse(f.Store.DateFormat(), lm)
			if err != nil {
				glog.Warningf("stat gfwlist(%#v) has parse %#v error: %v", f.GFWList.Filename, lm, err)
				continue
			}

			needUpdate = time.Now().After(modTime.Add(f.GFWList.Duration))
		}

		if needUpdate {
			req, err := http.NewRequest("GET", f.GFWList.URL.String(), nil)
			if err != nil {
				glog.Warningf("NewRequest(%#v) error: %v", f.GFWList.URL.String(), err)
				continue
			}

			glog.Infof("Downloading %#v", f.GFWList.URL.String())

			resp, err := f.Transport.RoundTrip(req)
			if err != nil {
				glog.Warningf("%T.RoundTrip(%#v) error: %v", f.Transport, f.GFWList.URL.String(), err)
				continue
			}

			var r io.Reader = resp.Body
			switch f.GFWList.Encoding {
			case "base64":
				r = base64.NewDecoder(base64.StdEncoding, r)
			default:
				break
			}

			data, err := ioutil.ReadAll(r)
			if err != nil {
				glog.Warningf("ReadAll(%#v) error: %v", r, err)
				resp.Body.Close()
				continue
			}

			err = f.Store.DeleteObject(f.GFWList.Filename)
			if err != nil {
				glog.Warningf("%T.DeleteObject(%#v) error: %v", f.Store, f.GFWList.Filename, err)
				continue
			}

			err = f.Store.PutObject(f.GFWList.Filename, http.Header{}, ioutil.NopCloser(bytes.NewReader(data)))
			if err != nil {
				glog.Warningf("%T.PutObject(%#v) error: %v", f.Store, f.GFWList.Filename, err)
				continue
			}

			glog.Infof("Update %#v from %#v OK", f.GFWList.Filename, f.GFWList.URL.String())
			resp.Body.Close()
		}
	}
}
