package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/phuslu/http2"
)

const (
	Version = "@VERSION@"
)

var (
	transport *http.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		},
		TLSHandshakeTimeout: 30 * time.Second,
		MaxIdleConnsPerHost: 4,
		DisableCompression:  false,
	}
)

type listener struct {
	net.Listener
}

func (l *listener) Accept() (c net.Conn, err error) {
	c, err = l.Listener.Accept()
	if err != nil {
		return
	}

	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(3 * time.Minute)
	}

	return
}

func genHostname() (hostname string, err error) {
	var length uint16
	if err = binary.Read(rand.Reader, binary.BigEndian, &length); err != nil {
		return
	}

	buf := make([]byte, 5+length%7)
	for i := 0; i < len(buf); i++ {
		var c uint8
		if err = binary.Read(rand.Reader, binary.BigEndian, &c); err != nil {
			return
		}
		buf[i] = 'a' + c%('z'-'a')
	}

	return fmt.Sprintf("www.%s.com", buf), nil
}

type RandomCertificate struct {
	mu    sync.Mutex
	mtime time.Time
	cert  *tls.Certificate
}

func (rc *RandomCertificate) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if rc.mtime.IsZero() || time.Now().Sub(rc.mtime) < 2*time.Hour {
		if rc.cert != nil {
			return rc.cert, nil
		}
	}

	rc.mu.Lock()
	defer rc.mu.Unlock()

	name := "www.gov.cn"
	if name1, err := genHostname(); err == nil {
		name = name1
	}

	glog.Infof("Generating RootCA for %v", name)
	template := x509.Certificate{
		IsCA:         true,
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{name},
		},
		NotBefore: time.Now().Add(-time.Duration(5 * time.Minute)),
		NotAfter:  time.Now().Add(180 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)

	rc.mtime = time.Now()
	rc.cert = &cert

	return rc.cert, err
}

type ProxyHandler struct {
	AuthMap map[string]string
}

func (p *ProxyHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var err error

	var paramsPreifx string = http.CanonicalHeaderKey("X-UrlFetch-")
	params := map[string]string{}
	for key, values := range req.Header {
		if strings.HasPrefix(key, paramsPreifx) {
			params[strings.ToLower(key[len(paramsPreifx):])] = values[0]
		}
	}

	for _, key := range params {
		req.Header.Del(paramsPreifx + key)
	}

	if false && p.AuthMap != nil {
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
					if pass1, ok := p.AuthMap[user]; !ok || pass != pass1 {
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

	if req.Method == "CONNECT" {
		host, port, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
			port = "443"
		}

		glog.Infof("%s \"%s %s:%s %s\" - -", req.RemoteAddr, req.Method, host, port, req.Proto)

		conn, err := net.Dial("tcp", net.JoinHostPort(host, port))
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

func main() {
	var err error

	addr := ":443"
	auth := `test:123456 foobar:123456`
	keyFile := "govps.key"
	certFile := "govps.crt"
	verbose := false
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

	flag.StringVar(&addr, "addr", addr, "goproxy vps listen addr")
	flag.StringVar(&keyFile, "keyFile", keyFile, "goproxy vps keyFile")
	flag.StringVar(&certFile, "certFile", certFile, "goproxy vps certFile")
	flag.BoolVar(&verbose, "verbose", verbose, "goproxy vps http2 verbose mode")
	flag.StringVar(&auth, "auth", auth, "goproxy vps auth user:pass list")
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

	var certs []tls.Certificate = nil
	if _, err := os.Stat(keyFile); err == nil {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			glog.Fatalf("LoadX509KeyPair(%#v, %#v) error: %v", certFile, keyFile, err)
		}
		certs = []tls.Certificate{cert}
	}

	srv := &http.Server{
		Handler: &ProxyHandler{authMap},
		TLSConfig: &tls.Config{
			Certificates: certs,
			MinVersion:   tls.VersionTLS12,
		},
	}

	if srv.TLSConfig.Certificates == nil {
		rc := &RandomCertificate{}
		srv.TLSConfig.GetCertificate = rc.GetCertificate
	}

	if verbose {
		http2.VerboseLogs = true
	}
	http2.ConfigureServer(srv, &http2.Server{})
	glog.Infof("goproxy %s ListenAndServe on %s\n", Version, ln.Addr().String())
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
