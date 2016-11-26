// https://git.io/goproxy

package main

import (
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/goproxy/httpproxy/helpers"
	"github.com/phuslu/net/http2"
	"golang.org/x/crypto/acme/autocert"
)

var (
	version = "r9999"
)

var (
	ListenAddrs string = os.Getenv("GOPROXY_VPS_LISTEN_ADDRS")
	ACMEDomain  string = os.Getenv("GOPROXY_VPS_ACME_DOMAIN")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type FlushWriter struct {
	w io.Writer
}

func (fw FlushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

type TCPListener struct {
	*net.TCPListener
}

func (ln TCPListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	tc.SetReadBuffer(32 * 1024)
	tc.SetWriteBuffer(32 * 1024)
	return tc, nil
}

type Handler struct {
	PWAuthEnabled bool
	PWAuthCache   lrucache.Cache
	PWAuthPath    string
	*http.Transport
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var err error

	var paramsPreifx string = http.CanonicalHeaderKey("X-UrlFetch-")
	params := http.Header{}
	for key, values := range req.Header {
		if strings.HasPrefix(key, paramsPreifx) {
			params[key] = values
		}
	}

	for key := range params {
		req.Header.Del(key)
	}

	if h.PWAuthEnabled {
		auth := req.Header.Get("Proxy-Authorization")
		if auth == "" {
			h.ProxyAuthorizationReqiured(rw, req)
			return
		}
		if _, ok := h.PWAuthCache.GetNotStale(auth); !ok {
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) == 2 {
				switch parts[0] {
				case "Basic":
					if userpass, err := base64.StdEncoding.DecodeString(parts[1]); err == nil {
						parts := strings.Split(string(userpass), ":")
						username := parts[0]
						password := parts[1]

						cmd := exec.Command(h.PWAuthPath)
						cmd.Stdin = strings.NewReader(username + "\n" + password + "\n")
						err = cmd.Run()

						if err != nil {
							glog.Warningf("pwauth: username=%v password=%v error: %+v", username, password, err)
							time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
							h.ProxyAuthorizationReqiured(rw, req)
							return
						}

						h.PWAuthCache.Set(auth, struct{}{}, time.Now().Add(2*time.Hour))
					}
				default:
					glog.Errorf("Unrecognized auth type: %#v", parts[0])
					http.Error(rw, "403 Forbidden", http.StatusForbidden)
					return
				}
			}
		}
		req.Header.Del("Proxy-Authorization")
	}

	if req.Method == http.MethodConnect {
		host, port, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
			port = "443"
		}

		glog.Infof("%s \"%s %s:%s %s\" - -", req.RemoteAddr, req.Method, host, port, req.Proto)

		conn, err := h.Transport.Dial("tcp", net.JoinHostPort(host, port))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}

		flusher, ok := rw.(http.Flusher)
		if !ok {
			http.Error(rw, fmt.Sprintf("%#v is not http.Flusher", rw), http.StatusBadGateway)
			return
		}

		rw.WriteHeader(http.StatusOK)
		flusher.Flush()

		var w io.Writer
		var r io.Reader

		switch req.ProtoMajor {
		case 2:
			w = FlushWriter{rw}
			r = req.Body
		default:
			hijacker, ok := rw.(http.Hijacker)
			if !ok {
				http.Error(rw, fmt.Sprintf("%#v is not http.Hijacker", rw), http.StatusBadGateway)
				return
			}
			lconn, _, err := hijacker.Hijack()
			if err != nil {
				http.Error(rw, err.Error(), http.StatusBadGateway)
				return
			}
			defer lconn.Close()

			w = lconn
			r = lconn
		}

		go helpers.IOCopy(conn, r)
		helpers.IOCopy(w, conn)

		return
	}

	if req.Host == "" {
		http.Error(rw, "403 Forbidden", http.StatusForbidden)
		return
	}

	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}

	glog.Infof("%s \"%s %s %s\" - -", req.RemoteAddr, req.Method, req.URL.String(), req.Proto)

	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}

	if req.ProtoMajor == 2 && req.ProtoMinor == 0 {
		req.ProtoMajor = 1
		req.ProtoMinor = 1
		req.Proto = "HTTP/1.1"
	}

	if req.ContentLength > 0 {
		if req.Header.Get("Content-Length") == "" {
			req.Header.Set("Content-Length", strconv.FormatInt(req.ContentLength, 10))
		}
	}

	resp, err := h.Transport.RoundTrip(req)
	if err != nil {
		msg := err.Error()
		if strings.HasPrefix(msg, "Invaid DNS Record: ") {
			http.Error(rw, "403 Forbidden", http.StatusForbidden)
		} else {
			http.Error(rw, err.Error(), http.StatusBadGateway)
		}
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			rw.Header().Add(key, value)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	helpers.IOCopy(rw, resp.Body)
}

