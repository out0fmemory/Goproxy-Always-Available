package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/goproxy/httpproxy/helpers"
	"github.com/phuslu/goproxy/httpproxy/proxy"
	"github.com/phuslu/net/http2"
	"github.com/phuslu/quic-go/h2quic"
)

var (
	version = "r9999"
)

func main() {
	var err error

	rand.Seed(time.Now().UnixNano())

	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Println(version)
		return
	}

	helpers.SetFlagsIfAbsent(map[string]string{
		"logtostderr": "true",
		"v":           "2",
	})
	flag.Parse()

	config, err := NewConfig(flag.Arg(0))
	if err != nil {
		glog.Fatalf("NewConfig(%#v) error: %+v", flag.Arg(0), err)
	}

	dialer := &helpers.Dialer{
		Dialer: &net.Dialer{
			KeepAlive: 0,
			Timeout:   16 * time.Second,
			DualStack: !config.Default.PreferIpv4,
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

		for i, servername := range server.ServerName {
			cm.Add(servername, server.Certfile, server.Keyfile, server.PEM, server.ClientAuthFile, server.ClientAuthPem, !server.DisableHttp2, server.DisableLegacySsl)
			h.ServerNames = append(h.ServerNames, servername)
			h.Handlers[servername] = handler
			if i == 0 || servername == "*" {
				glog.V(3).Infof("Set handler=%#v as default HTTP/2 handler", handler)
				h.Default = handler
			}
		}
	}

	for _, server := range config.TLS {
		cm.AddTLSProxy(server.ServerName, server.Backend, server.Terminate)
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
			glog.Fatalf("TLS Listen(%s) error: %s", addr, err)
		}
		glog.Infof("goproxy-vps %s ListenAndServe on %s\n", version, ln.Addr().String())
		go srv.Serve(tls.NewListener(TCPListener{ln.(*net.TCPListener)}, srv.TLSConfig))

		if uaddr, err := net.ResolveUDPAddr("udp", addr); err == nil {
			if conn, err := net.ListenUDP("udp", uaddr); err == nil {
				go (&h2quic.Server{Server: srv}).Serve(conn)
			}
		}
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
			Transport: transport,
		}

		if server.ProxyFallback != "" {
			handler.Fallback, err = url.Parse(server.ProxyFallback)
			if err != nil {
				glog.Fatalf("url.Parse(%+v) error: %+v", server.ProxyFallback, err)
			}
		}

		if server.DisableProxy {
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
