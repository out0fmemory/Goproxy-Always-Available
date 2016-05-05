// +build !phuslugo

package dialer

import (
	"crypto/tls"
	"math/rand"
	"sync"
)

var (
	defaultTLSConfigForGoogle *tls.Config = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
		ServerName:         "www.microsoft.com",
		ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
		NextProtos: []string{"h2", "h2-14", "http/1.1"},
	}

	onceDefaultTLSConfigForGoogle sync.Once
)

func GetDefaultTLSConfigForGoogle(fakeServerNames []string) *tls.Config {
	onceDefaultTLSConfigForGoogle.Do(func() {
		if len(fakeServerNames) > 0 {
			defaultTLSConfigForGoogle.ServerName = fakeServerNames[rand.Intn(len(fakeServerNames))]
		}
	})

	return defaultTLSConfigForGoogle
}
