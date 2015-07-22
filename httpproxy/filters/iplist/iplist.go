package iplist

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
	"github.com/phuslu/goproxy/httpproxy"
	"github.com/phuslu/goproxy/httpproxy/filters"
)

const (
	filterName string = "iplist"
)

type Filter struct {
	filters.RoundTripFilter
	transport *http.Transport
	dialer    *Dialer
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

func NewFilter(config *Config) (filters.Filter, error) {
	d := &Dialer{}
	d.Timeout = time.Duration(config.Dialer.Timeout) * time.Second
	d.KeepAlive = time.Duration(config.Dialer.KeepAlive) * time.Second
	d.Blacklist = make(map[string]struct{})

	d.Window = config.Dialer.Window

	d.Blacklist["127.0.0.1"] = struct{}{}
	d.Blacklist["::1"] = struct{}{}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			switch addr.Network() {
			case "ip":
				d.Blacklist[addr.String()] = struct{}{}
			}
		}
	}

	var err error

	d.hosts = httpproxy.NewHostMatcherWithString(config.Hosts)

	d.iplist, err = NewIplist(config.Iplist, config.DNS.Servers, config.DNS.Blacklist, d.DualStack)
	if err != nil {
		return nil, err
	}

	d.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		ClientSessionCache: tls.NewLRUClientSessionCache(1000),
	}

	d.connTCPDuration = lrucache.NewMultiLRUCache(4, 4096)
	d.connTLSDuration = lrucache.NewMultiLRUCache(4, 4096)
	d.connExpireDuration = 5 * time.Minute

	for _, name := range config.DNS.Expand {
		if _, ok := config.Iplist[name]; ok {
			go func(name string) {
				t := time.Tick(3 * time.Minute)
				for {
					select {
					case <-t:
						d.iplist.ExpandList(name)
					}
				}
			}(name)
		}
	}

	return &Filter{
		transport: &http.Transport{
			Dial:                d.Dial,
			DialTLS:             d.DialTLS,
			DisableKeepAlives:   config.Transport.DisableKeepAlives,
			DisableCompression:  config.Transport.DisableCompression,
			TLSHandshakeTimeout: time.Duration(config.Transport.TLSHandshakeTimeout) * time.Second,
			MaxIdleConnsPerHost: config.Transport.MaxIdleConnsPerHost,
		},
		dialer: d,
	}, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {
	if _, ok := f.dialer.hosts.Lookup(req.Host); !ok {
		return ctx, nil, nil
	}

	switch req.Method {
	case "CONNECT":
		glog.Infof("%s \"IPLIST %s %s %s\" - -", req.RemoteAddr, req.Method, req.Host, req.Proto)
		remote, err := f.transport.Dial("tcp", req.Host)
		if err != nil {
			return ctx, nil, err
		}

		switch req.Proto {
		case "HTTP/2.0":
			rw := ctx.GetResponseWriter()
			io.WriteString(rw, "HTTP/1.1 200 OK\r\n\r\n")
			go httpproxy.IoCopy(remote, req.Body)
			httpproxy.IoCopy(rw, remote)
		case "HTTP/1.1", "HTTP/1.0":
			rw := ctx.GetResponseWriter()
			hijacker, ok := rw.(http.Hijacker)
			if !ok {
				return ctx, nil, fmt.Errorf("http.ResponseWriter(%#v) does not implments Hijacker", rw)
			}
			local, _, err := hijacker.Hijack()
			if err != nil {
				return ctx, nil, err
			}
			local.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			go httpproxy.IoCopy(remote, local)
			httpproxy.IoCopy(local, remote)
		default:
			glog.Warningf("Unkown req=%#v", req)
		}
		ctx.SetHijacked(true)
		return ctx, nil, nil
	case "PRI":
		//TODO: fix for http2
		return ctx, nil, nil
	default:
		resp, err := f.transport.RoundTrip(req)
		if err != nil {
			glog.Errorf("%s \"IPLIST %s %s %s\" error: %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, err)
			data := err.Error()
			resp = &http.Response{
				Status:        "502 Bad Gateway",
				StatusCode:    502,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{},
				Request:       req,
				Close:         true,
				ContentLength: int64(len(data)),
				Body:          ioutil.NopCloser(bytes.NewReader([]byte(data))),
			}
			err = nil
		} else {
			if req.RemoteAddr != "" {
				glog.Infof("%s \"IPLIST %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
			}
		}
		return ctx, resp, err
	}
}
