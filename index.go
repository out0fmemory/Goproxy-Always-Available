package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/gzip"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/reuseport" 
)

const (
	Version  = "1.0"
	Password = "123456"
)

var (
	logger          = log.New(os.Stdout, "index.go: ", 0)
	paramsPreifx    = "X-Urlfetch-"
	flateReaderPool sync.Pool
	gzipReaderPool  sync.Pool
	bytesBufferPool sync.Pool
)

func main() {
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

	// windows
	// if err := fasthttp.ListenAndServe(addr, handler); err != nil {
	// 	panic(err)
	// }

	listener, err := reuseport.Listen("tcp4", addr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	if err := fasthttp.Serve(listener, handler); err != nil {
		panic(err)
	}
}

func ReadRequest(li *io.LimitedReader) (req *fasthttp.Request, err error) {
	req = fasthttp.AcquireRequest()

	r := acquireFlateReader(li)
	defer releaseFlateReader(r)

	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) != 3 {
			err = fmt.Errorf("Invaild Request Line: %#v", line)
			return
		}
		req.Header.SetMethod(parts[0])
		req.SetRequestURI(parts[1])

		if u, er := url.Parse(parts[1]); er != nil {
			logger.Printf("URL Parse error: %#v, when read request.", er)
			return
		} else {
			req.Header.Set("Host", u.Host)
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		req.Header.Set(key, value)
	}

	if err = scanner.Err(); err != nil {
		// ignore
	}
	return
}

func handler(ctx *fasthttp.RequestCtx) {
	var err error

	body := acquireBytesBuffer()
	body.Write(ctx.Request.Body())
	defer releaseBytesBuffer(body)

	var hdrLen uint16
	if err := binary.Read(body, binary.BigEndian, &hdrLen); err != nil {
		logger.Printf("binary.Read error :%#v", err)
		ctx.Error("binary.Read:"+err.Error(), fasthttp.StatusBadRequest)
		return
	}

	req1, err := ReadRequest(&io.LimitedReader{R: body, N: int64(hdrLen)})
	req1.SetBody(body.Bytes())

	if ce := b2s(req1.Header.Peek("Content-Encoding")); ce != "" {
		var r io.Reader
		bb := acquireBytesBuffer()
		bb.Write(req1.Body())
		defer releaseBytesBuffer(bb)
		switch ce {
		case "deflate":
			r = acquireFlateReader(bb)
			defer releaseFlateReader(r.(io.ReadCloser))
		case "gzip":
			if r, err = acquireGzipReader(bb); err != nil {
				ctx.Error("fetchserver:"+err.Error(), fasthttp.StatusBadRequest)
				return
			}
			defer releaseGzipReader(r.(*gzip.Reader))
		default:
			ctx.Error("fetchserver:"+fmt.Sprintf("Unsupported Content-Encoding: %#v", ce), fasthttp.StatusBadRequest)
			return
		}
		data, err := ioutil.ReadAll(r)
		if err != nil {
			req1.ResetBody()
			ctx.Error("fetchserver:"+err.Error(), fasthttp.StatusBadRequest)
			return
		}
		req1.ResetBody()
		req1.SetBody(data)

		req1.Header.Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
		req1.Header.Del("Content-Encoding")
	}
	logger.Printf("%s \"%s %s\" - -", ctx.RemoteAddr(), b2s(req1.Header.Method()), b2s(req1.URI().FullURI()))

	params := map[string]string{}
	req1.Header.VisitAll(func(key, value []byte) {
		if strings.HasPrefix(b2s(key), paramsPreifx) {
			params[strings.ToLower(b2s(key[len(paramsPreifx):]))] = b2s(value)
		}
	})

	for key, _ := range params {
		req1.Header.Del(paramsPreifx + key)
	}
	if Password != "" {
		if password, ok := params["password"]; !ok || password != Password {
			ctx.Error(fmt.Sprintf("fetchserver: wrong password %#v", password), fasthttp.StatusForbidden)
			return
		}
	}

	var resp = fasthttp.AcquireResponse()
	for i := 0; i < 2; i++ {
		err = fasthttp.Do(req1, resp)
		if err == nil {
			break
		}
		if err != nil {
			logger.Printf("fasthttp Do error :%#v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		ctx.Error("fetchserver:"+err.Error(), fasthttp.StatusBadGateway)
		return
	}
	go fasthttp.ReleaseRequest(req1)
	defer fasthttp.ReleaseResponse(resp)
	ctx.SetStatusCode(fasthttp.StatusOK)
	ctx.Write(resp.Header.Header())
	ctx.Write(resp.Body())
}

func acquireBytesBuffer() *bytes.Buffer {
	b := bytesBufferPool.Get()
	if b == nil {
		return bytes.NewBuffer(make([]byte, 0, 512))
	}
	return b.(*bytes.Buffer)
}

func releaseBytesBuffer(b *bytes.Buffer) {
	b.Reset()
	bytesBufferPool.Put(b)
}

func acquireFlateReader(r io.Reader) io.ReadCloser {
	v := flateReaderPool.Get()
	if v == nil {
		fr := flate.NewReader(r)
		return fr
	}
	fr := v.(io.ReadCloser)
	if err := resetFlateReader(fr, r); err != nil {
		return nil
	}
	return fr
}

func releaseFlateReader(zr io.ReadCloser) {
	zr.Close()
	flateReaderPool.Put(zr)
}

func resetFlateReader(zr io.ReadCloser, r io.Reader) error {
	zrr, ok := zr.(flate.Resetter)
	if !ok {
		panic("BUG: flate.Reader doesn't implement flate.Resetter???")
	}
	return zrr.Reset(r, nil)
}

func acquireGzipReader(r io.Reader) (*gzip.Reader, error) {
	v := gzipReaderPool.Get()
	if v == nil {
		return gzip.NewReader(r)
	}
	zr := v.(*gzip.Reader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

func releaseGzipReader(zr *gzip.Reader) {
	zr.Close()
	gzipReaderPool.Put(zr)
}

// b2s converts byte slice to a string without memory allocation.
// See https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ .
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// s2b converts string to a byte slice without memory allocation.
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func s2b(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}
