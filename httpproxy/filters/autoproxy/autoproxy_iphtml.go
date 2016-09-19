package autoproxy

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func (f *Filter) IPHTMLRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {

	tpl0, err := ioutil.ReadFile("ip.html")
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
	if req.Method == "POST" {
		//rawips := req.FormValue("rawips")
		jsonips := req.FormValue("jsonips")
		filterName := "gae"
		fileName := filterName + ".user.json"
		msg = "Failed."
		_, err = os.Stat(fileName)
		if !(err == nil || os.IsExist(err)) {
			fileName = filterName + ".json"
		}
		if len(jsonips) > 0 {
			jsonips = strings.Replace(jsonips, "\r\n", "", -1)
			jsonips = strings.Replace(jsonips, "\n", "", -1)
			ips := strings.Split(jsonips, ",")
			for i, ip := range ips {
				ips[i] = "\t\t\t" + ip
			}
			jsonips = strings.Join(ips, ",\n")

			bytes, err := ioutil.ReadFile(fileName)
			if err != nil {
				return ctx, nil, err
			}
			content := string(bytes)
			if n := strings.Index(content, "HostMap"); n > -1 {
				tmp := content[n:]
				tmp = tmp[strings.Index(tmp, "[")+1 : strings.Index(tmp, "]")]
				content = strings.Replace(content, tmp, "\n"+jsonips, -1)
				if err = ioutil.WriteFile(fileName, []byte(content), os.ModePerm); err != nil {
					return ctx, nil, err
				}
				msg = fmt.Sprintf("Success. Total %d IP.", len(ips)-1)
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
