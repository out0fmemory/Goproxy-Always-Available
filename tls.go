// https://git.io/goproxy

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/phuslu/goproxy/httpproxy/helpers"
	"golang.org/x/crypto/acme/autocert"
)

type CertManager struct {
	RejectNilSni bool
	Dial         func(network, addr string) (net.Conn, error)

	hosts  []string
	certs  map[string]*tls.Certificate
	cpools map[string]*x509.CertPool
	h2     map[string]struct{}
	ecc    *autocert.Manager
	rsa    *autocert.Manager
	cache  lrucache.Cache
	sni    map[string]string
}

func (cm *CertManager) Add(host string, certfile, keyfile string, pem string, cafile, capem string, h2 bool) error {
	var err error

	if cm.ecc == nil {
		cm.ecc = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache("ecc"),
			HostPolicy: cm.HostPolicy,
		}
	}

	if cm.rsa == nil {
		cm.rsa = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      autocert.DirCache("rsa"),
			HostPolicy: cm.HostPolicy,
			ForceRSA:   true,
		}
	}

	if cm.certs == nil {
		cm.certs = make(map[string]*tls.Certificate)
	}

	if cm.h2 == nil {
		cm.h2 = make(map[string]struct{})
	}

	if cm.cpools == nil {
		cm.cpools = make(map[string]*x509.CertPool)
	}

	if cm.cache == nil {
		cm.cache = lrucache.NewLRUCache(128)
	}

	switch {
	case pem != "":
		cert, err := tls.X509KeyPair([]byte(pem), []byte(pem))
		if err != nil {
			return err
		}
		cm.certs[host] = &cert
	case certfile != "" && keyfile != "":
		cert, err := tls.LoadX509KeyPair(certfile, keyfile)
		if err != nil {
			return err
		}
		cm.certs[host] = &cert
	default:
		cm.certs[host] = nil
	}

	var asn1Data []byte = []byte(capem)

	if cafile != "" {
		if asn1Data, err = ioutil.ReadFile(cafile); err != nil {
			glog.Fatalf("ioutil.ReadFile(%#v) error: %+v", cafile, err)
		}
	}

	if len(asn1Data) > 0 {
		cert, err := x509.ParseCertificate(asn1Data)
		if err != nil {
			return err
		}

		certPool := x509.NewCertPool()
		certPool.AddCert(cert)

		cm.cpools[host] = certPool
	}

	if h2 {
		cm.h2[host] = struct{}{}
	}

	cm.hosts = append(cm.hosts, host)

	return nil
}

func (cm *CertManager) AddSniProxy(serverNames []string, host string, port int) error {
	if cm.sni == nil {
		cm.sni = make(map[string]string)
	}

	portStr := "443"
	if port != 0 {
		portStr = strconv.Itoa(port)
	}
	for _, name := range serverNames {
		cm.sni[name] = net.JoinHostPort(host, portStr)
	}

	return nil
}

func (cm *CertManager) HostPolicy(_ context.Context, host string) error {
	return nil
}

func (cm *CertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, _ := cm.certs[hello.ServerName]
	if cert != nil {
		return cert, nil
	}

	if helpers.HasECCCiphers(hello.CipherSuites) {
		cert, err := cm.ecc.GetCertificate(hello)
		switch {
		case cert != nil:
			return cert, nil
		case err != nil && strings.HasSuffix(hello.ServerName, ".acme.invalid"):
			break
		default:
			return nil, err
		}
	}

	return cm.rsa.GetCertificate(hello)
}

func (cm *CertManager) Forward(hello *tls.ClientHelloInfo, addr string) (*tls.Config, error) {
	if addr[0] == ':' {
		addr = hello.ServerName + addr
	}

	remote, err := cm.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	b := new(bytes.Buffer)
	b.Write([]byte{0x16, 0x03, 0x01})
	binary.Write(b, binary.BigEndian, uint16(len(hello.Raw)))

	r := io.MultiReader(b, bytes.NewReader(hello.Raw), hello.Conn)

	glog.Infof("Sniproxy: forward %#v to %#v", hello.Conn, remote)
	go helpers.IOCopy(remote, r)
	helpers.IOCopy(hello.Conn, remote)

	remote.Close()
	hello.Conn.Close()

	return nil, io.EOF
}

func (cm *CertManager) GetConfigForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	if cm.sni != nil {
		if addr, ok := cm.sni[hello.ServerName]; ok {
			return cm.Forward(hello, addr)
		}
	}

	if hello.ServerName == "" {
		if cm.RejectNilSni {
			hello.Conn.Close()
			return nil, nil
		}
		hello.ServerName = cm.hosts[0]
	}

	hasECC := helpers.HasECCCiphers(hello.CipherSuites)

	cacheKey := hello.ServerName
	if !hasECC {
		cacheKey += ",rsa"
	}

	if v, ok := cm.cache.GetNotStale(cacheKey); ok {
		return v.(*tls.Config), nil
	}

	cert, err := cm.GetCertificate(hello)
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		MaxVersion:               tls.VersionTLS13,
		MinVersion:               tls.VersionTLS10,
		Certificates:             []tls.Certificate{*cert},
		Max0RTTDataSize:          100 * 1024,
		Accept0RTTData:           true,
		AllowShortHeaders:        true,
		PreferServerCipherSuites: true,
	}

	if p, ok := cm.cpools[hello.ServerName]; ok {
		config.ClientAuth = tls.RequireAndVerifyClientCert
		config.ClientCAs = p
	}

	if _, ok := cm.h2[hello.ServerName]; ok {
		config.NextProtos = []string{"h2", "http/1.1"}
	}

	cm.cache.Set(cacheKey, config, time.Now().Add(2*time.Hour))

	return config, nil
}
