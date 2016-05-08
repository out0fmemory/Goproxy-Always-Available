package stripssl

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"

	"../../filters"
	"../../helpers"
	"../../storage"
)

const (
	filterName string = "stripssl"
)

type Config struct {
	RootCA struct {
		Filename string
		Dirname  string
		Name     string
		Duration int
		RsaBits  int
	}
	Ports []int
	Sites []string
}

type Filter struct {
	Config
	CA             *RootCA
	CAExpiry       time.Duration
	TLSConfigCache lrucache.Cache
	Ports          map[string]struct{}
	Sites          *helpers.HostMatcher
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
		Config:         *config,
		CA:             ca,
		CAExpiry:       time.Duration(config.RootCA.Duration) * time.Second,
		TLSConfigCache: lrucache.NewMultiLRUCache(4, 4096),
		Ports:          make(map[string]struct{}),
		Sites:          helpers.NewHostMatcher(config.Sites),
	}

	for _, port := range config.Ports {
		f.Ports[strconv.Itoa(port)] = struct{}{}
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) Request(ctx context.Context, req *http.Request) (context.Context, *http.Request, error) {
	if req.Method != http.MethodConnect {
		return ctx, req, nil
	}

	host, port, err := net.SplitHostPort(req.RequestURI)
	if err != nil {
		return ctx, req, nil
	}

	if !f.Sites.Match(host) {
		return ctx, req, nil
	}

	needStripSSL := true
	if _, ok := f.Ports[port]; !ok {
		needStripSSL = false
	}

	rw := filters.GetResponseWriter(ctx)
	hijacker, ok := rw.(http.Hijacker)
	if !ok {
		return ctx, nil, fmt.Errorf("%#v does not implments Hijacker", rw)
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

	glog.V(2).Infof("%s \"STRIP %s %s %s\" - -", req.RemoteAddr, req.Method, req.Host, req.Proto)

	var c net.Conn = conn
	if needStripSSL {
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

		c = tlsConn
	}

	if ln1, ok := filters.GetListener(ctx).(helpers.Listener); ok {
		ln1.Add(c)
		ctx = filters.PutHijacked(ctx, true)
		return ctx, nil, nil
	}

	loConn, err := net.Dial("tcp", filters.GetListener(ctx).Addr().String())
	if err != nil {
		return ctx, nil, err
	}

	go helpers.IoCopy(loConn, c)
	go helpers.IoCopy(c, loConn)

	ctx = filters.PutHijacked(ctx, true)
	return ctx, nil, nil
}

func (f *Filter) issue(host string) (_ *tls.Config, err error) {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	name := GetCommonName(host)

	var config interface{}
	var ok bool
	if config, ok = f.TLSConfigCache.Get(name); !ok {
		cert, err := f.CA.Issue(name, f.CAExpiry, f.CA.RsaBits())
		if err != nil {
			return nil, err
		}
		config = &tls.Config{
			Certificates: []tls.Certificate{*cert},
		}
		f.TLSConfigCache.Set(name, config, time.Now().Add(f.CAExpiry))
	}
	return config.(*tls.Config), nil
}
