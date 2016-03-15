package gae

import (
	// "fmt"
	"math/rand"
	"mime"
	"net/http"
	// "net/url"
	"path"
	"strings"
	"sync"

	"github.com/golang/glog"

	"../../../httpproxy"
	"../../../storage"
	"../../filters"
)

const (
	filterName string = "gae2"
)

type Config struct {
	AppIDs      []string
	Scheme      string
	Domain      string
	Path        string
	Password    string
	SSLVerify   bool
	Sites       []string
	Site2Alias  map[string]string
	HostMap     map[string][]string
	DNSServers  []string
	IPBlackList []string
	Transport   struct {
		Dialer struct {
			Window    int
			Timeout   float32
			KeepAlive int
			DualStack bool
		}
		DisableKeepAlives   bool
		DisableCompression  bool
		TLSHandshakeTimeout int
		MaxIdleConnsPerHost int
	}
}

type Filter struct {
	Transports   []http.RoundTripper
	muTransports sync.Mutex
	Sites        *httpproxy.HostMatcher
}

func init() {
	filename := filterName + ".json"
	config := new(Config)
	err := storage.ReadJsonConfig(filters.LookupConfigStoreURI(filterName), filename, config)
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

func NewFilter(config *Config) (filters.Filter, error) {
	// tr := make([]*FetchServer, 0)
	// for _, appid := range config.AppIds {
	// 	var rawurl string
	// 	switch strings.Count(appid, ".") {
	// 	case 0, 1:
	// 		rawurl = fmt.Sprintf("%s://%s.%s%s", config.Scheme, appid, config.Domain, config.Path)
	// 	default:
	// 		rawurl = fmt.Sprintf("%s://%s.%s%s", config.Scheme, appid, config.Path)
	// 	}
	// 	u, err := url.Parse(rawurl)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	fs := &FetchServer{
	// 		URL:       u,
	// 		Password:  config.Password,
	// 		SSLVerify: config.SSLVerify,
	// 	}

	// 	fetchServers = append(fetchServers, fs)
	// }

	return &Filter{
		Transports: nil,
		Sites:      httpproxy.NewHostMatcher(config.Sites),
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
		i += rand.Intn(len(f.Transports) - i)
	}

	tr := f.Transports[i]

	resp, err := tr.RoundTrip(req)
	if err != nil || resp == nil {
		glog.Errorf("%s \"GAE %s %s %s\" %#v %v", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp, err)
		return ctx, resp, err
	} else {
		glog.Infof("%s \"GAE %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}

	switch resp.StatusCode {
	case 503:
		if len(f.Transports) == 1 {
			break
		}
		glog.Warningf("%s over qouta, switch to next appid.", tr)
		f.muTransports.Lock()
		if tr == f.Transports[0] {
			for i := 0; i < len(f.Transports)-1; i++ {
				f.Transports[i] = f.Transports[i+1]
			}
			f.Transports[len(f.Transports)-1] = tr
		}
		f.muTransports.Unlock()
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

	return ctx, resp, err
}
