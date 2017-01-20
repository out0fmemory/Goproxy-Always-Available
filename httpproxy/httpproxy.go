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

	h := Handler{
		Listener:         ln,
		RequestFilters:   []filters.RequestFilter{},
		RoundTripFilters: []filters.RoundTripFilter{},
		ResponseFilters:  []filters.ResponseFilter{},
		Branding:         branding,
	}

	for _, name := range config.RequestFilters {
		f, err := filters.GetFilter(name)
		f1, ok := f.(filters.RequestFilter)
		if !ok {
			glog.Fatalf("%#v is not a RequestFilter, err=%+v", f, err)
		}
		h.RequestFilters = append(h.RequestFilters, f1)
	}

	for _, name := range config.RoundTripFilters {
		f, err := filters.GetFilter(name)
		f1, ok := f.(filters.RoundTripFilter)
		if !ok {
			glog.Fatalf("%#v is not a RoundTripFilter, err=%+v", f, err)
		}
		h.RoundTripFilters = append(h.RoundTripFilters, f1)
	}

	for _, name := range config.ResponseFilters {
		f, err := filters.GetFilter(name)
		f1, ok := f.(filters.ResponseFilter)
		if !ok {
			glog.Fatalf("%#v is not a ResponseFilter, err=%+v", f, err)
		}
		h.ResponseFilters = append(h.ResponseFilters, f1)
	}

	s := &http.Server{
		Handler:        h,
		ReadTimeout:    time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(config.WriteTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return s.Serve(h.Listener)
}
