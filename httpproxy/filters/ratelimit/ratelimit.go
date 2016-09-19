package auth

import (
	"context"
	"io"
	"net/http"

	"github.com/juju/ratelimit"
	"github.com/phuslu/glog"

	"../../filters"
	"../../storage"
)

const (
	filterName string = "ratelimit"
)

type Config struct {
	Threshold int
	Rate      int
	Capacity  int
}

type Filter struct {
	Config
	Threshold int64
	Rate      float64
	Capacity  int64
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
	f := &Filter{
		Config:    *config,
		Threshold: int64(config.Threshold),
		Capacity:  int64(config.Capacity),
		Rate:      float64(config.Rate),
	}

	if config.Capacity <= 0 {
		f.Capacity = int64(config.Rate) * 1024
	}

	return f, nil
}

func (f *Filter) FilterName() string {
	return filterName
}

func (f *Filter) Response(ctx context.Context, resp *http.Response) (context.Context, *http.Response, error) {

	if f.Rate > 0 && resp.ContentLength > f.Threshold {
		glog.V(2).Infof("RateLimit %#v rate to %#v", resp.Request.URL.String(), f.Rate)
		resp.Body = NewRateLimitReader(resp.Body, f.Rate, f.Capacity)
	}

	return ctx, resp, nil
}

type rateLimitReader struct {
	rc  io.ReadCloser // underlying reader
	rlr io.Reader     // ratelimit.Reader
}

func NewRateLimitReader(rc io.ReadCloser, rate float64, capacity int64) io.ReadCloser {
	var rlr rateLimitReader

	rlr.rc = rc
	rlr.rlr = ratelimit.Reader(rc, ratelimit.NewBucketWithRate(rate, capacity))

	return &rlr
}

func (rlr *rateLimitReader) Read(p []byte) (n int, err error) {
	return rlr.rlr.Read(p)
}

func (rlr *rateLimitReader) Close() error {
	return rlr.rc.Close()
}
