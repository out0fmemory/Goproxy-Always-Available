package autoproxy

import (
	"context"
	"mime"
	"net/http"
	"net/url"
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

	transport := &http.Transport{}

	f := &Filter{
		Config:               *config,
		Store:                store,
		IndexFilesEnabled:    config.IndexFiles.Enabled,
		IndexFiles:           make(map[string]struct{}),
		ProxyPacCache:        lrucache.NewLRUCache(32),
		GFWListEnabled:       config.GFWList.Enabled,
		GFWList:              &gfwlist,
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
		go onceUpdater.Do(f.pacUpdater)
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
