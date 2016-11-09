package autoproxy

import (
	"context"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/wangtuanjie/ip17mon"

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
		Enabled      bool
		DataFile     string
		DNSServer    string
		DNSCacheSize int
		Rules        map[string]string
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
	MobileConfig struct {
		Enabled bool
	}
	IPHTML struct {
		Enabled   bool
		WhiteList []string
	}
	BlackList struct {
		Enabled   bool
		SiteRules []string
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
	IndexFiles           []string
	IndexFilesSet        map[string]struct{}
	ProxyPacCache        lrucache.Cache
	GFWListEnabled       bool
	GFWList              *GFWList
	MobileConfigEnabled  bool
	IPHTMLEnabled        bool
	IPHTMLWhiteList      *helpers.HostMatcher
	BlackListEnabled     bool
	BlackListSiteMatcher *helpers.HostMatcher
	SiteFiltersEnabled   bool
	SiteFiltersRules     *helpers.HostMatcher
	RegionFiltersEnabled bool
	RegionFiltersRules   map[string]filters.RoundTripFilter
	RegionResolver       *helpers.Resolver
	RegionLocator        *ip17mon.Locator
	RegionFilterCache    lrucache.Cache
	Transport            *http.Transport
}

func init() {
	filename := filterName + ".json"
	config := new(Config)
	err := storage.LookupStoreByFilterName(filterName).UnmarshallJson(filename, config)
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
	mime.AddExtensionType(".mobileconfig", "application/x-apple-aspen-config")
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

	store := storage.LookupStoreByFilterName(filterName)
	if err != nil {
		return nil, err
	}

	if _, err := store.Head(gfwlist.Filename); err != nil {
		return nil, err
	}

	transport := &http.Transport{}

	f := &Filter{
		Config:               *config,
		Store:                store,
		IndexFilesEnabled:    config.IndexFiles.Enabled,
		IndexFiles:           config.IndexFiles.Files,
		IndexFilesSet:        make(map[string]struct{}),
		ProxyPacCache:        lrucache.NewLRUCache(32),
		GFWListEnabled:       config.GFWList.Enabled,
		MobileConfigEnabled:  config.MobileConfig.Enabled,
		IPHTMLEnabled:        config.IPHTML.Enabled,
		BlackListEnabled:     config.BlackList.Enabled,
		BlackListSiteMatcher: helpers.NewHostMatcher(config.BlackList.SiteRules),
		GFWList:              &gfwlist,
		Transport:            transport,
		SiteFiltersEnabled:   config.SiteFilters.Enabled,
		RegionFiltersEnabled: config.RegionFilters.Enabled,
	}

	for _, name := range f.IndexFiles {
		f.IndexFilesSet[name] = struct{}{}
	}

	if f.IPHTMLEnabled {
		f.IPHTMLWhiteList = helpers.NewHostMatcher(config.IPHTML.WhiteList)
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
		resp, err := store.Get(f.Config.RegionFilters.DataFile)
		if err != nil {
			glog.Fatalf("AUTOPROXY: store.Get(%#v) error: %v", f.Config.RegionFilters.DataFile, err)
		}
		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Fatalf("AUTOPROXY: ioutil.ReadAll(%#v) error: %v", resp.Body, err)
		}

		f.RegionLocator = ip17mon.NewLocatorWithData(data)

		f.RegionResolver = &helpers.Resolver{}
		if config.RegionFilters.DNSServer != "" {
			f.RegionResolver.DNSServer = net.ParseIP(config.RegionFilters.DNSServer)
			if f.RegionResolver.DNSServer == nil {
				glog.Fatalf("AUTOPROXY: net.ParseIP(%+v) failed", config.RegionFilters.DNSServer)
			}
		}

		fm := make(map[string]filters.RoundTripFilter)
		for region, name := range config.RegionFilters.Rules {
			if name == "" {
				continue
			}
			f, err := filters.GetFilter(name)
			if err != nil {
				glog.Fatalf("AUTOPROXY: filters.GetFilter(%#v) for %#v error: %v", name, region, err)
			}
			f1, ok := f.(filters.RoundTripFilter)
			if !ok {
				glog.Fatalf("AUTOPROXY: filters.GetFilter(%#v) return %T, not a RoundTripFilter", name, f)
			}
			fm[strings.ToLower(region)] = f1
		}
		f.RegionFiltersRules = fm

		f.RegionFilterCache = lrucache.NewLRUCache(uint(f.Config.RegionFilters.DNSCacheSize))
	}

	if f.GFWListEnabled {
		go onceUpdater.Do(f.pacUpdater)
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) FindCountryByIP(ip string) (string, error) {
	li, err := f.RegionLocator.Find(ip)
	if err != nil {
		return "", err
	}

	//FIXME: Who should be ashamed?
	switch li.Country {
	case "中国":
		switch li.Region {
		case "台湾", "香港":
			li.Country = li.Region
		}
	}

	return li.Country, nil
}

func (f *Filter) Request(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	if strings.HasPrefix(req.RequestURI, "/") {
		return ctx, req, nil
	}

	host := helpers.GetHostName(req)

	if f.BlackListEnabled {
		if f.BlackListSiteMatcher.Match(host) {
			glog.V(2).Infof("%s \"AUTOPROXY BlackList %s %s %s\"", req.RemoteAddr, req.Method, req.URL.String(), req.Proto)
			return ctx, filters.DummyRequest, nil
		}
	}

	if f.SiteFiltersEnabled {
		if f1, ok := f.SiteFiltersRules.Lookup(host); ok {
			glog.V(2).Infof("%s \"AUTOPROXY SiteFilters %s %s %s\" with %T", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, f1)
			filters.SetRoundTripFilter(ctx, f1.(filters.RoundTripFilter))
			return ctx, req, nil
		}
	}

	if f.RegionFiltersEnabled {
		if f1, ok := f.RegionFilterCache.Get(host); ok {
			if f1 != nil {
				filters.SetRoundTripFilter(ctx, f1.(filters.RoundTripFilter))
			}
		} else if ips, err := f.RegionResolver.LookupIP(host); err == nil && len(ips) > 0 {
			ip := ips[0]

			if ip.IsLoopback() && !(strings.Contains(host, ".local") || strings.Contains(host, "localhost.")) {
				glog.V(2).Infof("%s \"AUTOPROXY RegionFilters BYPASS Loopback %s %s %s\" with nil", req.RemoteAddr, req.Method, req.URL.String(), req.Proto)
				f.RegionFilterCache.Set(host, nil, time.Now().Add(time.Hour))
			} else if ip.To4() == nil {
				if f1, ok := f.RegionFiltersRules["ipv6"]; ok {
					glog.V(2).Infof("%s \"AUTOPROXY RegionFilters IPv6 %s %s %s\" with %T", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, f1)
					f.RegionFilterCache.Set(host, f1, time.Now().Add(time.Hour))
					filters.SetRoundTripFilter(ctx, f1)
				}
			} else if country, err := f.FindCountryByIP(ip.String()); err == nil {
				if f1, ok := f.RegionFiltersRules[country]; ok {
					glog.V(2).Infof("%s \"AUTOPROXY RegionFilters %s %s %s %s\" with %T", req.RemoteAddr, country, req.Method, req.URL.String(), req.Proto, f1)
					f.RegionFilterCache.Set(host, f1, time.Now().Add(time.Hour))
					filters.SetRoundTripFilter(ctx, f1)
				} else if f1, ok := f.RegionFiltersRules["default"]; ok {
					glog.V(2).Infof("%s \"AUTOPROXY RegionFilters Default %s %s %s\" with %T", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, f1)
					f.RegionFilterCache.Set(host, f1, time.Now().Add(time.Hour))
					filters.SetRoundTripFilter(ctx, f1)
				} else {
					f.RegionFilterCache.Set(host, nil, time.Now().Add(time.Hour))
				}
			}
		}
	}

	return ctx, req, nil
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	if f := filters.GetRoundTripFilter(ctx); f != nil {
		return f.RoundTrip(ctx, req)
	}

	switch {
	case f.SiteFiltersEnabled && req.URL.Scheme == "https":
		if f1, ok := f.SiteFiltersRules.Lookup(helpers.GetHostName(req)); ok && f1 != nil {
			return f1.(filters.RoundTripFilter).RoundTrip(ctx, req)
		}
	case f.RegionFiltersEnabled && req.URL.Scheme == "https":
		if f1, ok := f.RegionFilterCache.Get(helpers.GetHostName(req)); ok && f1 != nil {
			return f1.(filters.RoundTripFilter).RoundTrip(ctx, req)
		}
	}

	if req.URL.Host == "" && req.RequestURI[0] == '/' && f.IndexFilesEnabled {
		if _, ok := f.IndexFilesSet[req.URL.Path[1:]]; ok || req.URL.Path == "/" {
			switch {
			case f.GFWListEnabled && strings.HasSuffix(req.URL.Path, ".pac"):
				glog.V(2).Infof("%s \"AUTOPROXY ProxyPac %s %s %s\" - -", req.RemoteAddr, req.Method, req.RequestURI, req.Proto)
				return f.ProxyPacRoundTrip(ctx, req)
			case f.MobileConfigEnabled && strings.HasSuffix(req.URL.Path, ".mobileconfig"):
				glog.V(2).Infof("%s \"AUTOPROXY ProxyMobileConfig %s %s %s\" - -", req.RemoteAddr, req.Method, req.RequestURI, req.Proto)
				return f.ProxyMobileConfigRoundTrip(ctx, req)
			case f.IPHTMLEnabled && req.URL.Path == "/ip.html":
				glog.V(2).Infof("%s \"AUTOPROXY IPHTML %s %s %s\" - -", req.RemoteAddr, req.Method, req.RequestURI, req.Proto)
				return f.IPHTMLRoundTrip(ctx, req)
			default:
				glog.V(2).Infof("%s \"AUTOPROXY IndexFiles %s %s %s\" - -", req.RemoteAddr, req.Method, req.RequestURI, req.Proto)
				return f.IndexFilesRoundTrip(ctx, req)
			}
		}
	}

	return ctx, nil, nil
}
