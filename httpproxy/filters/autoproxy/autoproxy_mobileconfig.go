package autoproxy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

func (f *Filter) ProxyMobileConfigRoundTrip(ctx context.Context, req *http.Request) (context.Context, *http.Response, error) {
	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
		port = "80"
	}

	s := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>PayloadContent</key>
    <array>
        <dict>
            <key>APNs</key>
            <array>
                <dict>
                    <key>Name</key>
                    <string>3gnet</string>
                    <key>ProxyServer</key>
                    <string>%s</string>
                    <key>ProxyPort</key>
                    <integer>%s</integer>
                </dict>
            </array>
            <key>AttachAPN</key>
            <dict>
                <key>Name</key>
                <string>3gnet</string>
            </dict>
            <key>PayloadDescription</key>
            <string>Configures cellular data settings</string>
            <key>PayloadDisplayName</key>
            <string>Cellular</string>
            <key>PayloadIdentifier</key>
            <string>com.apple.cellular.50342D3D-1F0A-4AC2-BBBB-F91BE4303D36</string>
            <key>PayloadType</key>
            <string>com.apple.cellular</string>
            <key>PayloadUUID</key>
            <string>ED9A04BB-F7C4-451F-B541-1FDBA4BA3715</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
        </dict>
    </array>
    <key>PayloadDisplayName</key>
    <string>GoProxy APN</string>
    <key>PayloadIdentifier</key>
    <string>F38E3192-7193-4E00-AD3D-859572FE28E7</string>
    <key>PayloadRemovalDisallowed</key>
    <false/>
    <key>PayloadType</key>
    <string>Configuration</string>
    <key>PayloadUUID</key>
    <string>B075216F-63A8-498C-B8CC-8BBE85221FEE</string>
    <key>PayloadVersion</key>
    <integer>1</integer>
</dict>
</plist>
`, host, port)

	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{},
		Request:       req,
		Close:         true,
		ContentLength: int64(len(s)),
		Body:          ioutil.NopCloser(strings.NewReader(s)),
	}

	return ctx, resp, nil
}
