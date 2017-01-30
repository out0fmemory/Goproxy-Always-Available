// https://git.io/goproxy

package main

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/naoina/toml"
	"github.com/phuslu/glog"
	"github.com/phuslu/goproxy/httpproxy/helpers"
	"github.com/phuslu/goproxy/httpproxy/proxy"
	"github.com/phuslu/net/http2"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/context"
)

var (
	version = "r9999"
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

type SimplePAM struct {
	CacheSize uint

	path  string
	cache lrucache.Cache
	once  sync.Once
}

func (p *SimplePAM) init() {
	p.cache = lrucache.NewLRUCache(p.CacheSize)

	exe, err := os.Executable()
	if err != nil {
		glog.Fatalf("os.Executable() error: %+v", err)
	}

	p.path = filepath.Join(filepath.Dir(exe), "pwauth")
	if _, err := os.Stat(p.path); err != nil {
		glog.Fatalf("os.Stat(%#v) error: %+v", p.path, err)
	}

	if syscall.Geteuid() != 0 {
		glog.Warningf("Please run as root if you want to use pam auth")
	}
}

func (p *SimplePAM) Authenticate(username, password string) error {
	p.once.Do(p.init)

	auth := username + ":" + password

	if _, ok := p.cache.GetNotStale(auth); ok {
		return nil
	}

	cmd := exec.Command(p.path)
	cmd.Stdin = strings.NewReader(username + "\n" + password + "\n")
	err := cmd.Run()

	if err != nil {
		glog.Warningf("SimplePAM: username=%v password=%v error: %+v", username, password, err)
		time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
		return err
	}

	p.cache.Set(auth, struct{}{}, time.Now().Add(2*time.Hour))
	return nil
}

type HTTPHandler struct {
	Dial func(network, address string) (net.Conn, error)
	*http.Transport
	*SimplePAM
}

func (h *HTTPHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
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

	if h.SimplePAM != nil {
		auth := req.Header.Get("Proxy-Authorization")
		if auth == "" {
			h.ProxyAuthorizationReqiured(rw, req)
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "Basic":
				if userpass, err := base64.StdEncoding.DecodeString(parts[1]); err == nil {
					parts := strings.Split(string(userpass), ":")
					username := parts[0]
					password := parts[1]

					if err := h.SimplePAM.Authenticate(username, password); err != nil {
						http.Error(rw, "403 Forbidden", http.StatusForbidden)
					}
				}
			default:
				glog.Errorf("Unrecognized auth type: %#v", parts[0])
				http.Error(rw, "403 Forbidden", http.StatusForbidden)
				return
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

		dial := h.Dial
		if dial == nil {
			dial = h.Transport.Dial
		}

		conn, err := dial("tcp", net.JoinHostPort(host, port))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}

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

		io.WriteString(lconn, "HTTP/1.1 200 OK\r\n\r\n")

		defer lconn.Close()
		defer conn.Close()

		go helpers.IOCopy(conn, lconn)
		helpers.IOCopy(lconn, conn)

		return
	}

	if req.Host == "" {
		http.Error(rw, "400 Bad Request", http.StatusBadRequest)
		return
	}

	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}

	if req.ContentLength == 0 {
		io.Copy(ioutil.Discard, req.Body)
		req.Body.Close()
		req.Body = nil
	}

	glog.Infof("%s \"%s %s %s\" - -", req.RemoteAddr, req.Method, req.URL.String(), req.Proto)

	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
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

	defer resp.Body.Close()

	var r io.Reader = resp.Body
	helpers.IOCopy(rw, r)
}

