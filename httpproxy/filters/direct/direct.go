package direct

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"

	"../../../httpproxy"
	"../../../storage"
	"../../filters"
	"../../transport/direct"
)

const (
	filterName string = "direct"
)

type Config struct {
	Dialer struct {
		Timeout   int
		KeepAlive int
		DualStack bool
	}
	Transport struct {
		DisableKeepAlives   bool
		DisableCompression  bool
		TLSHandshakeTimeout int
		MaxIdleConnsPerHost int
	}
	DNSCache struct {
		Size    int
		Expires int
	}
}

type Filter struct {
	filters.RoundTripFilter
	transport *direct.Transport
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
	tr := &direct.Transport{
		Dialer: &direct.Dialer{},
	}

	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: false,
		ClientSessionCache: tls.NewLRUClientSessionCache(1000),
	}
	tr.TLSHandshakeTimeout = time.Duration(config.Transport.TLSHandshakeTimeout) * time.Second
	tr.MaxIdleConnsPerHost = config.Transport.MaxIdleConnsPerHost
	tr.DisableCompression = config.Transport.DisableCompression

	return &Filter{
		transport: tr,
	}, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) RoundTrip(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Response, error) {
	switch req.Method {
	case "CONNECT":
		glog.Infof("%s \"DIRECT %s %s %s\" - -", req.RemoteAddr, req.Method, req.Host, req.Proto)
		rconn, err := f.transport.Dial("tcp", req.Host)
		if err != nil {
			return ctx, nil, err
		}

		rw := ctx.GetResponseWriter()

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

		go httpproxy.IoCopy(rconn, lconn)
		httpproxy.IoCopy(lconn, rconn)

		ctx.SetHijacked(true)
		return ctx, nil, nil
	case "PRI":
		//TODO: fix for http2
		return ctx, nil, nil
	default:
		resp, err := f.transport.RoundTrip(req)

		if err != nil {
			glog.Errorf("%s \"DIRECT %s %s %s\" error: %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, err)
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
				glog.Infof("%s \"DIRECT %s %s %s\" %d %s", req.RemoteAddr, req.Method, req.URL.String(), req.Proto, resp.StatusCode, resp.Header.Get("Content-Length"))
			}
		}
		return ctx, resp, err
	}
}
