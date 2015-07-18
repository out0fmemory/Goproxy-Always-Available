package stripssl

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
	"github.com/phuslu/goproxy/httpproxy"
	"github.com/phuslu/goproxy/httpproxy/filters"
)

const (
	filterName       string = "stripssl"
	ENV_OPENSSL_CONF string = "OPENSSL_CONF"
)

type Filter struct {
	CA             *RootCA
	CAExpires      time.Duration
	TLSConfigCache lrucache.Cache
	SiteLists1     map[string]struct{}
	SiteLists2     []string
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
	var ca *RootCA

	ca, err = NewRootCA(config.RootCA.Name, time.Duration(config.RootCA.Duration)*time.Second, config.RootCA.RsaBits, config.RootCA.Dirname)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(config.RootCA.Dirname); os.IsNotExist(err) {
		if err = os.Mkdir(config.RootCA.Dirname, 0755); err != nil {
			return nil, err
		}
	}

	f := &Filter{
		CA:             ca,
		CAExpires:      time.Duration(config.RootCA.Duration) * time.Second,
		TLSConfigCache: lrucache.NewMultiLRUCache(4, 4096),
		SiteLists1:     make(map[string]struct{}),
		SiteLists2:     make([]string, 0),
	}

	for _, site := range config.Sites {
		if !strings.Contains(site, "*") {
			f.SiteLists1[site] = struct{}{}
		} else {
			f.SiteLists2 = append(f.SiteLists2, site)
		}
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) Match(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	if _, ok := f.SiteLists1[host]; ok {
		return true
	}

	for _, pattern := range f.SiteLists2 {
		if matched, _ := path.Match(pattern, host); matched {
			return true
		}
	}

	return false
}

func (f *Filter) Request(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Request, error) {
	if req.Method != "CONNECT" || !f.Match(req.Host) {
		return ctx, req, nil
	}

	hijacker, ok := ctx.GetResponseWriter().(http.Hijacker)
	if !ok {
		return ctx, nil, fmt.Errorf("%#v does not implments Hijacker", ctx.GetResponseWriter())
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		return ctx, nil, fmt.Errorf("http.ResponseWriter Hijack failed: %s", err)
	}

	_, err = io.WriteString(conn, "HTTP/1.1 200 OK\r\n\r\n")
	if err != nil {
		conn.Close()
		return ctx, nil, err
	}

	glog.Infof("%s \"STRIP %s %s %s\" - -", req.RemoteAddr, req.Method, req.Host, req.Proto)

	config, err := f.issue(req.Host)
	if err != nil {
		conn.Close()
		return ctx, nil, err
	}

	tlsConn := tls.Server(conn, config)

	if err := tlsConn.Handshake(); err != nil {
		glog.V(2).Infof("%s %T.Handshake() error: %#v", req.RemoteAddr, tlsConn, err)
		conn.Close()
		return ctx, nil, err
	}

	if ln1, ok := ctx.GetListener().(httpproxy.Listener); ok {
		ln1.Add(tlsConn)
		ctx.SetHijacked(true)
		return ctx, nil, nil
	}

	loConn, err := net.Dial("tcp", ctx.GetListener().Addr().String())
	if err != nil {
		return ctx, nil, err
	}

	go httpproxy.IoCopy(loConn, tlsConn)
	go httpproxy.IoCopy(tlsConn, loConn)

	ctx.SetHijacked(true)
	return ctx, nil, nil
}

func (f *Filter) issue(host string) (_ *tls.Config, err error) {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	name, err := GetCommonName(host)
	if err != nil {
		return nil, err
	}

	var config interface{}
	var ok bool
	if config, ok = f.TLSConfigCache.Get(name); !ok {
		glog.Infof("generate certificate for %s...", name)
		cert, err := f.CA.Issue(name, f.CAExpires, f.CA.RsaBits())
		if err != nil {
			return nil, err
		}
		config = &tls.Config{
			Certificates: []tls.Certificate{*cert},
			ClientAuth:   tls.VerifyClientCertIfGiven,
		}
		f.TLSConfigCache.Set(name, config, time.Now().Add(f.CAExpires))
	}
	return config.(*tls.Config), nil
}
