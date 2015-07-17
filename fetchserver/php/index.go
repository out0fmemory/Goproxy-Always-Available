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

func handler(rw http.ResponseWriter, r *http.Request) {
	var err error

	logger := log.New(os.Stderr, "index.go: ", 0)

	var hdrLen uint16
	if err := binary.Read(r.Body, binary.BigEndian, &hdrLen); err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	req, err := http.ReadRequest(bufio.NewReader(flate.NewReader(&io.LimitedReader{R: r.Body, N: int64(hdrLen)})))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	req.Body = r.Body

	logger.Printf("%s \"%s %s %s\" - -", r.RemoteAddr, req.Method, req.URL.String(), req.Proto)

	const PasswordKey string = "X-UrlFetch-Password"
	if Password != "" {
		if password := req.Header.Get(PasswordKey); password != Password {
			http.Error(rw, fmt.Sprintf("wrong password %#v", password), http.StatusForbidden)
			return
		}
	}
	req.Header.Del(PasswordKey)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		TLSHandshakeTimeout: 30 * time.Second,
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}

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

	fmt.Fprintf(w, "HTTP/1.1 200\r\n")
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
