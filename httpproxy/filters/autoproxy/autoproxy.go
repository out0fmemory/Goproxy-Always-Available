package autoproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	IndexFiles struct {
		Enabled bool
		Files   []string
	}
	GFWList struct {
		Enabled  bool
		URL      string
		File     string
		Encoding string
		Expiry   int
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
	Expiry   time.Duration
	Duration time.Duration
}

type Filter struct {
	Config
	Store                storage.Store
	IndexFilesEnabled    bool
	IndexFiles           map[string]struct{}
	ProxyPacCache        lrucache.Cache
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

	mime.AddExtensionType(".crt", "application/x-x509-ca-cert")
}

func NewFilter(config *Config) (_ filters.Filter, err error) {
	var gfwlist GFWList

	gfwlist.Encoding = config.GFWList.Encoding
	gfwlist.Filename = config.GFWList.File
	gfwlist.Expiry = time.Duration(config.GFWList.Expiry) * time.Second
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

	transport := &http.Transport{}

	f := &Filter{
		Config:               *config,
		Store:                store,
		IndexFilesEnabled:    config.IndexFiles.Enabled,
		IndexFiles:           make(map[string]struct{}),
		ProxyPacCache:        lrucache.NewLRUCache(32),
		GFWListEnabled:       config.GFWList.Enabled,
		GFWList:              &gfwlist,
		AutoProxy2Pac:        autoproxy2pac,
		Transport:            transport,
		SiteFiltersEnabled:   config.SiteFilters.Enabled,
		RegionFiltersEnabled: config.RegionFilters.Enabled,
	}

	for _, name := range config.IndexFiles.Files {
		f.IndexFiles[name] = struct{}{}
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

	if req.URL.Host == "" && req.RequestURI[0] == '/' && f.IndexFilesEnabled {
		if _, ok := f.IndexFiles[req.URL.Path[1:]]; ok || req.URL.Path == "/" {
			if f.GFWListEnabled && strings.HasSuffix(req.URL.Path, ".pac") {
				glog.V(2).Infof("%s \"AUTOPROXY ProxyPac %s %s %s\" - -", req.RemoteAddr, req.Method, req.RequestURI, req.Proto)
				return f.ProxyPacRoundTrip(ctx, req)
			} else {
				glog.V(2).Infof("%s \"AUTOPROXY IndexFiles %s %s %s\" - -", req.RemoteAddr, req.Method, req.RequestURI, req.Proto)
				return f.IndexFilesRoundTrip(ctx, req)
			}
		}
	}

	return ctx, nil, nil
}

func (f *Filter) IndexFilesRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	filename := req.URL.Path[1:]

	if filename == "" {
		const tpl = `<!DOCTYPE html>
<html>
<meta charset="UTF-8">
<head><title>Index of /</title>
</head>
<body>
<h1>Index of /</h1>
<pre>Name</pre><hr/>
<pre>{{ range $key, $value := .IndexFiles }}
📄 <a href="{{ $key }}">{{ $key }}</a>{{ end }}</pre>
<hr/><address style="font-size:small;">GoProxy Server</address>
</body>
</html>`
		t, err := template.New("index").Parse(tpl)
		if err != nil {
			return ctx, nil, err
		}

		b := new(bytes.Buffer)
		err = t.Execute(b, struct{ IndexFiles map[string]struct{} }{f.IndexFiles})
		if err != nil {
			return ctx, nil, err
		}

		return ctx, &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"text/html"},
			},
			Request:       req,
			Close:         true,
			ContentLength: int64(b.Len()),
			Body:          ioutil.NopCloser(b),
		}, nil
	}

	obj, err := f.Store.GetObject(filename, -1, -1)
	if err != nil {
		return ctx, nil, err
	}

	body := obj.Body()
	defer body.Close()

	data, err := ioutil.ReadAll(body)
	if err != nil {
		return ctx, nil, err
	}

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	return ctx, &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{contentType},
		},
		Request:       req,
		Close:         true,
		ContentLength: int64(len(data)),
		Body:          ioutil.NopCloser(bytes.NewReader(data)),
	}, nil
}

