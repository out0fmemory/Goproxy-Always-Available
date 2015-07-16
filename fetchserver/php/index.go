// Copyright 2012 Phus Lu. All rights reserved.

package gae

import (
	"bufio"
	"compress/flate"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	Version  = "1.0"
	Password = "123456"
)

func handler(rw http.ResponseWriter, r *http.Request) {
	var err error

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

	params := make(map[string]string)
	paramPrefix := "X-UrlFetch-"
	for key, values := range r.Header {
		if strings.HasPrefix(key, paramPrefix) {
			params[strings.ToLower(key[len(paramPrefix):])] = values[0]
		}
	}

	for key, _ := range params {
		req.Header.Del(paramPrefix + key)
	}

	if Password != "" {
		if password, ok := params["password"]; !ok || password != Password {
			http.Error(rw, err.Error(), http.StatusForbidden)
			return
		}
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		TLSHandshakeTimeout: 30 * time.Second,
	}

	resp, err := tr.RoundTrip(req)
	if err == nil {
		http.Error(rw, err.Error(), http.StatusBadGateway)
		return
	}

	if resp.ContentLength > 0 {
		resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}

	rw.Header().Set("Content-Type", "image/gif")
	rw.WriteHeader(http.StatusOK)

	fmt.Fprintf(rw, "HTTP/1.1 200\r\n")
	resp.Header.Write(rw)
	io.WriteString(rw, "\r\n")
	io.Copy(rw, resp.Body)
}

func init() {
	http.HandleFunc("/", handler)
}
