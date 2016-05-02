package httpproxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/phuslu/glog"

	"./filters"
	"./helpers"
)

type Handler struct {
	http.Handler
	Listener         helpers.Listener
	RequestFilters   []filters.RequestFilter
	RoundTripFilters []filters.RoundTripFilter
	ResponseFilters  []filters.ResponseFilter
}

func (h Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var err error

	remoteAddr := req.RemoteAddr

	// Prepare filter.Context
	ctx := filters.NewContext(h.Listener, rw, req)

	// Enable transport http proxy
	if req.Method != "CONNECT" && !req.URL.IsAbs() {
		if req.URL.Scheme == "" {
			if req.TLS != nil && req.ProtoMajor == 1 {
				req.URL.Scheme = "https"
			} else {
				req.URL.Scheme = "http"
			}
		}
		if req.URL.Host == "" {
			if req.Host != "" {
				req.URL.Host = req.Host
			} else {
				if req.TLS != nil {
					req.URL.Host = req.TLS.ServerName
				}
			}
		}
	}

	// Filter Request
	for _, f := range h.RequestFilters {
		ctx, req, err = f.Request(ctx, req)
		// A roundtrip filter hijacked
		if ctx.Hijacked() {
			return
		}
		if err != nil {
			if err != io.EOF {
				glog.Errorf("%s Filter Request %T error: %#v", remoteAddr, f, err)
			}
			return
		}
	}

	if req.Body != nil {
		defer req.Body.Close()
	}

	// Filter Request -> Response
	var resp *http.Response
	for _, f := range h.RoundTripFilters {
		ctx, resp, err = f.RoundTrip(ctx, req)
		// A roundtrip filter hijacked
		if ctx.Hijacked() {
			return
		}
		// Unexcepted errors
		if err != nil {
			glog.Errorf("%s Filter RoundTrip %T error: %v", remoteAddr, f, err)
			http.Error(rw, err.Error(), http.StatusBadGateway)
			return
		}
		// A roundtrip filter give a response
		if resp != nil {
			resp.Request = req
			ctx.SetRoundTripFilter(f)
			break
		}
	}

	// Filter Response
	for _, f := range h.ResponseFilters {
		if resp == nil {
			return
		}
		ctx, resp, err = f.Response(ctx, resp)
		if err != nil {
			msg := fmt.Sprintf("%s Filter %T Response error: %v", remoteAddr, f, err)
			glog.Errorln(msg)
			http.Error(rw, msg, http.StatusBadGateway)
			return
		}
	}

	if resp == nil {
		msg := fmt.Sprintf("%s Handler %#v Response empty response", remoteAddr, h)
		glog.Errorln(msg)
		http.Error(rw, msg, http.StatusBadGateway)
		return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			rw.Header().Add(key, value)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		defer resp.Body.Close()
		n, err := helpers.IoCopy(rw, resp.Body)
		if err != nil {
			if isClosedConnError(err) {
				glog.Infof("IoCopy %#v return %#v %T(%v)", resp.Body, n, err, err)
			} else {
				glog.Warningf("IoCopy %#v return %#v %T(%v)", resp.Body, n, err, err)
			}
		}
	}
}

func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}

	str := err.Error()
	if strings.Contains(str, "use of closed network connection") {
		return true
	}

	if runtime.GOOS == "windows" {
		const WSAECONNABORTED = 10053
		const WSAECONNRESET = 10054
		if oe, ok := err.(*net.OpError); ok && (oe.Op == "read" || oe.Op == "write") {
			if se, ok := oe.Err.(*os.SyscallError); ok && (se.Syscall == "wsarecv" || se.Syscall == "wsasend") {
				if n, ok := se.Err.(syscall.Errno); ok {
					if n == WSAECONNRESET || n == WSAECONNABORTED {
						return true
					}
				}
			}
		}
	}
	return false
}
