// +build phuslugo

package direct

import (
	"crypto/tls"
)

var (
	defaultTLSConfigForGoogle *tls.Config = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		ServerName:         "www.bing.com",
		ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		},
		NextProtos: []string{"h2", "h2-14", "http/1.1"},
	}
)
