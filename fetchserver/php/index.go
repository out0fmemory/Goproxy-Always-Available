package main

import (
	"bufio"
	"compress/flate"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	Version  = "1.0"
	Password = "123456"
)

func main() {
	http.HandleFunc("/", handler)
	err := http.ListenAndServe("0.0.0.0:"+os.Getenv("PORT"), nil)
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
			err = fmt.Errorf("Invaild Request Line: %#v", line)
			return
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		req.Header.Add(key, value)
	}

	if err = scanner.Err(); err != nil {
		return
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
	delete(req.Header, "Host")

	return
}

func httpError(rw http.ResponseWriter, err string, code int) {
	rw.Header().Set("Content-Length", strconv.Itoa(len(err)))
	rw.Header().Set("Connection", "close")
	http.Error(rw, err, http.StatusBadRequest)
}

func handler(rw http.ResponseWriter, req *http.Request) {
	var err error

	logger := log.New(os.Stderr, "index.go: ", 0)

	var hdrLen uint16
	if err := binary.Read(req.Body, binary.BigEndian, &hdrLen); err != nil {
		httpError(rw, err.Error(), http.StatusBadRequest)
		return
	}

	req1, err := ReadRequest(flate.NewReader(&io.LimitedReader{R: req.Body, N: int64(hdrLen)}))
	req1.Body = req.Body

	logger.Printf("%s \"%s %s %s\" - -", req.RemoteAddr, req1.Method, req1.URL.String(), req1.Proto)

	const PasswordKey string = "X-UrlFetch-Password"
	if Password != "" {
		if password := req1.Header.Get(PasswordKey); password != Password {
			httpError(rw, fmt.Sprintf("wrong password %#v", password), http.StatusForbidden)
			return
		}
	}
	req1.Header.Del(PasswordKey)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		TLSHandshakeTimeout: 30 * time.Second,
	}

	resp, err := tr.RoundTrip(req1)
	if err != nil {
		httpError(rw, err.Error(), http.StatusBadGateway)
		return
	}

	// rewise resp.Header
	resp.Header.Del("Transfer-Encoding")
	if resp.ContentLength > 0 {
		resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}

	var w io.Writer = rw
	switch strings.Split(resp.Header.Get("Content-Type"), "/")[0] {
	case "image", "audio", "video":
		rw.Header().Set("Content-Type", "image/gif")
		w = newXorWriter(rw, []byte(Password))
	default:
		rw.Header().Set("Content-Type", "image/x-png")
	}

	rw.WriteHeader(http.StatusOK)

	fmt.Fprintf(w, fmt.Sprintf("%s %s\r\n", resp.Proto, resp.Status))
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
