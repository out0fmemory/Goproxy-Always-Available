// https://git.io/goproxy

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
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

type SimpleAuth struct {
	Mode      string
	CacheSize uint
	Builtin   map[string]string

	path  string
	cache lrucache.Cache
	once  sync.Once
}

func (p *SimpleAuth) init() {
	if p.Mode == "builtin" {
		return
	}

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

func (p *SimpleAuth) Authenticate(username, password string) error {
	p.once.Do(p.init)

	if p.Builtin != nil {
		if v, ok := p.Builtin[username]; ok && v == password {
			return nil
		} else {
			err := errors.New("wrong username or password")
			glog.Warningf("SimpleAuth: builtin username=%v password=%v error: %+v", username, password, err)
			time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
			return err
		}
	}

	auth := p.Mode + ":" + username + ":" + password

	if _, ok := p.cache.GetNotStale(auth); ok {
		return nil
	}

	cmd := exec.Command(p.path, p.Mode)
	//glog.Infof("SimpleAuth exec cmd=%#v", cmd)
	cmd.Stdin = strings.NewReader(username + "\n" + password + "\n")
	err := cmd.Run()

	if err != nil {
		glog.Warningf("SimpleAuth: username=%v password=%v error: %+v", username, password, err)
		time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
		return err
	}

	p.cache.Set(auth, struct{}{}, time.Now().Add(2*time.Hour))
	return nil
}

type HTTPHandler struct {
	ServerNames []string
	Fallback    *url.URL
	Dial        func(network, address string) (net.Conn, error)
	*http.Transport
	*SimpleAuth
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

	var isProxyRequest bool = !helpers.ContainsString(h.ServerNames, req.URL.Hostname())

	if isProxyRequest && h.SimpleAuth != nil {
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

					if err := h.SimpleAuth.Authenticate(username, password); err != nil {
						http.Error(rw, "403 Forbidden", http.StatusForbidden)
						return
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

	if !isProxyRequest {
		if h.Fallback == nil {
			http.FileServer(http.Dir("/var/www/html")).ServeHTTP(rw, req)
			return
		}
		if h.Fallback.Scheme == "file" {
			http.FileServer(http.Dir(h.Fallback.Path)).ServeHTTP(rw, req)
			return
		}
		req.URL.Scheme = h.Fallback.Scheme
		req.URL.Scheme = h.Fallback.Scheme
		req.URL.Host = h.Fallback.Host
		if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			xff := req.Header.Get("X-Forwarded-For")
			if xff == "" {
				req.Header.Set("X-Forwarded-For", ip)
			} else {
				req.Header.Set("X-Forwarded-For", xff+", "+ip)
			}
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Real-IP", ip)
		}
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
	ServerNames  []string
	Fallback     *url.URL
	DisableProxy bool
	Dial         func(network, address string) (net.Conn, error)
	*http.Transport
	*SimpleAuth
}

func (h *HTTP2Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var err error

	reqHostname := req.Host
	if host, _, err := net.SplitHostPort(req.Host); err == nil {
		reqHostname = host
	}

	var h2 bool = req.ProtoMajor == 2 && req.ProtoMinor == 0
	var isProxyRequest bool = !helpers.ContainsString(h.ServerNames, reqHostname)

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

	if isProxyRequest && h.DisableProxy {
		http.Error(rw, "403 Forbidden", http.StatusForbidden)
		return
	}

	var username, password string
	if isProxyRequest && h.SimpleAuth != nil {
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
					username = parts[0]
					password = parts[1]

					if err := h.SimpleAuth.Authenticate(username, password); err != nil {
						http.Error(rw, "403 Forbidden", http.StatusForbidden)
						return
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

		glog.Infof("[%v 0x%04x %s] %s \"%s %s %s\" - -", req.TLS.ServerName, req.TLS.Version, username, req.RemoteAddr, req.Method, req.Host, req.Proto)

		dial := h.Dial
		if dial == nil {
			dial = h.Transport.Dial
		}

		conn, err := dial("tcp", net.JoinHostPort(host, port))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}

		var w io.Writer
		var r io.Reader

		if h2 {
			flusher, ok := rw.(http.Flusher)
			if !ok {
				http.Error(rw, fmt.Sprintf("%#v is not http.Flusher", rw), http.StatusBadGateway)
				return
			}

			rw.WriteHeader(http.StatusOK)
			flusher.Flush()

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

			io.WriteString(lconn, "HTTP/1.1 200 OK\r\n\r\n")
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

	glog.Infof("[%v 0x%04x %s] %s \"%s %s %s\" - -", req.TLS.ServerName, req.TLS.Version, username, req.RemoteAddr, req.Method, req.URL.String(), req.Proto)

	if req.URL.Scheme == "" {
		req.URL.Scheme = "http"
	}

	if h2 {
		req.ProtoMajor = 1
		req.ProtoMinor = 1
		req.Proto = "HTTP/1.1"
	}

	if !isProxyRequest && h.Fallback != nil {
		if h.Fallback.Scheme == "file" {
			http.FileServer(http.Dir(h.Fallback.Path)).ServeHTTP(rw, req)
			return
		}
		req.URL.Scheme = h.Fallback.Scheme
		req.URL.Scheme = h.Fallback.Scheme
		req.URL.Host = h.Fallback.Host
		if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			xff := req.Header.Get("X-Forwarded-For")
			if xff == "" {
				req.Header.Set("X-Forwarded-For", ip)
			} else {
				req.Header.Set("X-Forwarded-For", xff+", "+ip)
			}
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Real-IP", ip)
		}
		if req.TLS.Unique0RTTToken != nil {
			req.Header.Set("CF-0RTT-Unique", hex.EncodeToString(req.TLS.Unique0RTTToken))
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
	RejectNilSni bool
	Dial         func(network, addr string) (net.Conn, error)

	hosts  []string
	certs  map[string]*tls.Certificate
	cpools map[string]*x509.CertPool
	h2     map[string]struct{}
	ecc    *autocert.Manager
	rsa    *autocert.Manager
	cache  lrucache.Cache
	sni    map[string]string
}

func (cm *CertManager) Add(host string, certfile, keyfile string, pem string, cafile, capem string, h2 bool) error {
	var err error

	if cm.ecc == nil {
		cm.ecc = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache("ecc"),
			HostPolicy: cm.HostPolicy,
		}
	}

	if cm.rsa == nil {
		cm.rsa = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache("rsa"),
			HostPolicy: cm.HostPolicy,
			ForceRSA:   true,
		}
	}

	if cm.certs == nil {
		cm.certs = make(map[string]*tls.Certificate)
	}

	if cm.h2 == nil {
		cm.h2 = make(map[string]struct{})
	}

	if cm.cpools == nil {
		cm.cpools = make(map[string]*x509.CertPool)
	}

	if cm.cache == nil {
		cm.cache = lrucache.NewLRUCache(128)
	}

	switch {
	case pem != "":
		cert, err := tls.X509KeyPair([]byte(pem), []byte(pem))
		if err != nil {
			return err
		}
		cm.certs[host] = &cert
	case certfile != "" && keyfile != "":
		cert, err := tls.LoadX509KeyPair(certfile, keyfile)
		if err != nil {
			return err
		}
		cm.certs[host] = &cert
	default:
		cm.certs[host] = nil
	}

	var asn1Data []byte = []byte(capem)

	if cafile != "" {
		if asn1Data, err = ioutil.ReadFile(cafile); err != nil {
			glog.Fatalf("ioutil.ReadFile(%#v) error: %+v", cafile, err)
		}
	}

	if len(asn1Data) > 0 {
		cert, err := x509.ParseCertificate(asn1Data)
		if err != nil {
			return err
		}

		certPool := x509.NewCertPool()
		certPool.AddCert(cert)

		cm.cpools[host] = certPool
	}

	if h2 {
		cm.h2[host] = struct{}{}
	}

	cm.hosts = append(cm.hosts, host)

	return nil
}

func (cm *CertManager) AddSniProxy(serverNames []string, host string, port int) error {
	if cm.sni == nil {
		cm.sni = make(map[string]string)
	}

	portStr := "443"
	if port != 0 {
		portStr = strconv.Itoa(port)
	}
	for _, name := range serverNames {
		cm.sni[name] = net.JoinHostPort(host, portStr)
	}

	return nil
}

func (cm *CertManager) HostPolicy(_ context.Context, host string) error {
	if _, ok := cm.certs[host]; !ok {
		return errors.New("acme/autocert: host not configured")
	}
	return nil
}

func (cm *CertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, _ := cm.certs[hello.ServerName]
	if cert != nil {
		return cert, nil
	}

	if helpers.HasECCCiphers(hello.CipherSuites) {
		cert, err := cm.ecc.GetCertificate(hello)
		switch {
		case cert != nil:
			return cert, nil
		case err != nil && strings.HasSuffix(hello.ServerName, ".acme.invalid"):
			break
		default:
			return nil, err
		}
	}

	return cm.rsa.GetCertificate(hello)
}

func (cm *CertManager) Forward(hello *tls.ClientHelloInfo, addr string) (*tls.Config, error) {
	if addr[0] == ':' {
		addr = hello.ServerName + addr
	}

	remote, err := cm.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	b := new(bytes.Buffer)
	b.Write([]byte{0x16, 0x03, 0x01})
	binary.Write(b, binary.BigEndian, uint16(len(hello.Raw)))

	r := io.MultiReader(b, bytes.NewReader(hello.Raw), hello.Conn)

	glog.Infof("Sniproxy: forward %#v to %#v", hello.Conn, remote)
	go helpers.IOCopy(remote, r)
	helpers.IOCopy(hello.Conn, remote)

	remote.Close()
	hello.Conn.Close()

	return nil, io.EOF
}

func (cm *CertManager) GetConfigForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	if cm.sni != nil {
		if addr, ok := cm.sni[hello.ServerName]; ok {
			return cm.Forward(hello, addr)
		}
	}

	if hello.ServerName == "" {
		if cm.RejectNilSni {
			hello.Conn.Close()
			return nil, nil
		}
		hello.ServerName = cm.hosts[0]
	}

	hasECC := helpers.HasECCCiphers(hello.CipherSuites)

	cacheKey := hello.ServerName
	if !hasECC {
		cacheKey += ",rsa"
	}

	if v, ok := cm.cache.GetNotStale(cacheKey); ok {
		return v.(*tls.Config), nil
	}

	cert, err := cm.GetCertificate(hello)
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		MaxVersion:               tls.VersionTLS13,
		MinVersion:               tls.VersionTLS10,
		Certificates:             []tls.Certificate{*cert},
		Max0RTTDataSize:          100 * 1024,
		Accept0RTTData:           true,
		AllowShortHeaders:        true,
		PreferServerCipherSuites: true,
	}

	if p, ok := cm.cpools[hello.ServerName]; ok {
		config.ClientAuth = tls.RequireAndVerifyClientCert
		config.ClientCAs = p
	}

	if _, ok := cm.h2[hello.ServerName]; ok {
		config.NextProtos = []string{"h2", "http/1.1"}
	}

	cm.cache.Set(cacheKey, config, time.Now().Add(2*time.Hour))

	return config, nil
}

type Config struct {
	Default struct {
		LogLevel            int
		DaemonStderr        string
		RejectNilSni        bool
		DnsTtl              int
		IdleConnTimeout     int
		MaxIdleConnsPerHost int
	}
	HTTP2 []struct {
		DisableHttp2 bool

		Network string
		Listen  string

		ServerName []string

		Keyfile  string
		Certfile string
		PEM      string

		ClientAuthFile string
		ClientAuthPem  string

		ParentProxy string

		ProxyFallback string
		DisableProxy  bool

		ProxyAuthMethod  string
		ProxyBuiltinAuth map[string]string
	}
	HTTP struct {
		Network string
		Listen  string

		ServerName []string

		ParentProxy   string
		ProxyFallback string

		ProxyAuthMethod  string
		ProxyBuiltinAuth map[string]string
	}
	Sniproxy []struct {
		ServerName []string
		Host       string
		Port       int
	}
}

type Handler struct {
	ServerNames []string
	Handlers    map[string]http.Handler
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	handler, ok := h.Handlers[req.TLS.ServerName]
	if !ok {
		handler, ok = h.Handlers[h.ServerNames[0]]
		if !ok {
			http.Error(rw, "403 Forbidden", http.StatusForbidden)
			return
		}
	}
	handler.ServeHTTP(rw, req)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Println(version)
		return
	}

	helpers.SetFlagsIfAbsent(map[string]string{
		"logtostderr": "true",
		"v":           "2",
	})
	flag.Parse()

	filename := flag.Arg(0)

	var tomlData []byte
	var err error
	switch {
	case strings.HasPrefix(filename, "data:text/x-toml;"):
		parts := strings.SplitN(filename, ",", 2)
		switch parts[0] {
		case "data:text/x-toml;base64":
			tomlData, err = base64.StdEncoding.DecodeString(parts[1])
		case "data:text/x-toml;utf8":
			tomlData = []byte(parts[1])
		default:
			err = fmt.Errorf("Unkown data scheme: %#v", parts[0])
		}
		if err != nil {
			glog.Fatalf("Parse(%+v) error: %+v", parts[1], err)
		}
	case os.Getenv("GOPROXY_VPS_CONFIG_URL") != "":
		filename = os.Getenv("GOPROXY_VPS_CONFIG_URL")
		fallthrough
	case strings.HasPrefix(filename, "https://"):
		glog.Infof("http.Get(%+v) ...", filename)
		resp, err := http.Get(filename)
		if err != nil {
			glog.Fatalf("http.Get(%+v) error: %+v", filename, err)
		}
		defer resp.Body.Close()
		tomlData, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Fatalf("ioutil.ReadAll(%+v) error: %+v", resp.Body, err)
		}
	case filename == "":
		if _, err := os.Stat("goproxy-vps.user.toml"); err == nil {
			filename = "goproxy-vps.user.toml"
		} else {
			filename = "goproxy-vps.toml"
		}
		fallthrough
	default:
		tomlData, err = ioutil.ReadFile(filename)
		if err != nil {
			glog.Fatalf("ioutil.ReadFile(%+v) error: %+v", filename, err)
		}
	}

	tomlData = bytes.Replace(tomlData, []byte("\r\n"), []byte("\n"), -1)
	tomlData = bytes.Replace(tomlData, []byte("\n[[https]]"), []byte("\n[[http2]]\ndisable_http2=true"), -1)

	var config Config
	if err = toml.Unmarshal(tomlData, &config); err != nil {
		glog.Fatalf("toml.Decode(%s) error: %+v\n", tomlData, err)
	}

	dialer := &helpers.Dialer{
		Dialer: &net.Dialer{
			KeepAlive: 0,
			Timeout:   16 * time.Second,
			DualStack: true,
		},
		Resolver: &helpers.Resolver{
			LRUCache:  lrucache.NewLRUCache(8 * 1024),
			BlackList: lrucache.NewLRUCache(1024),
			DNSExpiry: time.Duration(config.Default.DnsTtl) * time.Second,
		},
	}

	for _, s := range []string{"127.0.0.1", "::1"} {
		dialer.Resolver.BlackList.Set(s, struct{}{}, time.Time{})
	}

	transport := &http.Transport{
		Dial: dialer.Dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(1024),
		},
		TLSHandshakeTimeout: 16 * time.Second,
		IdleConnTimeout:     180 * time.Second,
		MaxIdleConnsPerHost: 8,
		DisableCompression:  false,
	}

	if config.Default.IdleConnTimeout > 0 {
		transport.IdleConnTimeout = time.Duration(config.Default.IdleConnTimeout) * time.Second
	}

	if config.Default.MaxIdleConnsPerHost > 0 {
		transport.MaxIdleConnsPerHost = config.Default.MaxIdleConnsPerHost
	}

	cm := &CertManager{
		RejectNilSni: config.Default.RejectNilSni,
		Dial:         dialer.Dial,
	}
	h := &Handler{
		Handlers:    map[string]http.Handler{},
		ServerNames: []string{},
	}
	for _, server := range config.HTTP2 {
		handler := &HTTP2Handler{
			ServerNames: server.ServerName,
			Transport:   transport,
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
				handler.Transport.Dial = dialer2.Dial
			}
		}

		switch server.ProxyAuthMethod {
		case "pam", "htpasswd":
			if _, err := exec.LookPath("python"); err != nil {
				glog.Fatalf("pam: exec.LookPath(\"python\") error: %+v", err)
			}
			handler.SimpleAuth = &SimpleAuth{
				Mode:      server.ProxyAuthMethod,
				CacheSize: 2048,
			}
		case "builtin":
			handler.SimpleAuth = &SimpleAuth{
				Mode:    server.ProxyAuthMethod,
				Builtin: server.ProxyBuiltinAuth,
			}
		case "":
			break
		default:
			glog.Fatalf("unsupport proxy_auth_method(%+v)", server.ProxyAuthMethod)
		}

		for _, servername := range server.ServerName {
			cm.Add(servername, server.Certfile, server.Keyfile, server.PEM, server.ClientAuthFile, server.ClientAuthPem, !server.DisableHttp2)
			h.ServerNames = append(h.ServerNames, servername)
			h.Handlers[servername] = handler
		}
	}

	for _, server := range config.Sniproxy {
		cm.AddSniProxy(server.ServerName, server.Host, server.Port)
	}

	srv := &http.Server{
		Handler: h,
		TLSConfig: &tls.Config{
			GetConfigForClient: cm.GetConfigForClient,
		},
	}

	http2.ConfigureServer(srv, &http2.Server{})

	seen := make(map[string]struct{})
	for _, server := range config.HTTP2 {
		network := server.Network
		if network == "" {
			network = "tcp"
		}
		addr := server.Listen
		if _, ok := seen[network+":"+addr]; ok {
			continue
		}
		seen[network+":"+addr] = struct{}{}
		ln, err := net.Listen(network, addr)
		if err != nil {
			glog.Fatalf("Listen(%s) error: %s", addr, err)
		}
		glog.Infof("goproxy-vps %s ListenAndServe on %s\n", version, ln.Addr().String())
		go srv.Serve(tls.NewListener(TCPListener{ln.(*net.TCPListener)}, srv.TLSConfig))
	}

	if config.HTTP.Listen != "" {
		server := config.HTTP
		network := server.Network
		if network == "" {
			network = "tcp"
		}
		addr := server.Listen
		if _, ok := seen[network+":"+addr]; ok {
			glog.Fatalf("goproxy-vps: addr(%#v) already listened by http2", addr)
		}

		ln, err := net.Listen(network, addr)
		if err != nil {
			glog.Fatalf("Listen(%s) error: %s", addr, err)
		}

		handler := &HTTPHandler{
			ServerNames: config.HTTP.ServerName,
			Transport:   transport,
		}

		if server.ProxyFallback != "" {
			handler.Fallback, err = url.Parse(server.ProxyFallback)
			if err != nil {
				glog.Fatalf("url.Parse(%+v) error: %+v", server.ProxyFallback, err)
			}
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
				handler.Transport.Dial = dialer2.Dial
			}
		}

		switch server.ProxyAuthMethod {
		case "pam":
			if _, err := exec.LookPath("python"); err != nil {
				glog.Fatalf("pam: exec.LookPath(\"python\") error: %+v", err)
			}
			handler.SimpleAuth = &SimpleAuth{
				CacheSize: 2048,
			}
		case "builtin":
			handler.SimpleAuth = &SimpleAuth{
				Mode:    server.ProxyAuthMethod,
				Builtin: server.ProxyBuiltinAuth,
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
