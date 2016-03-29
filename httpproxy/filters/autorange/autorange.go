package autorange

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	"../../../httpproxy"
	"../../../storage"
	"../../filters"
	"../../transport"
)

const (
	filterName string = "autorange"
)

type Config struct {
	Sites   []string
	MaxSize int
	BufSize int
	Threads int
}

type Filter struct {
	Config
	SiteMatcher *httpproxy.HostMatcher
	MaxSize     int
	BufSize     int
	Threads     int
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
	f := &Filter{
		Config:      *config,
		SiteMatcher: httpproxy.NewHostMatcher(config.Sites),
		MaxSize:     config.MaxSize,
		BufSize:     config.BufSize,
		Threads:     config.Threads,
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) Request(ctx *filters.Context, req *http.Request) (*filters.Context, *http.Request, error) {
	if req.Method != http.MethodGet || strings.Contains(req.URL.RawQuery, "range=") {
		return ctx, req, nil
	}

	if r := req.Header.Get("Range"); r == "" {
		switch {
		case f.SiteMatcher.Match(req.Host):
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", 0, f.MaxSize))
			glog.V(2).Infof("AUTORANGE Sites rule matched, add %s for\"%s\"", req.Header.Get("Range"), req.URL.String())
		default:
			glog.V(2).Infof("AUTORANGE ignore preserved range=%v", r)
		}
	} else {
		parts := strings.Split(r, " ")
		switch parts[0] {
		case "bytes":
			parts1 := strings.Split(parts[1], "-")
			if start, err := strconv.Atoi(parts1[0]); err == nil {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+f.MaxSize))
				glog.V(2).Infof("AUTORANGE Default rule matched, change %s to %s for\"%s\"", r, req.Header.Get("Range"), req.URL.String())
			}
		default:
			glog.Warningf("AUTORANGE Default rule matched, but cannot support %#v range for \"%s\"", r, req.URL.String())
		}
	}

	return ctx, req, nil
}

func (f *Filter) Response(ctx *filters.Context, resp *http.Response) (*filters.Context, *http.Response, error) {
	if resp.StatusCode != http.StatusPartialContent || resp.Header.Get("Content-Length") == "" {
		return ctx, resp, nil
	}

	f1 := ctx.GetRoundTripFilter()
	if f1 == nil {
		return ctx, resp, nil
	}

	parts := strings.Split(resp.Header.Get("Content-Range"), " ")
	if len(parts) != 2 || parts[0] != "bytes" {
		return ctx, resp, nil
	}

	parts1 := strings.Split(parts[1], "/")
	parts2 := strings.Split(parts1[0], "-")
	if len(parts1) != 2 || len(parts2) != 2 {
		return ctx, resp, nil
	}

	var end, length int64
	var err error

	end, err = strconv.ParseInt(parts2[1], 10, 64)
	if err != nil {
		return ctx, resp, nil
	}

	length, err = strconv.ParseInt(parts1[1], 10, 64)
	if err != nil {
		return ctx, resp, nil
	}

	glog.V(2).Infof("AUTORANGE respone matched, start rangefetch for %#v", resp.Header.Get("Content-Range"))

	resp.ContentLength = length
	resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	resp.Header.Del("Content-Range")

	ap := NewAutoPipe(f.Threads)

	go func(ap *autoPipe, filter filters.RoundTripFilter, req0 *http.Request, start, length int64) {
		glog.V(2).Infof("AUTORANGE begin rangefetch for %#v by using %#v", req0.URL.String(), filter.FilterName())

		req, err := http.NewRequest(req0.Method, req0.URL.String(), nil)
		if err != nil {
			glog.Warningf("AUTORANGE http.NewRequest(%#v) error: %#v", req, err)
			return
		}

		for key, values := range req0.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		ap.WaitForReading()
		var index uint32
		for {
			if ap.FatalErr() {
				break
			}
			if start > length-1 {
				ap.WClose()
				break
			}
			//FIXME: make this configurable!
			if ap.Len() > 128*1024*1024 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			//end := start + int64(f.BufSize-1)
			end := start + int64(1024<<10-1)
			if end > length-1 {
				end = length - 1
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

			_, resp, err := filter.RoundTrip(nil, req)
			if err != nil {
				glog.Warningf("AUTORANGE %#v.RoundTrip(%v) error: %#v", filter, req, err)
				time.Sleep(1 * time.Second)
				continue
			}
			if resp.StatusCode != http.StatusPartialContent {
				continue // TODO: rewise, retry times
			}

			ap.Threads <- true
			go func(index uint32, resp *http.Response) {
				piper := ap.NewPiper(index)
				_, err := httpproxy.IoCopy(piper, resp.Body)
				if err != nil {
					glog.Warningf("AUTORANGE httpproxy.IoCopy(%#v) error: %#v", resp.Body, err)
					piper.EIndex()
				}
				piper.WClose()
				<-ap.Threads
				resp.Body.Close()
			}(index, resp)

			start = end + 1
			index++
		}
	}(ap, f1, resp.Request, end+1, length)

	resp.Body = transport.NewMultiReadCloser(resp.Body, ap)

	return ctx, resp, nil
}
