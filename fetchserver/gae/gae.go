// Copyright 2012 Phus Lu. All rights reserved.

package gae

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"appengine"
	"appengine/urlfetch"
)

const (
	Version  = "1.0"
	Password = ""

	FetchMaxSize = 1024 * 1024 * 4
	Deadline     = 30 * time.Second
)

func handlerError(rw http.ResponseWriter, html string, code int) {
	var b bytes.Buffer
	w, err := flate.NewWriter(&b, flate.BestCompression)
	if err != nil {
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusBadGateway)
		io.WriteString(rw, err.Error())
	}

	fmt.Fprintf(w, "HTTP/1.1 %d\r\n", code)
	fmt.Fprintf(w, "Content-Type: text/html; charset=utf-8\r\n")
	fmt.Fprintf(w, "Content-Length: %d\r\n", len(html))
	io.WriteString(w, "\r\n")
	io.WriteString(w, html)
	w.Close()

	b0 := make([]byte, 2)
	binary.BigEndian.PutUint16(b0, uint16(b.Len()))

	rw.Header().Set("Content-Type", "image/gif")
	rw.Header().Set("Content-Length", strconv.Itoa(len(b0)+b.Len()))
	rw.WriteHeader(http.StatusOK)
	rw.Write(b0)
	rw.Write(b.Bytes())
}

func handler(rw http.ResponseWriter, r *http.Request) {
	var err error
	context := appengine.NewContext(r)
	context.Infof("Hanlde Request %#v\n", r)

	var hdrLen uint16
	if err := binary.Read(r.Body, binary.BigEndian, &hdrLen); err != nil {
		context.Criticalf("binary.Read(&hdrLen) return %v", err)
	}

	req, err := http.ReadRequest(bufio.NewReader(flate.NewReader(&io.LimitedReader{R: r.Body, N: int64(hdrLen)})))
	if err != nil {
		context.Criticalf("http.ReadRequest(%#v) return %#v", r.Body, err)
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
			handlerError(rw, "Wrong Password.", 403)
		}
	}

	deadline := Deadline

	var errors []error
	var resp *http.Response
	for i := 0; i < 2; i++ {
		t := &urlfetch.Transport{Context: context, Deadline: deadline, AllowInvalidServerCertificate: true}
		resp, err = t.RoundTrip(req)
		if err == nil {
			break
		}
		errors = append(errors, err)
		message := err.Error()
		switch {
		case strings.Contains(message, "FETCH_ERROR"):
			context.Warningf("FETCH_ERROR(type=%T, deadline=%v, url=%v)", err, deadline, req.URL)
			time.Sleep(time.Second)
			deadline *= 2
		case strings.Contains(message, "DEADLINE_EXCEEDED"):
			context.Warningf("DEADLINE_EXCEEDED(type=%T, deadline=%v, url=%v)", err, deadline, req.URL)
			time.Sleep(time.Second)
			deadline *= 2
		case strings.Contains(message, "INVALID_URL"):
			handlerError(rw, fmt.Sprintf("Invalid URL: %v", err), 501)
			return
		case strings.Contains(message, "RESPONSE_TOO_LARGE"):
			context.Warningf("RESPONSE_TOO_LARGE(type=%T, deadline=%v, url=%v)", err, deadline, req.URL)
			req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", FetchMaxSize))
			deadline *= 2
		default:
			context.Warningf("URLFetchServiceError UNKOWN(type=%T, deadline=%v, url=%v, error=%v)", err, deadline, req.URL, err)
			time.Sleep(4 * time.Second)
		}
	}

	if len(errors) == 2 {
		handlerError(rw, fmt.Sprintf("Go Server Fetch Failed: %v", errors), 502)
	}

	// Fix missing content-length
	resp.Header.Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))

	var b bytes.Buffer
	w, err := flate.NewWriter(&b, flate.BestCompression)
	if err != nil {
		handlerError(rw, fmt.Sprintf("Go Server Fetch Failed: %v", w), 502)
	}

	resp.Header.Write(w)
	w.Close()

	b0 := make([]byte, 2)
	binary.BigEndian.PutUint16(b0, uint16(b.Len()))

	rw.Header().Set("Content-Type", "image/gif")
	rw.Header().Set("Content-Length", strconv.FormatInt(int64(len(b0)+b.Len())+resp.ContentLength, 10))
	rw.WriteHeader(http.StatusOK)
	rw.Write(b0)
	rw.Write(b.Bytes())
	io.Copy(rw, resp.Body)
}

func favicon(rw http.ResponseWriter, r *http.Request) {
	rw.WriteHeader(http.StatusOK)
}

func robots(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(rw, "User-agent: *\nDisallow: /\n")
}

func root(rw http.ResponseWriter, r *http.Request) {
	context := appengine.NewContext(r)
	version, _ := strconv.ParseInt(strings.Split(appengine.VersionID(context), ".")[1], 10, 64)
	ctime := time.Unix(version/(1<<28)+8*3600, 0).Format(time.RFC3339)
	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(rw, "GoProxy server %s works, deployed at %s\n", Version, ctime)
}

func init() {
	http.HandleFunc("/_gh/", handler)
	http.HandleFunc("/favicon.ico", favicon)
	http.HandleFunc("/robots.txt", robots)
	http.HandleFunc("/", root)
}
