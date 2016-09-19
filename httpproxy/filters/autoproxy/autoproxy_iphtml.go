package autoproxy

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"../../storage"
)

const (
	IPHTMLFilename string = "ip.html"
)

var (
	ipv4Regex = regexp.MustCompile(`(?s)(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
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

	tpl := string(tpl0)
	tpl = strings.Replace(tpl, "<!-- BEGIN IPHTML COMMENT", "", -1)
	tpl = strings.Replace(tpl, "END IPHTML COMMENT -->", "", -1)

	t, err := template.New("ip").Parse(tpl)
	if err != nil {
		return ctx, nil, err
	}

	var msg string

	switch req.Method {
	case http.MethodPost:
		store := storage.LookupStoreByFilterName("gae")
		//rawips := req.FormValue("rawips")
		jsonips := req.FormValue("jsonips")
		filename := "gae.user.json"
		if storage.NotExist(store, filename) {
			filename = "gae.json"
		}
		if len(jsonips) > 0 {
			ips := make([]string, 0, 64)
			for _, m := range ipv4Regex.FindAllStringSubmatch(jsonips, -1) {
				ips = append(ips, m[1])
			}
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
