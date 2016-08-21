// https://git.io/goproxy

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/net/http2"
)

var (
	version = "r9999"
)

type Handler struct {
	AuthMap map[string]string
	*http.Transport
}

type flushWriter struct {
	w io.Writer
}

func (fw flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
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

	if len(h.AuthMap) > 0 {
		auth := req.Header.Get("Proxy-Authorization")
		if auth == "" {
			http.Error(rw, "403 Forbidden", http.StatusForbidden)
			return
		}
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "Basic":
				if userpass, err := base64.StdEncoding.DecodeString(parts[1]); err == nil {
					parts := strings.Split(string(userpass), ":")
					user := parts[0]
					pass := parts[1]
					glog.Infof("username=%v password=%v", user, pass)
					if pass1, ok := h.AuthMap[user]; !ok || pass != pass1 {
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
			w = flushWriter{rw}
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

		go io.Copy(conn, r)
		io.Copy(w, conn)

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
	io.Copy(rw, resp.Body)
}

func SetFlagsIfAbsent(m map[string]string) {
	seen := map[string]struct{}{}

	for i := 1; i < len(os.Args); i++ {
		for key := range m {
			if strings.HasPrefix(os.Args[i], "-"+key+"=") {
				seen[key] = struct{}{}
			}
		}
	}

	for key, value := range m {
		if _, ok := seen[key]; !ok {
			flag.Set(key, value)
		}
	}
}

func genCertificates(hostname string) (tls.Certificate, error) {
	if hostname == "" {
		return tls.Certificate{}, fmt.Errorf("genCertificates: Invalid hostname(%#v)", hostname)
	}

	glog.V(2).Infof("Generating RootCA for %v", hostname)
	template := x509.Certificate{
		IsCA:         true,
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{hostname},
		},
		NotBefore: time.Now().Add(-time.Duration(24 * time.Hour)),
		NotAfter:  time.Now().Add(180 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return tls.Certificate{}, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return tls.X509KeyPair(certPEMBlock, keyPEMBlock)
}

func main() {
	var err error

	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Print(version)
		return
	}

	addr := ":443"
	auth := ""
	keyFile := "goproxy-vps.key"
	certFile := "goproxy-vps.crt"
	http2verbose := false

	flag.StringVar(&addr, "addr", addr, "goproxy vps listen addr")
	flag.StringVar(&keyFile, "keyfile", keyFile, "goproxy vps keyfile")
	flag.StringVar(&certFile, "certfile", certFile, "goproxy vps certfile")
	flag.BoolVar(&http2verbose, "http2verbose", http2verbose, "goproxy vps http2 verbose mode")
	flag.StringVar(&auth, "auth", auth, "goproxy vps auth user:pass list")

	SetFlagsIfAbsent(map[string]string{
		"logtostderr": "true",
		"v":           "2",
	})
	flag.Parse()

	authMap := map[string]string{}
	for _, pair := range strings.Split(auth, " ") {
		parts := strings.Split(pair, ":")
		if len(parts) == 2 {
			username := strings.TrimSpace(parts[0])
			password := strings.TrimSpace(parts[1])
			authMap[username] = password
		}
	}

	var ln net.Listener
	ln, err = net.Listen("tcp", addr)
	if err != nil {
		glog.Fatalf("Listen(%s) error: %s", addr, err)
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		glog.Warningf("LoadX509KeyPair(%#v, %#v) error: %v", certFile, keyFile, err)
		hostname := "www.apple.com"
		cert, err = genCertificates(hostname)
		if err != nil {
			glog.Fatalf("genCertificates(%#v) error: %+v", hostname, err)
		} else {
			glog.V(2).Infof("genCertificates(%#v) OK", hostname)
		}
	}

	dialer := &Dialer{
		Dialer: &net.Dialer{
			KeepAlive: 0,
			Timeout:   16 * time.Second,
			DualStack: true,
		},
		DNSCache:       lrucache.NewLRUCache(16 * 1024),
		DNSCacheExpiry: 8 * time.Hour,
		BlackList:      lrucache.NewLRUCache(8 * 1024),
	}

	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			addr1 := addr.String()
			switch addr.Network() {
			case "ip+net":
				addr1 = strings.Split(addr1, "/")[0]
			}
			dialer.BlackList.Set(addr1, struct{}{}, time.Time{})
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

	srv := &http.Server{
		Handler: &Handler{
			AuthMap:   authMap,
			Transport: transport,
		},
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	http2.VerboseLogs = http2verbose
	http2.ConfigureServer(srv, &http2.Server{})
	glog.Infof("goproxy-vps %s ListenAndServe on %s\n", version, ln.Addr().String())
	srv.Serve(tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, srv.TLSConfig))
}