func (h *Handler) ProxyAuthorizationReqiured(rw http.ResponseWriter, req *http.Request) {
	data := "Proxy Authentication Required"
	resp := &http.Response{
		StatusCode: http.StatusProxyAuthRequired,
		Header: http.Header{
			"Proxy-Authenticate": []string{"Basic realm=\"Proxy Authentication Required\""},
		},
		Request:       req,
		ContentLength: int64(len(data)),
		Body:          ioutil.NopCloser(strings.NewReader(data)),
	}
	for key, values := range resp.Header {
		for _, value := range values {
			rw.Header().Add(key, value)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	helpers.IOCopy(rw, resp.Body)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Print(version)
		return
	}

	addrs := ":443"
	pwauth := false
	acmeDomain := ""
	keyFile := "goproxy-vps.key"
	certFile := "goproxy-vps.crt"
	http2verbose := false

	flag.StringVar(&addrs, "addr", addrs, "goproxy vps listen addrs, i.e. 0.0.0.0:443,0.0.0.0:8443")
	flag.StringVar(&acmeDomain, "acmedomain", acmeDomain, "goproxy vps acme domain, i.e. vps.example.com")
	flag.StringVar(&keyFile, "keyfile", keyFile, "goproxy vps keyfile")
	flag.StringVar(&certFile, "certfile", certFile, "goproxy vps certfile")
	flag.BoolVar(&http2verbose, "http2verbose", http2verbose, "goproxy vps http2 verbose mode")
	flag.BoolVar(&pwauth, "pwauth", pwauth, "goproxy vps enable pwauth")

	helpers.SetFlagsIfAbsent(map[string]string{
		"logtostderr": "true",
		"v":           "2",
	})
	flag.Parse()

	dialer := &helpers.Dialer{
		Dialer: &net.Dialer{
			KeepAlive: 0,
			Timeout:   16 * time.Second,
			DualStack: true,
		},
		Resolver: &helpers.Resolver{
			LRUCache:  lrucache.NewLRUCache(8 * 1024),
			BlackList: lrucache.NewLRUCache(1024),
			DNSExpiry: 8 * time.Hour,
		},
	}

	if ips, err := helpers.LocalIPv4s(); err == nil {
		for _, ip := range ips {
			dialer.Resolver.BlackList.Set(ip.String(), struct{}{}, time.Time{})
		}
		for _, s := range []string{"127.0.0.1", "::1"} {
			dialer.Resolver.BlackList.Set(s, struct{}{}, time.Time{})
		}
	}

	transport := &http.Transport{
		Dial: dialer.Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(1024),
		},
		TLSHandshakeTimeout: 16 * time.Second,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     180,
		DisableCompression:  false,
	}

	handler := &Handler{
		PWAuthEnabled: pwauth,
		Transport:     transport,
	}

	if handler.PWAuthEnabled {
		handler.PWAuthCache = lrucache.NewLRUCache(1024)
		handler.PWAuthPath = filepath.Join(filepath.Dir(os.Args[0]), "./pwauth")
		if _, err := os.Stat(handler.PWAuthPath); err != nil {
			glog.Fatalf("Ensure bundled `pwauth' error: %+v", err)
		}

		switch runtime.GOOS {
		case "linux", "freebsd", "darwin":
			if u, err := user.Current(); err == nil && u.Uid == "0" {
				glog.Warningf("If you want to use system native pwauth, please run as root, otherwise please add/edit pwauth.txt.")
			}
		case "windows":
			glog.Warningf("Current platform %+v not support native pwauth, please add/edit pwauth.txt.", runtime.GOOS)
		}
	}

	srv := &http.Server{
		Handler: handler,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	if acmeDomain != "" {
		m := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache("."),
			HostPolicy: autocert.HostWhitelist(strings.Split(acmeDomain, ",")...),
		}

		srv.TLSConfig.GetCertificate = m.GetCertificate
	} else {
		if _, err := os.Stat(keyFile); os.IsNotExist(err) {
			cmd := exec.Command("openssl",
				"req",
				"-subj", "/CN=github.com/O=GitHub, Inc./C=US",
				"-new",
				"-newkey", "rsa:2048",
				"-days", "365",
				"-nodes",
				"-x509",
				"-keyout", keyFile,
				"-out", certFile)
			if err = cmd.Run(); err != nil {
				glog.Fatalf("openssl: %+v error: %+v", cmd.Args, err)
			}
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			glog.Fatalf("LoadX509KeyPair(%#v, %#v) error: %v", certFile, keyFile, err)
		}

		srv.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	http2.VerboseLogs = http2verbose
	http2.ConfigureServer(srv, &http2.Server{})

	if ListenAddrs != "" {
		addrs = ListenAddrs
	}
	for _, addr := range strings.Split(addrs, ",") {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			glog.Fatalf("Listen(%s) error: %s", addr, err)
		}
		glog.Infof("goproxy-vps %s ListenAndServe on %s\n", version, ln.Addr().String())
		go srv.Serve(tls.NewListener(TCPListener{ln.(*net.TCPListener)}, srv.TLSConfig))
	}

	select {}
}
