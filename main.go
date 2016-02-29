package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/golang/groupcache"
	"github.com/phuslu/http2"

	"./httpproxy"
	"./httpproxy/filters"
	"./storage"

	_ "./httpproxy/filters/auth"
	_ "./httpproxy/filters/autoproxy"
	_ "./httpproxy/filters/direct"
	_ "./httpproxy/filters/gae"
	_ "./httpproxy/filters/iplist"
	_ "./httpproxy/filters/php"
	_ "./httpproxy/filters/ratelimit"
	_ "./httpproxy/filters/stripssl"
	_ "./httpproxy/filters/vps"
)

var version = "r9999"

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Config struct {
	LogToStderr bool
	Addr        string
	Http        struct {
		Ssl             bool
		KeepAlivePeriod int
		ReadTimeout     int
		WriteTimeout    int
		Certificate     string
		PrivateKey      string
	}
	GroupCache struct {
		Addr  string
		Peers []string
	}
	Filters struct {
		Request   []string
		RoundTrip []string
		Response  []string
	}
}

func main() {

	filename := "main.json"
	configUri := filters.LookupConfigStoreURI(filename)
	config := new(Config)
	err := storage.ReadJsonConfig(configUri, filename, config)
	if err != nil {
		fmt.Printf("storage.ReadJsonConfig(%#v) failed: %s", filename, err)
		return
	}

	pidfile := ""
	flag.StringVar(&pidfile, "pidfile", "", "goproxy pidfile")
	flag.StringVar(&config.Addr, "addr", config.Addr, "goproxy listen address")
	flag.StringVar(&config.GroupCache.Addr, "groupcache-addr", config.GroupCache.Addr, "groupcache listen address")
	if config.LogToStderr || runtime.GOOS == "windows" {
		logToStderr := true
		for i := 1; i < len(os.Args); i++ {
			if strings.HasPrefix(os.Args[i], "-logtostderr=") {
				logToStderr = false
				break
			}
		}
		if logToStderr {
			flag.Set("logtostderr", "true")
		}
	}
	flag.Parse()

	if runtime.GOOS != "windows" && pidfile != "" {
		if err = ioutil.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
			glog.Fatalf("Write pidfile(%s) error: %s", pidfile, err)
		}
	}

	var ln0 net.Listener
	if config.GroupCache.Addr != "" {
		peers := groupcache.NewHTTPPool("http://" + config.GroupCache.Addr)
		peers.Set(config.GroupCache.Peers...)
		ln0, err = net.Listen("tcp", config.GroupCache.Addr)
		if err != nil {
			glog.Fatalf("ListenTCP(%s) error: %s", config.GroupCache.Addr, err)
		}
		go http.Serve(ln0, peers)
	}

	fmt.Fprintf(os.Stderr, `------------------------------------------------------
GoProxy Version    : %s (go/%s %s/%s)
Listen Address     : %s
RoundTrip Filters  : %v
Pac Server         : http://%s/proxy.pac
------------------------------------------------------
`, version, runtime.Version(), runtime.GOOS, runtime.GOARCH,
		config.Addr,
		fmt.Sprintf("%s|%s|%s", strings.Join(config.Filters.Request, ","), strings.Join(config.Filters.RoundTrip, ","), strings.Join(config.Filters.Response, ",")),
		config.Addr)

	requestFilters, roundtripFilters, responseFilters := getFilters(config)

	var tlsConfig *tls.Config
	if config.Http.Ssl {

		readPem := func(object string) []byte {
			store, err := storage.OpenURI(configUri)
			if err != nil {
				glog.Fatalf("store.OpenURI(%v) error: %s", configUri, err)
			}

			o, err := store.GetObject(object, -1, -1)
			if err != nil {
				glog.Fatalf("store.GetObject(%v) error: %s", object, err)
			}

			rc := o.Body()
			defer rc.Close()

			b, err := ioutil.ReadAll(rc)
			if err != nil {
				glog.Fatalf("ioutil.ReadAll error: %s", err)
			}
			return b
		}

		certPem := readPem(config.Http.Certificate)
		keyPem := readPem(config.Http.Certificate)
		tlsCert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			glog.Fatalf("tls.X509KeyPair error: %s", err)
		}

		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		}
	}

	listenOpts := &httpproxy.ListenOptions{TLSConfig: tlsConfig}

	ln, err := httpproxy.ListenTCP("tcp", config.Addr, listenOpts)
	if err != nil {
		glog.Fatalf("ListenTCP(%s, %#v) error: %s", config.Addr, listenOpts, err)
	}

	h := httpproxy.Handler{
		Listener:         ln,
		RequestFilters:   requestFilters,
		RoundTripFilters: roundtripFilters,
		ResponseFilters:  responseFilters,
	}

	s := &http.Server{
		Handler:        h,
		ReadTimeout:    time.Duration(config.Http.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(config.Http.WriteTimeout) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if config.Http.Ssl {
		s.TLSConfig = tlsConfig
		http2.ConfigureServer(s, &http2.Server{})
	}

	glog.Infof("ListenAndServe on %s\n", h.Listener.Addr().String())
	go s.Serve(h.Listener)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	for {
		switch <-c {
		case os.Interrupt, syscall.SIGTERM:
			os.Exit(0)
		case syscall.SIGHUP:
			glog.Infof("os.StartProcess %#v", os.Args)

			p, err := httpproxy.StartProcess()
			if err != nil {
				glog.Warningf("StartProcess() with Listeners(%#v, %#v) error: %#v, abort", ln0, h.Listener)
				os.Exit(-1)
			} else {
				glog.Infof("Spawn child(pid=%d) OK, exit in %d seconds", p.Pid, config.Http.WriteTimeout)
				if ln0 != nil {
					ln0.Close()
				}
				h.Listener.Close()
			}

			done := make(chan struct{}, 1)
			go func(c chan<- struct{}) {
				h.Listener.Wait()
				c <- struct{}{}
			}(done)

			select {
			case <-done:
				glog.Infof("All connections were closed, graceful shutdown")
				os.Exit(0)
			case <-time.After(s.WriteTimeout):
				glog.Warningf("Graceful shutdown timeout, quit")
				os.Exit(0)
			}
		}
	}
}

func getFilters(config *Config) ([]filters.RequestFilter, []filters.RoundTripFilter, []filters.ResponseFilter) {

	fs := make(map[string]filters.Filter)
	for _, names := range [][]string{config.Filters.Request,
		config.Filters.RoundTrip,
		config.Filters.Response} {
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
	for _, name := range config.Filters.Request {
		f := fs[name]
		f1, ok := f.(filters.RequestFilter)
		if !ok {
			glog.Fatalf("%#v is not a RequestFilter", f)
		}
		requestFilters = append(requestFilters, f1)
	}

	roundtripFilters := make([]filters.RoundTripFilter, 0)
	for _, name := range config.Filters.RoundTrip {
		f := fs[name]
		f1, ok := f.(filters.RoundTripFilter)
		if !ok {
			glog.Fatalf("%#v is not a RoundTripFilter", f)
		}
		roundtripFilters = append(roundtripFilters, f1)
	}

	responseFilters := make([]filters.ResponseFilter, 0)
	for _, name := range config.Filters.Response {
		f := fs[name]
		f1, ok := f.(filters.ResponseFilter)
		if !ok {
			glog.Fatalf("%#v is not a ResponseFilter", f)
		}
		responseFilters = append(responseFilters, f1)
	}

	return requestFilters, roundtripFilters, responseFilters
}
