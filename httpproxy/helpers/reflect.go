package helpers

import (
	"fmt"
	"net"
	"net/http"
	"reflect"
)

func ReflectRemoteIPFromResponse(resp *http.Response) (net.IP, error) {
	addr, err := ReflectRemoteAddrFromResponse(resp)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return nil, fmt.Errorf("ReflectRemoteIPFromResponse: cannot parse %+v to ip format", host)
	}

	return ip, nil
}

func ReflectRemoteAddrFromResponse(resp *http.Response) (string, error) {
	// if v := reflect.ValueOf(resp).Elem().FieldByName("RemoteAddr"); v.IsValid() {
	// 	return v.String(), nil
	// }
	return reflectRemoteAddrFromResponse(resp)
}

func reflectRemoteAddrFromResponse(resp *http.Response) (string, error) {

	if resp.Body == nil {
		return "", fmt.Errorf("ReflectRemoteAddrFromResponse: cannot reflect %#v for %v", resp, resp.Request.URL.String())
	}

	v := reflect.ValueOf(resp.Body)

	switch v.Type().String() {
	case "*http.gzipReader":
		v = v.Elem().FieldByName("body")
		fallthrough
	case "*http.bodyEOFSignal":
		v = v.Elem().FieldByName("body").Elem()
		v = reflect.Indirect(v).FieldByName("src").Elem()
		switch v.Type().String() {
		case "*internal.chunkedReader":
			v = reflect.Indirect(v).FieldByName("r").Elem()
			v = reflect.Indirect(v).FieldByName("rd").Elem()
			v = reflect.Indirect(v).FieldByName("conn").Elem()
		case "*io.LimitedReader":
			v = reflect.Indirect(v).FieldByName("R").Elem()
			v = reflect.Indirect(v).FieldByName("rd").Elem()
			v = reflect.Indirect(v).FieldByName("conn").Elem()
		default:
			return "", fmt.Errorf("ReflectRemoteAddrFromResponse: unsupport %#v Type=%s", v, v.Type().String())
		}
	case "http2.transportResponseBody":
		v = v.FieldByName("cs").Elem()
		v = v.FieldByName("cc").Elem()
		v = v.FieldByName("tconn").Elem()
	default:
		return "", fmt.Errorf("ReflectRemoteAddrFromResponse: unsupport %#v Type=%s", v, v.Type().String())
	}

	switch v.Type().String() {
	case "*tls.Conn":
		v = reflect.Indirect(v).FieldByName("conn").Elem()
		fallthrough
	case "*net.TCPConn":
		v = reflect.Indirect(v).FieldByName("fd").Elem()
		v = reflect.Indirect(v).FieldByName("raddr").Elem()
		v1 := reflect.Indirect(reflect.Indirect(v).FieldByName("IP"))
		v2 := reflect.Indirect(reflect.Indirect(v).FieldByName("Port"))
		v3 := reflect.Indirect(reflect.Indirect(v).FieldByName("Zone"))
		raddr := &net.TCPAddr{
			IP:   v1.Slice(0, v1.Len()).Bytes(),
			Port: int(v2.Int()),
			Zone: v3.String(),
		}
		return raddr.String(), nil
	}

	return "", fmt.Errorf("ReflectRemoteAddrFromResponse: unsupport %#v Type=%s", v, v.Type().String())
}
