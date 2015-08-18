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
	"time"

	"github.com/bradfitz/http2"
	// "github.com/cloudflare/golibs/lrucache"
	"github.com/golang/glog"
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

func getCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	// name := clientHello.ServerName
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
	return &cert, err
}

func handler(rw http.ResponseWriter, req *http.Request) {
	var err error

	glog.Infof("%s \"%s %s %s\" - -", req.RemoteAddr, req.Method, req.URL.String(), req.Proto)

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

	if auth := req.Header.Get("Proxy-Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 {
			switch parts[0] {
			case "Basic":
				if userpass, err := base64.StdEncoding.DecodeString(parts[1]); err == nil {
					parts := strings.Split(string(userpass), ":")
					user := parts[0]
					pass := parts[1]
					glog.Infof("username=%v password=%v", user, pass)
				}
			default:
				glog.Errorf("Unrecognized auth type: %#v", parts[0])
				break
			}
		}
		req.Header.Del("Proxy-Authorization")
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

func main() {
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

	addr := *flag.String("addr", ":443", "goproxy vps listen addr")
	flag.Parse()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		glog.Fatalf("Listen(%s) error: %s", addr, err)
	}

	cert, err := getCertificate(nil)
	if err != nil {
		glog.Fatalf("getCertificate error: %s", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		// GetCertificate: getCertificate,
	}

	s := &http.Server{
		Handler:   http.HandlerFunc(handler),
		TLSConfig: tlsConfig,
	}

	http2.VerboseLogs = true
	http2.ConfigureServer(s, &http2.Server{})
	glog.Infof("ListenAndServe on %s\n", ln.Addr().String())
	s.Serve(tls.NewListener(ln, tlsConfig))
}
