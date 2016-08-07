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
                <key>PayloadContent</key>
                <array>
                    <dict>
                        <key>DefaultsData</key>
                        <dict>
                            <key>apns</key>
                            <array>
                                <dict>
                                    <key>apn</key>
                                    <string>3gnet</string>
                                    <key>proxy</key>
                                    <string>%s</string>
                                    <key>proxyPort</key>
                                    <integer>%s</integer>
                                </dict>
                            </array>
                        </dict>
                        <key>DefaultsDomainName</key>
                        <string>com.apple.managedCarrier</string>
                    </dict>
                </array>
                <key>PayloadDescription</key>
                <string>Provides customization of carrier Access Point Name.</string>
                <key>PayloadDisplayName</key>
                <string>APN</string>
                <key>PayloadIdentifier</key>
                <string>org.goproxy.APNMobileConfig.</string>
                <key>PayloadOrganization</key>
                <string>goproxy.org</string>
                <key>PayloadType</key>
                <string>com.apple.apn.managed</string>
                <key>PayloadUUID</key>
                <string>8D83F161-1F78-4201-9A07-C136B58DB2A2</string>
                <key>PayloadVersion</key>
                <integer>1</integer>
            </dict>
        </array>
        <key>PayloadDescription</key>
        <string>Profile description.</string>
        <key>PayloadDisplayName</key>
        <string>GoProxyAPN</string>
        <key>PayloadIdentifier</key>
        <string>org.goproxy.APNMobileConfig</string>
        <key>PayloadOrganization</key>
        <string></string>
        <key>PayloadRemovalDisallowed</key>
        <false/>
        <key>PayloadType</key>
        <string>Configuration</string>
        <key>PayloadUUID</key>
        <string>F95E2370-5C89-11E6-8F3D-001E3724BDEB</string>
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
