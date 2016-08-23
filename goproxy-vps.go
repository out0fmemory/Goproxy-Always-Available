// https://git.io/goproxy

package main

import (
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/net/http2"
)

var (
	version = "r9999"
)

const (
	PWAUTH_PATH = "/usr/sbin/pwauth"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Handler struct {
	PWAuthEnabled bool
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
			http.Error(rw, "407 Proxy Authentication Required", http.StatusProxyAuthRequired)
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
					glog.Infof("pwauth: username=%v password=%v", username, password)

					cmd := exec.Command(PWAUTH_PATH)
					cmd.Stdin = strings.NewReader(username + "\n" + password + "\n")
					err = cmd.Run()

					if err != nil {
						time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
						http.Error(rw, "407 Proxy Authentication Required", http.StatusProxyAuthRequired)
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

func main() {
	var err error

	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Print(version)
		return
	}

	addr := ":443"
	pwauth := false
	keyFile := "goproxy-vps.key"
	certFile := "goproxy-vps.crt"
	http2verbose := false

	flag.StringVar(&addr, "addr", addr, "goproxy vps listen addr")
	flag.StringVar(&keyFile, "keyfile", keyFile, "goproxy vps keyfile")
	flag.StringVar(&certFile, "certfile", certFile, "goproxy vps certfile")
	flag.BoolVar(&http2verbose, "http2verbose", http2verbose, "goproxy vps http2 verbose mode")
	flag.BoolVar(&pwauth, "pwauth", pwauth, "goproxy vps enable pwauth")

	SetFlagsIfAbsent(map[string]string{
		"logtostderr": "true",
		"v":           "2",
	})
	flag.Parse()

	if pwauth {
		if _, err := os.Stat(PWAUTH_PATH); err != nil {
			glog.Fatalf("Find %+v error: %+v, please install pwauth", PWAUTH_PATH, err)
		}
	}

	var ln net.Listener
	ln, err = net.Listen("tcp", addr)
	if err != nil {
		glog.Fatalf("Listen(%s) error: %s", addr, err)
	}

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

	if ips, err := LocalInterfaceIPs(); err == nil {
		for _, ip := range ips {
			dialer.BlackList.Set(ip.String(), struct{}{}, time.Time{})
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
			PWAuthEnabled: pwauth,
			Transport:     transport,
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
