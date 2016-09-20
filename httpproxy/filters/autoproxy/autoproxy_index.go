package autoproxy

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"../../filters"
)

func (f *Filter) IndexFilesRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	filename := req.URL.Path[1:]

	if filename == "" {
		const tpl = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1">
<link rel="shortcut icon" type="image/png" href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAACOElEQVQ4jWWTTUhVQRTHf/e+a+/1RCghSqNN0KYWbVq1aCFIm5AKHtQ2cFOUazdB0CKIiHZKmxbSxkUkiFFEkRGmpW0tIrNMBTU/3r0zb+bM3Bbv3rzqwGFg5vc/5z8fJ2DX6Hnw9pk0zDnvTLvYBmIM0lDaKv3448D1fkADLufD3Qm8k4uj/d3tTizOWrxYvHeVNPA3gWNApcjvSeCs5fydEZzY7bA23+4AqkU+Auh5+P62t/a4WBUYnQwZVY9ExyVnDOG+ci1Nfc6XgdKOBJcffXjtxHalXsB7AqAUtUC5CkEJJ7bIk87XFr2Ntej6/fKpd/dC711X06rZaTuLfIjaGgMskhA6VYnQN4DOKAd9QeQL505Tr52O30w/6R9cme7uRhRIQuhVO9AR7RBbi66vjy1MjDxdmHm1DgiQAmZ+vPvkwVbpwylwCUgCsD8S09BebMWJRW2sDE8O9g1t/rx1pbVy4lLoVOU/LDF5dUThRWsgCJ3IXG79z9TL5/Gvvt62qr8a+qI4KYgTcJokSacAEzodD4gx2onl9/ToerXiLmwLCqKCGGBmVr8AtoLsZ3UAR4FGunhtMoeXl/4Ol7xKm04UOAXAtznz5Wzv2gTwIwIMsARsAmmx2uEDroYIOAtiCc4sdQF1QGX8agj4bGEVWPM20U2rhTj9FZ+mOnuRReB7Nid7eqG+uVGzOv7sTaxzJ3aic3b8k7oLWCDJClogDXYnyO7kEHAEaANaMpdbWdVlmi0NwD8Bv6//isUl3wAAAABJRU5ErkJggg==" />
<title>Index of /</title>
</head>
<body>
<h1>Index of /</h1>
<pre>Name</pre><hr/>
<pre>{{ range $key, $value := .IndexFiles }}
📄 <a href="{{ $value }}">{{ $value }}</a>{{ end }}</pre>
<hr/><address style="font-size:small;">{{.Branding}}, remote ip {{.Remote}}</address>
</body>
</html>`
		t, err := template.New("index").Parse(tpl)
		if err != nil {
			return ctx, nil, err
		}

		remote, _, err := net.SplitHostPort(req.RemoteAddr)
		if err == nil && f.RegionLocator != nil {
			if li, err := f.RegionLocator.Find(remote); err == nil {
				regions := []string{li.Country}
				for i, r := range []string{li.Region, li.City, li.Isp} {
					if r != "" && r != "N/A" && r != regions[i] {
						regions = append(regions, r)
					}
				}
				remote = fmt.Sprintf("%s (%s)", remote, strings.Join(regions, " "))
			}
		}

		data := struct {
			IndexFiles []string
			Remote     string
			Branding   string
		}{
			IndexFiles: f.IndexFiles,
			Remote:     remote,
			Branding:   filters.GetBranding(ctx),
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

	resp, err := f.Store.Get(filename)
	if err != nil {
		return ctx, nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ctx, nil, err
	}

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	return ctx, &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{contentType},
		},
		Request:       req,
		Close:         true,
		ContentLength: int64(len(data)),
		Body:          ioutil.NopCloser(bytes.NewReader(data)),
	}, nil
}
