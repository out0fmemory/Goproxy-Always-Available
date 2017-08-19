package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/phuslu/glog"
	"github.com/phuslu/goproxy/httpproxy/helpers"
)

type HTTPHandler struct {
	Fallback     *url.URL
	DisableProxy bool
	Dial         func(network, address string) (net.Conn, error)
	*http.Transport
	*SimpleAuth
}

func (h *HTTPHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var err error

	if !h.DisableProxy && h.SimpleAuth != nil {
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

	if h.DisableProxy {
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
