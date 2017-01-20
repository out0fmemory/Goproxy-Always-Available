package httpproxy

import (
	"net/http"
	"time"

	"github.com/phuslu/glog"

	"./filters"
	"./helpers"

	_ "./filters/auth"
	_ "./filters/autoproxy"
	_ "./filters/autorange"
	_ "./filters/direct"
	_ "./filters/gae"
	_ "./filters/php"
	_ "./filters/rewrite"
	_ "./filters/ssh2"
	_ "./filters/stripssl"
	_ "./filters/vps"
)

type Config struct {
	Enabled          bool
	Address          string
	KeepAlivePeriod  int
	ReadTimeout      int
	WriteTimeout     int
	RequestFilters   []string
	RoundTripFilters []string
	ResponseFilters  []string
}

func ServeProfile(config Config, branding string) error {

	listenOpts := &helpers.ListenOptions{TLSConfig: nil}

	ln, err := helpers.ListenTCP("tcp", config.Address, listenOpts)
	if err != nil {
		glog.Fatalf("ListenTCP(%s, %#v) error: %s", config.Address, listenOpts, err)
	}

	requestFilters, roundtripFilters, responseFilters := getFilters(config)

	h := Handler{
		Listener:         ln,
		RequestFilters:   requestFilters,
		RoundTripFilters: roundtripFilters,
		ResponseFilters:  responseFilters,
		Branding:         branding,
	}

	s := &http.Server{
		Handler:        h,
		ReadTimeout:    time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(config.WriteTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return s.Serve(h.Listener)
}

func getFilters(config Config) ([]filters.RequestFilter, []filters.RoundTripFilter, []filters.ResponseFilter) {

	fs := make(map[string]filters.Filter)
	for _, names := range [][]string{config.RequestFilters,
		config.RoundTripFilters,
		config.ResponseFilters} {
		for _, name := range names {
			if _, ok := fs[name]; !ok {
				f, err := filters.GetFilter(name)
				if err != nil {
					glog.Fatalf("filters.GetFilter(%#v) failed: %#v", name, err)
				}
				fs[name] = f
			}
		}
	}

	requestFilters := make([]filters.RequestFilter, 0)
	for _, name := range config.RequestFilters {
		f := fs[name]
		f1, ok := f.(filters.RequestFilter)
		if !ok {
			glog.Fatalf("%#v is not a RequestFilter", f)
		}
		requestFilters = append(requestFilters, f1)
	}

	roundtripFilters := make([]filters.RoundTripFilter, 0)
	for _, name := range config.RoundTripFilters {
		f := fs[name]
		f1, ok := f.(filters.RoundTripFilter)
		if !ok {
			glog.Fatalf("%#v is not a RoundTripFilter", f)
		}
		roundtripFilters = append(roundtripFilters, f1)
	}

	responseFilters := make([]filters.ResponseFilter, 0)
	for _, name := range config.ResponseFilters {
		f := fs[name]
		f1, ok := f.(filters.ResponseFilter)
		if !ok {
			glog.Fatalf("%#v is not a ResponseFilter", f)
		}
		responseFilters = append(responseFilters, f1)
	}

	return requestFilters, roundtripFilters, responseFilters
}