func (h *HTTPHandler) ProxyAuthorizationReqiured(rw http.ResponseWriter, req *http.Request) {
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

type HTTP2Handler struct {
	Fallback     *url.URL
	DisableProxy bool
	Dial         func(network, address string) (net.Conn, error)
	*http.Transport
	*SimplePAM
}

func (h *HTTP2Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var err error

	var h2 bool = req.ProtoMajor == 2 && req.ProtoMinor == 0

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

	if h.DisableProxy && req.Host != req.TLS.ServerName {
		http.Error(rw, "403 Forbidden", http.StatusForbidden)
		return
	}

	if h.SimplePAM != nil && req.Host != req.TLS.ServerName {
		auth := req.Header.Get("Proxy-Authorization")
		if auth == "" {
			h.ProxyAuthorizationReqiured(rw, req)
			return
		}

		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "Basic":
				if userpass, err := base64.StdEncoding.DecodeString(parts[1]); err == nil {
					parts := strings.Split(string(userpass), ":")
					username := parts[0]
					password := parts[1]

					if err := h.SimplePAM.Authenticate(username, password); err != nil {
						http.Error(rw, "403 Forbidden", http.StatusForbidden)
					}
				}
			default:
				glog.Errorf("Unrecognized auth type: %#v", parts[0])
				http.Error(rw, "403 Forbidden", http.StatusForbidden)
				return
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

		dial := h.Dial
		if dial == nil {
			dial = h.Transport.Dial
		}

		conn, err := dial("tcp", net.JoinHostPort(host, port))
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

		if h2 {
			w = FlushWriter{rw}
			r = req.Body
		} else {
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

		defer conn.Close()

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

	if req.ContentLength == 0 {
		io.Copy(ioutil.Discard, req.Body)
		req.Body.Close()
		req.Body = nil
	}

	glog.Infof("%s \"%s %s %s\" - -", req.RemoteAddr, req.Method, req.URL.String(), req.Proto)

	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}

	if h2 {
		req.ProtoMajor = 1
		req.ProtoMinor = 1
		req.Proto = "HTTP/1.1"
	}

	if req.URL.Host == req.TLS.ServerName && h.Fallback != nil {
		req.URL.Scheme = h.Fallback.Scheme
		req.URL.Scheme = h.Fallback.Scheme
		req.URL.Host = h.Fallback.Host
		if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			req.Header.Set("X-Real-IP", ip)
			xff := req.Header.Get("X-Forwarded-For")
			if xff == "" {
				req.Header.Set("X-Forwarded-For", ip)
			} else {
				req.Header.Set("X-Forwarded-For", xff+", "+ip)
			}
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

	if h2 {
		resp.Header.Del("Connection")
		resp.Header.Del("Keep-Alive")
	}

	for key, values := range resp.Header {
		for _, value := range values {
			rw.Header().Add(key, value)
		}
	}
	rw.WriteHeader(resp.StatusCode)

	defer resp.Body.Close()

	var r io.Reader = resp.Body
	helpers.IOCopy(rw, r)
}

func (h *HTTP2Handler) ProxyAuthorizationReqiured(rw http.ResponseWriter, req *http.Request) {
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

type CertManager struct {
	hosts   []string
	certs   map[string]*tls.Certificate
	manager *autocert.Manager
}

func (cm *CertManager) Add(host string, certfile, keyfile string) error {
	if cm.manager == nil {
		cm.manager = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache("."),
			HostPolicy: cm.HostPolicy,
		}
	}

	if cm.certs == nil {
		cm.certs = make(map[string]*tls.Certificate)
	}

	if certfile != "" && keyfile != "" {
		cert, err := tls.LoadX509KeyPair(certfile, keyfile)
		if err != nil {
			return err
		}
		cm.certs[host] = &cert
	} else {
		cm.certs[host] = nil
	}

	cm.hosts = append(cm.hosts, host)

	return nil
}

func (cm *CertManager) HostPolicy(_ context.Context, host string) error {
	if _, ok := cm.certs[host]; !ok {
		return errors.New("acme/autocert: host not configured")
	}
	return nil
}

func (cm *CertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName == "" {
		hello.ServerName = cm.hosts[0]
	}
	if cert, ok := cm.certs[hello.ServerName]; ok && cert != nil {
		return cert, nil
	}
	return cm.manager.GetCertificate(hello)
}

type Config struct {
	Default struct {
		LogLevel int
	}
	HTTP2 []struct {
		Listen string

		ServerName []string
		Keyfile    string
		Certfile   string

		ParentProxy string

		ProxyFallback   string
		DisableProxy    bool
		ProxyAuthMethod string
	}
	HTTP struct {
		Listen string

		ParentProxy string

		ProxyAuthMethod string
	}
}

type Handler struct {
	Domains  []string
	Handlers map[string]http.Handler
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	handler, ok := h.Handlers[req.TLS.ServerName]
	if !ok {
		handler, ok = h.Handlers[h.Domains[0]]
		if !ok {
			http.Error(rw, "403 Forbidden", http.StatusForbidden)
			return
		}
	}
	handler.ServeHTTP(rw, req)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Print(version)
		return
	}

	var config Config

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "os.Executable() error: %+v\n", err)
		os.Exit(1)
	}

	for _, filename := range []string{exe + ".user.toml", exe + ".toml"} {
		if _, err := os.Stat(filename); err == nil {
			tomlData, err := ioutil.ReadFile(filename)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ioutil.ReadFile(%#v) error: %+v\n", filename, err)
				os.Exit(1)
			}

			err = toml.Unmarshal(tomlData, &config)
			if err != nil {
				fmt.Fprintf(os.Stderr, "toml.Decode(%s) error: %+v\n", tomlData, err)
				os.Exit(1)
			}

			break
		}
	}

	helpers.SetFlagsIfAbsent(map[string]string{
		"logtostderr": "true",
		"v":           strconv.Itoa(config.Default.LogLevel),
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

	cm := &CertManager{}
	h := &Handler{
		Handlers: map[string]http.Handler{},
		Domains:  []string{},
	}
	for _, server := range config.HTTP2 {
		handler := &HTTP2Handler{
			Transport: transport,
		}

		if server.ProxyFallback != "" {
			handler.Fallback, err = url.Parse(server.ProxyFallback)
			if err != nil {
				glog.Fatalf("url.Parse(%+v) error: %+v", server.ProxyFallback, err)
			}
			handler.DisableProxy = server.DisableProxy
		}

		if server.ParentProxy != "" {
			handler.Transport = &http.Transport{}
			*handler.Transport = *transport

			fixedURL, err := url.Parse(server.ParentProxy)
			if err != nil {
				glog.Fatalf("url.Parse(%#v) error: %+v", server.ParentProxy, err)
			}

			switch fixedURL.Scheme {
			case "http":
				handler.Transport.Proxy = http.ProxyURL(fixedURL)
				fallthrough
			default:
				dialer2, err := proxy.FromURL(fixedURL, dialer, nil)
				if err != nil {
					glog.Fatalf("proxy.FromURL(%#v) error: %s", fixedURL.String(), err)
				}
				handler.Dial = dialer2.Dial
			}
		}

		switch server.ProxyAuthMethod {
		case "pam":
			if _, err := exec.LookPath("python"); err != nil {
				glog.Fatalf("pam: exec.LookPath(\"python\") error: %+v", err)
			}
			handler.SimplePAM = &SimplePAM{
				CacheSize: 2048,
			}
		case "":
			break
		default:
			glog.Fatalf("unsupport proxy_auth_method(%+v)", server.ProxyAuthMethod)
		}

		for _, servername := range server.ServerName {
			cm.Add(servername, server.Certfile, server.Keyfile)
			h.Domains = append(h.Domains, servername)
			h.Handlers[servername] = handler
		}
	}

	srv := &http.Server{
		Handler: h,
		TLSConfig: &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: cm.GetCertificate,
		},
	}

	http2.ConfigureServer(srv, &http2.Server{})

	seen := make(map[string]struct{})
	for _, server := range config.HTTP2 {
		addr := server.Listen
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			glog.Fatalf("Listen(%s) error: %s", addr, err)
		}
		glog.Infof("goproxy-vps %s ListenAndServe on %s\n", version, ln.Addr().String())
		go srv.Serve(tls.NewListener(TCPListener{ln.(*net.TCPListener)}, srv.TLSConfig))
	}

	if config.HTTP.Listen != "" {
		server := config.HTTP
		addr := server.Listen
		if _, ok := seen[addr]; ok {
			glog.Fatalf("goproxy-vps: addr(%#v) already listened by http2", addr)
		}

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			glog.Fatalf("Listen(%s) error: %s", addr, err)
		}

		handler := &HTTPHandler{
			Transport: transport,
		}

		if server.ParentProxy != "" {
			handler.Transport = &http.Transport{}
			*handler.Transport = *transport

			fixedURL, err := url.Parse(server.ParentProxy)
			if err != nil {
				glog.Fatalf("url.Parse(%#v) error: %+v", server.ParentProxy, err)
			}

			switch fixedURL.Scheme {
			case "http":
				handler.Transport.Proxy = http.ProxyURL(fixedURL)
				fallthrough
			default:
				dialer2, err := proxy.FromURL(fixedURL, dialer, nil)
				if err != nil {
					glog.Fatalf("proxy.FromURL(%#v) error: %s", fixedURL.String(), err)
				}
				handler.Dial = dialer2.Dial
			}
		}

		switch server.ProxyAuthMethod {
		case "pam":
			if _, err := exec.LookPath("python"); err != nil {
				glog.Fatalf("pam: exec.LookPath(\"python\") error: %+v", err)
			}
			handler.SimplePAM = &SimplePAM{
				CacheSize: 2048,
			}
		case "":
			break
		default:
			glog.Fatalf("unsupport proxy_auth_method(%+v)", server.ProxyAuthMethod)
		}

		srv := &http.Server{
			Handler: handler,
		}

		glog.Infof("goproxy-vps %s ListenAndServe on %s\n", version, ln.Addr().String())
		go srv.Serve(ln)
	}

	select {}
}
