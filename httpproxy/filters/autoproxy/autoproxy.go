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
	IndexFilesEnabled    bool
	IndexFiles           map[string]struct{}
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

	mime.AddExtensionType(".crt", "application/x-x509-user-cert")
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

	transport := &http.Transport{}

	f := &Filter{
		Config:               *config,
		Store:                store,
		IndexFilesEnabled:    config.IndexFiles.Enabled,
		IndexFiles:           make(map[string]struct{}),
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
	<head>
		<meta charset="UTF-8">
		<link rel="icon" type="image/x-icon" href="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7" />
	</head>
	<body>
		{{ range $key, $value := .IndexFiles }}
		   <li><a href="{{ $key }}">{{ $key }}</a></li>
		{{ end }}
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
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
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
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Content-Type": []string{contentType},
		},
		Request:       req,
		Close:         true,
		ContentLength: int64(len(data)),
		Body:          ioutil.NopCloser(bytes.NewReader(data)),
	}, nil
}

func (f *Filter) ProxyPacRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	filename := req.URL.Path[1:]

	data := ""

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
        return 'PROXY %s';
    }
    return 'DIRECT';
}
`, req.Host)
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
			host, _, err := net.SplitHostPort(req.Host)
			if err != nil {
				host = req.Host
			}
			for _, localaddr := range []string{"127.0.0.1:", "[::1]:", "localhost:"} {
				s = strings.Replace(s, localaddr, net.JoinHostPort(host, ""), -1)
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
	glog.V(2).Infof("start updater for %#v", f.GFWList.URL.String())

	ticker := time.Tick(10 * time.Minute)

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

			if time.Now().Before(modTime.Add(f.GFWList.Duration)) {
				continue
			}
		}

		glog.Infof("Downloading %#v", f.GFWList.URL.String())

		req, err := http.NewRequest("GET", f.GFWList.URL.String(), nil)
		if err != nil {
			glog.Warningf("NewRequest(%#v) error: %v", f.GFWList.URL.String(), err)
			continue
		}
		ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
		req = req.WithContext(ctx)

		resp, err := f.Transport.RoundTrip(req)
		cancel()
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
