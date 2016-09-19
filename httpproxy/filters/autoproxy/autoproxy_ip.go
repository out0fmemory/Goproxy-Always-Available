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

func (f *Filter) IPFilesRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {

	const tpl = `<!DOCTYPE html>
		<html>
		<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1">
		<link rel="shortcut icon" type="image/png" href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAACOElEQVQ4jWWTTUhVQRTHf/e+a+/1RCghSqNN0KYWbVq1aCFIm5AKHtQ2cFOUazdB0CKIiHZKmxbSxkUkiFFEkRGmpW0tIrNMBTU/3r0zb+bM3Bbv3rzqwGFg5vc/5z8fJ2DX6Hnw9pk0zDnvTLvYBmIM0lDaKv3448D1fkADLufD3Qm8k4uj/d3tTizOWrxYvHeVNPA3gWNApcjvSeCs5fydEZzY7bA23+4AqkU+Auh5+P62t/a4WBUYnQwZVY9ExyVnDOG+ci1Nfc6XgdKOBJcffXjtxHalXsB7AqAUtUC5CkEJJ7bIk87XFr2Ntej6/fKpd/dC711X06rZaTuLfIjaGgMskhA6VYnQN4DOKAd9QeQL505Tr52O30w/6R9cme7uRhRIQuhVO9AR7RBbi66vjy1MjDxdmHm1DgiQAmZ+vPvkwVbpwylwCUgCsD8S09BebMWJRW2sDE8O9g1t/rx1pbVy4lLoVOU/LDF5dUThRWsgCJ3IXG79z9TL5/Gvvt62qr8a+qI4KYgTcJokSacAEzodD4gx2onl9/ToerXiLmwLCqKCGGBmVr8AtoLsZ3UAR4FGunhtMoeXl/4Ol7xKm04UOAXAtznz5Wzv2gTwIwIMsARsAmmx2uEDroYIOAtiCc4sdQF1QGX8agj4bGEVWPM20U2rhTj9FZ+mOnuRReB7Nid7eqG+uVGzOv7sTaxzJ3aic3b8k7oLWCDJClogDXYnyO7kEHAEaANaMpdbWdVlmi0NwD8Bv6//isUl3wAAAABJRU5ErkJggg==" />
		<title>Index of /ip.html</title>
		</head>
		<body>
			<form method="POST" action="ip">
				<center>
					<textarea name="rawips" rows="20" cols="50" onkeyup="javascript:convert()" onblur="javascript:convert()" spellcheck="false" placeholder="请把包含 IP 地址的文本粘贴到此处"></textarea>
					<textarea name="jsonips" rows="20" cols="50" spellcheck="false" style="background-color:#F3F3F3;" readonly="true" placeholder="格式化后的 IP 地址将显示在此"></textarea>
				</center>
				<center>
					<input type="submit" name="submit" value="Submit" style="padding:8px;margin-top:40px;"/>
				</center>
				<center>
					<p>{{.Message}}</p>
				</center>
			</form>
			<script type="text/javascript">
				function convert() {
					var text1 = document.getElementsByTagName('textarea')[0];
					var text2 = document.getElementsByTagName('textarea')[1];
					if (text1.value != "") {
						var matches = text1.value.match(/([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})/g)
						if (matches != null) {
							text2.value = "\"" + matches.join("\",\n\"") + "\",";
							// text2.select();
						} else {
							text2.value = "没有检测到 IP 地址";
						}
					} else {
						text2.value = "";
					}
				}
			</script>
		</body>
		</html>`
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
