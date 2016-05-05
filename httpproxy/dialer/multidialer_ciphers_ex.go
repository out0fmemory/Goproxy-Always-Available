// +build phuslugo

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
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		},
		NextProtos: []string{"h2", "h2-14", "http/1.1"},
	}

	mixinCiphersForGoogle []uint16 = []uint16{
		tls.TLS_RSA_WITH_AES_256_CBC_SHA256,
	}

	onceDefaultTLSConfigForGoogle sync.Once
)

func pickupCiphers(ciphers []uint16) []uint16 {
	ciphers = ciphers[:]
	length := len(ciphers)
	n := rand.Intn(length)
	for i := n + 1; i < length; i++ {
		j := rand.Intn(i)
		ciphers[i], ciphers[j] = ciphers[j], ciphers[i]
	}
	return ciphers[:n+1]
}

func GetDefaultTLSConfigForGoogle(fakeServerNames []string) *tls.Config {
	onceDefaultTLSConfigForGoogle.Do(func() {
		ciphers := pickupCiphers(defaultTLSConfigForGoogle.CipherSuites)
		ciphers = append(ciphers, pickupCiphers(mixinCiphersForGoogle)...)
		defaultTLSConfigForGoogle.CipherSuites = ciphers

		if len(fakeServerNames) > 0 {
			defaultTLSConfigForGoogle.ServerName = fakeServerNames[rand.Intn(len(fakeServerNames))]
		}
	})

	return defaultTLSConfigForGoogle
}
