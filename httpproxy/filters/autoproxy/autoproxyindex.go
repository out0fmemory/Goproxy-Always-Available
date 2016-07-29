package autoproxy

import (
	"bytes"
	"context"
	"html/template"
	"io/ioutil"
	"mime"
	"net/http"
	"path/filepath"
)

func (f *Filter) IndexFilesRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	filename := req.URL.Path[1:]

	if filename == "" {
		const tpl = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1">
<link rel="icon" href="data:image/ico;base64,">
<title>Index of /</title>
</head>
<body>
<h1>Index of /</h1>
<pre>Name</pre><hr/>
<pre>{{ range $key, $value := .IndexFiles }}
📄 <a href="{{ $key }}">{{ $key }}</a>{{ end }}</pre>
<hr/><address style="font-size:small;">GoProxy Server</address>
</body>
</html>`
		t, err := template.New("index").Parse(tpl)
		if err != nil {
			return ctx, nil, err
		}

		b := new(bytes.Buffer)
		err = t.Execute(b, struct{ IndexFiles map[string]struct{} }{f.IndexFiles})
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

	resp, err := f.Store.Get(filename, -1, -1)
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
