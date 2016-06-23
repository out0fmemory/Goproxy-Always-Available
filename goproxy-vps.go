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

var version = "r9999"

type Dialer struct {
	net.Dialer

	RetryTimes     int
	RetryDelay     time.Duration
	DNSCache       lrucache.Cache
	DNSCacheExpiry time.Duration
	LoopbackAddrs  map[string]struct{}
	Level          int
}

func (d *Dialer) Dial(network, address string) (conn net.Conn, err error) {
	glog.V(3).Infof("Dail(%#v, %#v)", network, address)

	switch network {
	case "tcp", "tcp4", "tcp6":
		if d.DNSCache != nil {
			if addr, ok := d.DNSCache.Get(address); ok {
				address = addr.(string)
			} else {
				if host, port, err := net.SplitHostPort(address); err == nil {
					if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
						ip := ips[0].String()
						if d.LoopbackAddrs != nil {
							if _, ok := d.LoopbackAddrs[ip]; ok {
								return nil, net.InvalidAddrError(fmt.Sprintf("Invaid DNS Record: %s(%s)", host, ip))
							}
						}
						addr := net.JoinHostPort(ip, port)
						d.DNSCache.Set(address, addr, time.Now().Add(d.DNSCacheExpiry))
						glog.V(3).Infof("direct Dial cache dns %#v=%#v", address, addr)
						address = addr
					}
				}
			}
		}
	default:
		break
	}

	if d.Level <= 1 {
		retry := d.RetryTimes
		for i := 0; i < retry; i++ {
			conn, err = d.Dialer.Dial(network, address)
			if err == nil || i == retry-1 {
				break
			}
			time.Sleep(d.RetryDelay)
		}
		return conn, err
	} else {
		type racer struct {
			c net.Conn
			e error
		}

		lane := make(chan racer, d.Level)
		retry := (d.RetryTimes + d.Level - 1) / d.Level
		for i := 0; i < retry; i++ {
			for j := 0; j < d.Level; j++ {
				go func(addr string, c chan<- racer) {
					conn, err := d.Dialer.Dial(network, addr)
					lane <- racer{conn, err}
				}(address, lane)
			}

			var r racer
			for k := 0; k < d.Level; k++ {
				r = <-lane
				if r.e == nil {
					go func(count int) {
						var r1 racer
						for ; count > 0; count-- {
							r1 = <-lane
							if r1.c != nil {
								r1.c.Close()
							}
						}
					}(d.Level - 1 - k)
					return r.c, nil
				}
			}

			if i == retry-1 {
				return nil, r.e
			}
		}
	}

	return nil, net.UnknownNetworkError("Unkown transport/direct error")
}

var (
	dialer *Dialer = &Dialer{
		Dialer: net.Dialer{
			KeepAlive: 0,
			Timeout:   0,
			DualStack: true,
		},
		RetryTimes:     2,
		RetryDelay:     100 * time.Millisecond,
		DNSCache:       lrucache.NewLRUCache(8 * 1024),
		DNSCacheExpiry: 1 * time.Hour,
		LoopbackAddrs:  make(map[string]struct{}),
	}

	transport *http.Transport = &http.Transport{
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
)

type Handler struct {
	AuthMap map[string]string
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

	for key, _ := range params {
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

		conn, err := transport.Dial("tcp", net.JoinHostPort(host, port))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}

		hijacker, ok := rw.(http.Hijacker)
		if !ok {
			http.Error(rw, fmt.Sprintf("%#v is not http.Hijacker", rw), http.StatusBadGateway)
			return
		}

		flusher, ok := rw.(http.Flusher)
		if !ok {
			http.Error(rw, fmt.Sprintf("%#v is not http.Flusher", rw), http.StatusBadGateway)
			return
		}

		rw.WriteHeader(http.StatusOK)
		flusher.Flush()

		lconn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}
		defer lconn.Close()

		go io.Copy(conn, lconn)
		io.Copy(lconn, conn)

		return
	}

	glog.Infof("%s \"%s %s %s\" - -", req.RemoteAddr, req.Method, req.URL.String(), req.Proto)

	if req.Host == "" {
		http.Error(rw, "403 Forbidden", http.StatusForbidden)
		return
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
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

func hint(v map[string]string) {
	seen := map[string]struct{}{}

	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-version" {
			fmt.Print(version)
			os.Exit(0)
		}

		for key, _ := range v {
			if strings.HasPrefix(os.Args[i], "-"+key+"=") {
				seen[key] = struct{}{}
			}
		}
	}

	for key, value := range v {
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
	keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return tls.X509KeyPair(certPEMBlock, keyPEMBlock)
}

func main() {
	var err error

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

	hint(map[string]string{
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

	srv := &http.Server{
		Handler: &Handler{authMap},
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	http2.VerboseLogs = http2verbose
	http2.ConfigureServer(srv, &http2.Server{})
	glog.Infof("goproxy %s ListenAndServe on %s\n", version, ln.Addr().String())
	srv.Serve(tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, srv.TLSConfig))
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
