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
	"github.com/phuslu/goproxy/httpproxy"
	"github.com/phuslu/goproxy/httpproxy/filters"
	_ "github.com/phuslu/goproxy/httpproxy/filters/auth"
	_ "github.com/phuslu/goproxy/httpproxy/filters/autoproxy"
	_ "github.com/phuslu/goproxy/httpproxy/filters/direct"
	_ "github.com/phuslu/goproxy/httpproxy/filters/gae"
	_ "github.com/phuslu/goproxy/httpproxy/filters/iplist"
	_ "github.com/phuslu/goproxy/httpproxy/filters/php"
	_ "github.com/phuslu/goproxy/httpproxy/filters/stripssl"
	"github.com/phuslu/goproxy/storage"
	"github.com/phuslu/http2"
)

const (
	Version = "@VERSION@"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
}

func main() {

	configUri := filters.LookupConfigStoreURI("main.json")
	filename := "main.json"
	config, err := NewConfig(configUri, filename)
	if err != nil {
		fmt.Printf("NewConfig(%#v) failed: %s", filename, err)
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
`, Version, runtime.Version(), runtime.GOOS, runtime.GOARCH,
		config.Addr,
		strings.Join(config.Filters.RoundTrip, ","),
		config.Addr)

	requestFilters, roundtripFilters, responseFilters := getFilters(config)

	var tlsConfig *tls.Config
	if config.Http.Mode == "h2" {

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

	if strings.HasPrefix(config.Http.Mode, "h2") {
		http2.VerboseLogs = true
		listenOpts.KeepAlivePeriod = 3 * time.Minute
	}

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

	switch config.Http.Mode {
	case "h2":
		s.TLSConfig = tlsConfig
		http2.ConfigureServer(s, &http2.Server{})
	case "h2c":
		s.TLSConfig = tlsConfig
		s = http2.UpgradeServer(s, &http2.Server{})
	case "h1":
		break
	default:
		glog.Fatalf("Unknow Http mode %s", config.Http.Mode)
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
