package httpproxy

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/phuslu/glog"

	"./filters"
	"./helpers"
	"./storage"

	_ "./filters/auth"
	_ "./filters/autoproxy"
	_ "./filters/autorange"
	_ "./filters/direct"
	_ "./filters/gae"
	_ "./filters/php"
	_ "./filters/ratelimit"
	_ "./filters/rewrite"
	_ "./filters/ssh2"
	_ "./filters/stripssl"
	_ "./filters/vps"
)

type configType map[string]struct {
	Enabled          bool
	Address          string
	KeepAlivePeriod  int
	ReadTimeout      int
	WriteTimeout     int
	RequestFilters   []string
	RoundTripFilters []string
	ResponseFilters  []string
}

var (
	Config configType
)

func init() {
	rand.Seed(time.Now().UnixNano())

	filename := "httpproxy.json"
	err := storage.LookupStoreByFilterName("httpproxy").UnmarshallJson(filename, &Config)
	if err != nil {
		fmt.Printf("storage.ReadJsonConfig(%#v) failed: %s\n", filename, err)
		return
	}
}

func ServeProfile(profile string, branding string) error {
	config, ok := Config[profile]
	if !ok {
		return fmt.Errorf("profile(%#v) not exists", profile)
	}

	listenOpts := &helpers.ListenOptions{TLSConfig: nil}

	ln, err := helpers.ListenTCP("tcp", config.Address, listenOpts)
	if err != nil {
		glog.Fatalf("ListenTCP(%s, %#v) error: %s", config.Address, listenOpts, err)
	}

	requestFilters, roundtripFilters, responseFilters := getFilters(profile)

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

	glog.Infof("ListenAndServe(%#v) on %s\n", profile, h.Listener.Addr().String())
	return s.Serve(h.Listener)
}

func getFilters(profile string) ([]filters.RequestFilter, []filters.RoundTripFilter, []filters.ResponseFilter) {
	config, ok := Config[profile]
	if !ok {
		panic(fmt.Errorf("profile(%#v) not exists", profile))
	}

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
