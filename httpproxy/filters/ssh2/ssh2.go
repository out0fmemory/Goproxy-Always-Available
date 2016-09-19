package ssh2

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"golang.org/x/crypto/ssh"

	"../../filters"
	"../../helpers"
	"../../storage"
)

const (
	filterName string = "ssh2"
)

type Config struct {
	Servers []struct {
		Addr     string
		Username string
		Password string
	}
	Transport struct {
		DisableKeepAlives   bool
		DisableCompression  bool
		TLSHandshakeTimeout int
		MaxIdleConnsPerHost int
	}
}

type Filter struct {
	Config
	Transport      *http.Transport
	SSHClientCache lrucache.Cache
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
}

func NewFilter(config *Config) (filters.Filter, error) {
	ss := &Servers{
		servers:    make([]Server, 0),
		sshClients: lrucache.NewLRUCache(uint(len(config.Servers))),
	}

	for _, s := range config.Servers {
		server := Server{
			Address: s.Addr,
			ClientConfig: &ssh.ClientConfig{
				User: s.Username,
				Auth: []ssh.AuthMethod{
					ssh.Password(s.Password),
				},
			},
		}

		ss.servers = append(ss.servers, server)
	}

	tr := &http.Transport{
		Dial: ss.Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		},
		TLSHandshakeTimeout: time.Duration(config.Transport.TLSHandshakeTimeout) * time.Second,
		MaxIdleConnsPerHost: config.Transport.MaxIdleConnsPerHost,
	}

	return &Filter{
		Config:    *config,
		Transport: tr,
	}, nil
}

func (p *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	switch req.Method {
	case "CONNECT":
		glog.V(2).Infof("%s \"SSH2 %s %s %s\" - -", req.RemoteAddr, req.Method, req.Host, req.Proto)
		rconn, err := f.Transport.Dial("tcp", req.Host)
		if err != nil {
			return ctx, nil, err
		}

		rw := filters.GetResponseWriter(ctx)

		hijacker, ok := rw.(http.Hijacker)
		if !ok {
			return ctx, nil, fmt.Errorf("http.ResponseWriter(%#v) does not implments http.Hijacker", rw)
		}

		flusher, ok := rw.(http.Flusher)
		if !ok {
			return ctx, nil, fmt.Errorf("http.ResponseWriter(%#v) does not implments http.Flusher", rw)
		}

		rw.WriteHeader(http.StatusOK)
		flusher.Flush()

		lconn, _, err := hijacker.Hijack()
		if err != nil {
			return ctx, nil, fmt.Errorf("%#v.Hijack() error: %v", hijacker, err)
		}
		defer lconn.Close()

		go helpers.IOCopy(rconn, lconn)
		helpers.IOCopy(lconn, rconn)

		return ctx, filters.DummyResponse, nil
	default:
		helpers.FixRequestURL(req)
		resp, err := f.Transport.RoundTrip(req)

		if err != nil {
			return ctx, nil, err
		}

		if req.RemoteAddr != "" {
			glog.V(2).Infof("%s \"SSH2 %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
		}

		return ctx, resp, err
	}
}