func fixProxyPac(s string, req *http.Request) string {
	ports := make([]string, 0)
	for _, addr := range []string{req.Host, filters.GetListener(req.Context()).Addr().String()} {
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			port = "80"
		}
		ports = append(ports, port)
	}

	r := regexp.MustCompile(`PROXY (127.0.0.1|\[::1\]|localhost):(` + strings.Join(ports, "|") + `)`)
	return r.ReplaceAllString(s, "PROXY "+req.Host)
}

func (f *Filter) ProxyPacRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	_, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		port = "80"
	}

	if v, ok := f.ProxyPacCache.Get(req.RequestURI); ok {
		if s, ok := v.(string); ok {
			s = fixProxyPac(s, req)
			return ctx, &http.Response{
				StatusCode:    http.StatusOK,
				Header:        http.Header{},
				Request:       req,
				Close:         true,
				ContentLength: int64(len(s)),
				Body:          ioutil.NopCloser(strings.NewReader(s)),
			}, nil
		}
	}

	filename := req.URL.Path[1:]

	buf := new(bytes.Buffer)

	obj, err := f.Store.GetObject(filename, -1, -1)
	switch {
	case os.IsNotExist(err):
		glog.V(2).Infof("AUTOPROXY ProxyPac: generate %#v", filename)
		s := fmt.Sprintf(`// User-defined FindProxyForURL
function FindProxyForURL(url, host) {
    if (shExpMatch(host, '*.google*.*') ||
       dnsDomainIs(host, '.ggpht.com') ||
       dnsDomainIs(host, '.gstatic.com') ||
       host == 'goo.gl') {
        return 'PROXY localhost:%s';
    }
    return 'DIRECT';
}
`, port)
		f.Store.PutObject(filename, http.Header{}, ioutil.NopCloser(bytes.NewBufferString(s)))
	case err != nil:
		return ctx, nil, err
	case obj != nil:
		if body := obj.Body(); body != nil {
			body.Close()
		}
	}

	if obj, err := f.Store.GetObject(filename, -1, -1); err == nil {
		body := obj.Body()
		defer body.Close()
		if b, err := ioutil.ReadAll(body); err == nil {
			s := strings.Replace(string(b), "function FindProxyForURL(", "function MyFindProxyForURL(", 1)
			io.WriteString(buf, s)
		}
	}

	if f.GFWListEnabled {
		io.WriteString(buf, f.AutoProxy2Pac.GeneratePac("localhost:"+port))
	} else {
		io.WriteString(buf, `
function FindProxyForURL(url, host) {
    if (isPlainHostName(host) ||
        host.indexOf('127.') == 0 ||
        host.indexOf('192.168.') == 0 ||
        host.indexOf('10.') == 0 ||
        shExpMatch(host, 'localhost.*')) {
        return 'DIRECT';
    }

    return MyFindProxyForURL(url, host);
}`)
	}

	s := buf.String()
	f.ProxyPacCache.Set(req.RequestURI, s, time.Now().Add(15*time.Minute))

	s = fixProxyPac(s, req)
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: int64(len(s)),
		Body:          ioutil.NopCloser(strings.NewReader(s)),
	}

	return ctx, resp, nil
}

func (f *Filter) updater() {
	glog.V(2).Infof("start updater for %+v, expiry=%s, duration=%s", f.GFWList.URL.String(), f.GFWList.Expiry, f.GFWList.Duration)

	ticker := time.Tick(f.GFWList.Duration)

	for {
		select {
		case <-ticker:
			glog.V(2).Infof("Begin auto gfwlist(%#v) update...", f.GFWList.URL.String())
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

			if time.Now().Sub(modTime) < f.GFWList.Expiry {
				continue
			}
		}

		glog.Infof("Downloading %#v", f.GFWList.URL.String())

		req, err := http.NewRequest(http.MethodGet, f.GFWList.URL.String(), nil)
		if err != nil {
			glog.Warningf("NewRequest(%#v) error: %v", f.GFWList.URL.String(), err)
			continue
		}

		resp, err := f.Transport.RoundTrip(req)
		if err != nil {
			glog.Warningf("%T.RoundTrip(%#v) error: %v", f.Transport, f.GFWList.URL.String(), err.Error())
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
			glog.Warningf("ioutil.ReadAll(%T) error: %v", r, err)
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

		f.ProxyPacCache.Clear()

		glog.Infof("Update %#v from %#v OK", f.GFWList.Filename, f.GFWList.URL.String())
		resp.Body.Close()
	}
}
