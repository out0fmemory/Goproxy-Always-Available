package main

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	Version = "1.0"
)

var Password = func() string {
	if s := os.Getenv("PASSWORD"); s != "" {
		return s
	} else {
		return "123456"
	}
}()

var (
	secureTransport *http.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
			ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		},
		TLSHandshakeTimeout: 30 * time.Second,
		MaxIdleConnsPerHost: 4,
		DisableCompression:  false,
	}

	insecureTransport *http.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		},
		TLSHandshakeTimeout: 30 * time.Second,
		MaxIdleConnsPerHost: 4,
		DisableCompression:  false,
	}
)

func main() {
	http.HandleFunc("/", handler)

	parts := []string{"", "8080"}

	for i, keys := range [][]string{{"VCAP_APP_HOST", "HOST"}, {"VCAP_APP_PORT", "PORT"}} {
		for _, key := range keys {
			if s := os.Getenv(key); s != "" {
				parts[i] = s
			}
		}
	}

	addr := strings.Join(parts, ":")
	fmt.Fprintf(os.Stdout, "Start ListenAndServe on %v\n", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		panic(err)
	}
}

func ReadRequest(r io.Reader) (req *http.Request, err error) {
	req = new(http.Request)

	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) != 3 {
			err = fmt.Errorf("Invaild Request Line: %#v", line)
			return
		}

		req.Method = parts[0]
		req.RequestURI = parts[1]
		req.Proto = "HTTP/1.1"
		req.ProtoMajor = 1
		req.ProtoMinor = 1

		if req.URL, err = url.Parse(req.RequestURI); err != nil {
			return
		}
		req.Host = req.URL.Host

		req.Header = http.Header{}
	}

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		req.Header.Add(key, value)
	}

	if err = scanner.Err(); err != nil {
		// ignore
	}

	if cl := req.Header.Get("Content-Length"); cl != "" {
		if req.ContentLength, err = strconv.ParseInt(cl, 10, 64); err != nil {
			return
		}
	}

	req.Host = req.URL.Host
	if req.Host == "" {
		req.Host = req.Header.Get("Host")
	}

	return
}

func httpError(rw http.ResponseWriter, err string, code int) {
	rw.WriteHeader(http.StatusOK)
	fmt.Fprintf(rw, "HTTP/1.1 %d\r\n", code)
	fmt.Fprintf(rw, "Content-Length: %d\r\n", len(err))
	fmt.Fprintf(rw, "Content-Type: text/plain\r\n")
	io.WriteString(rw, "\r\n")
	io.WriteString(rw, err)
}

func handler(rw http.ResponseWriter, req *http.Request) {
	var err error

	logger := log.New(os.Stdout, "index.go: ", 0)

	var hdrLen uint16
	if err := binary.Read(req.Body, binary.BigEndian, &hdrLen); err != nil {
		parts := strings.Split(req.Host, ".")
		switch len(parts) {
		case 1, 2:
			httpError(rw, "fetchserver:"+err.Error(), http.StatusBadRequest)
		default:
			u := *req.URL
			if u.Scheme == "" {
				u.Scheme = "http"
			}
			u.Host = fmt.Sprintf("phuslu-%d.%s", time.Now().Nanosecond(), strings.Join(parts[1:], "."))
			if resp, err := http.Get(u.String()); err == nil {
				defer resp.Body.Close()
				for key, values := range resp.Header {
					for _, value := range values {
						rw.Header().Add(key, value)
					}
				}
				rw.WriteHeader(resp.StatusCode)
				io.Copy(rw, resp.Body)
			} else {
				u.Host = "www." + strings.Join(parts[1:], ".")
				http.Redirect(rw, req, u.String(), 301)
			}
		}
		return
	}

	req1, err := ReadRequest(flate.NewReader(&io.LimitedReader{R: req.Body, N: int64(hdrLen)}))
	req1.Body = req.Body

	if ce := req1.Header.Get("Content-Encoding"); ce != "" {
		var r io.Reader
		switch ce {
		case "deflate":
			r = flate.NewReader(req1.Body)
		case "gzip":
			if r, err = gzip.NewReader(req1.Body); err != nil {
				httpError(rw, "fetchserver:"+err.Error(), http.StatusBadRequest)
				return
			}
		default:
			httpError(rw, "fetchserver:"+fmt.Sprintf("Unsupported Content-Encoding: %#v", ce), http.StatusBadRequest)
			return
		}
		data, err := ioutil.ReadAll(r)
		if err != nil {
			req1.Body.Close()
			httpError(rw, "fetchserver:"+err.Error(), http.StatusBadRequest)
			return
		}
		req1.Body.Close()
		req1.Body = ioutil.NopCloser(bytes.NewReader(data))
		req1.ContentLength = int64(len(data))
		req1.Header.Set("Content-Length", strconv.FormatInt(req1.ContentLength, 10))
		req1.Header.Del("Content-Encoding")
	}

	logger.Printf("%s \"%s %s %s\" - -", req.RemoteAddr, req1.Method, req1.URL.String(), req1.Proto)

	var paramsPreifx string = http.CanonicalHeaderKey("X-UrlFetch-")
	params := map[string]string{}
	for key, values := range req1.Header {
		if strings.HasPrefix(key, paramsPreifx) {
			params[strings.ToLower(key[len(paramsPreifx):])] = values[0]
		}
	}

	for key := range params {
		req1.Header.Del(paramsPreifx + key)
	}

	if Password != "" {
		if password, ok := params["password"]; !ok || password != Password {
			httpError(rw, fmt.Sprintf("fetchserver: wrong password %#v", password), http.StatusForbidden)
			return
		}
	}

	transport := insecureTransport
	if v, ok := params["sslverify"]; ok && v == "1" {
		transport = secureTransport
	}

	var resp *http.Response
	for i := 0; i < 2; i++ {
		resp, err = transport.RoundTrip(req1)
		if err == nil {
			break
		}

		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		if err1, ok := err.(interface {
			Temporary() bool
		}); ok && err1.Temporary() {
			time.Sleep(1 * time.Second)
			continue
		}

		httpError(rw, "fetchserver:"+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// rewise resp.Header
	resp.Header.Del("Transfer-Encoding")
	if resp.ContentLength > 0 {
		resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}

	needCrypto := false
	if v, ok := params["https"]; !ok || v == "0" {
		switch strings.Split(resp.Header.Get("Content-Type"), "/")[0] {
		case "image", "audio", "video":
			break
		default:
			needCrypto = true
		}
	}

	var w io.Writer = rw
	if needCrypto {
		rw.Header().Set("Content-Type", "image/gif")
		w = newXorWriter(rw, []byte(Password))
	} else {
		rw.Header().Set("Content-Type", "image/x-png")
	}

	rw.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, "%s %s\r\n", resp.Proto, resp.Status)
	resp.Header.Write(w)
	io.WriteString(w, "\r\n")
	io.Copy(w, resp.Body)
}

type xorWriter struct {
	w   io.Writer
	key []byte
}

func newXorWriter(w io.Writer, key []byte) io.Writer {
	x := new(xorWriter)
	x.w = w
	x.key = key
	return x
}

func (x *xorWriter) Write(p []byte) (n int, err error) {
	c := x.key[0]
	for i := 0; i < len(p); i++ {
		p[i] ^= c
	}

	return x.w.Write(p)
}
