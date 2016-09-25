package autoproxy

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"../../storage"
)

const (
	IPHTMLFilename string = "ip.html"
)

func (f *Filter) IPHTMLRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {

	resp, err := f.Store.Get(IPHTMLFilename)
	if err != nil {
		return ctx, nil, err
	}
	defer resp.Body.Close()

	tpl0, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ctx, nil, err
	}

	tpl := strings.Replace(string(tpl0), "IPHTMLVISABLE", "1", 1)

	t, err := template.New("ip").Parse(tpl)
	if err != nil {
		return ctx, nil, err
	}

	var msg string

	switch req.Method {
	case http.MethodPost:
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			return ctx, nil, err
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return ctx, nil, fmt.Errorf("Invaild RemoteAddr: %+v", req.RemoteAddr)
		}
		if !(ip.IsLoopback() || f.IPHTMLWhiteList.Match(host)) {
			return ctx, nil, fmt.Errorf("Post from a non-local address: %+v", req.RemoteAddr)
		}

		store := storage.LookupStoreByFilterName("gae")
		//rawips := req.FormValue("rawips")
		jsonips := req.FormValue("jsonips")
		filename := "gae.user.json"
		if storage.NotExist(store, filename) {
			filename = "gae.json"
		}
		if len(jsonips) > 0 {
			s := jsonips
			for _, sep := range []string{" ", "\t", "\r", "\n"} {
				s = strings.Replace(s, sep, "", -1)
			}

			ips := strings.Split(strings.Trim(s, "\","), "\",\"")
			jsonips = "\r\n\t\t\t\"" + strings.Join(ips, "\",\r\n\t\t\t\"") + "\",\r\n\t\t"

			resp, err := store.Get(filename)
			if err != nil {
				return ctx, nil, err
			}
			defer resp.Body.Close()

			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return ctx, nil, err
			}

			content := string(data)
			if n := strings.Index(content, "HostMap"); n > -1 {
				tmp := content[n:]
				tmp = tmp[strings.Index(tmp, "[")+1 : strings.Index(tmp, "]")]
				content = strings.Replace(content, tmp, jsonips, -1)
				if _, err = store.Put(filename, http.Header{}, ioutil.NopCloser(strings.NewReader(content))); err != nil {
					return ctx, nil, err
				}
				msg = fmt.Sprintf("Updated %d IP to %s.", len(ips), filename)
			}
		}
	}
	data := struct {
		Message string
	}{
		Message: msg,
	}
	b := new(bytes.Buffer)
	err = t.Execute(b, data)
	if err != nil {
		return ctx, nil, err
	}

	return ctx, &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/html"},
		},
		Request:       req,
		Close:         true,
		ContentLength: int64(b.Len()),
		Body:          ioutil.NopCloser(b),
	}, nil

}
