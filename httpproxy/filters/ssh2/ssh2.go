package ssh2

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"golang.org/x/crypto/ssh"

	"../../filters"
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
	err := storage.LookupStoreByConfig(filterName).UnmarshallJson(filename, config)
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
	resp, err := f.Transport.RoundTrip(req)
	if err != nil {
		return ctx, nil, err
	} else {
		glog.V(2).Infof("%s \"PHP %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
	}
	return ctx, resp, nil
}
