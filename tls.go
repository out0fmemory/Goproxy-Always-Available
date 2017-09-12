package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
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
	tls12  map[string]struct{}
	ecc    *autocert.Manager
	rsa    *autocert.Manager
	cache  lrucache.Cache
	sni    map[string]string
	snits  map[string]bool
}

func (cm *CertManager) Add(host string, certfile, keyfile string, pem string, cafile, capem string, h2 bool, tls12 bool) error {
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

	if cm.tls12 == nil {
		cm.tls12 = make(map[string]struct{})
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

	if tls12 {
		cm.tls12[host] = struct{}{}
	}

	cm.hosts = append(cm.hosts, host)

	return nil
}

func (cm *CertManager) AddTLSProxy(serverNames []string, addr string, terminate bool) error {
	if cm.sni == nil {
		cm.sni = make(map[string]string)
		cm.snits = make(map[string]bool)
	}

	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "443")
	}

	for _, name := range serverNames {
		cm.sni[name] = addr
		cm.snits[name] = terminate
	}

	return nil
}

func (cm *CertManager) HostPolicy(_ context.Context, host string) error {
	return nil
}

func (cm *CertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	hello.ServerName = strings.ToLower(hello.ServerName)

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

func (cm *CertManager) Forward(hello *tls.ClientHelloInfo, addr string, terminate bool) (*tls.Config, error) {
	if addr[0] == ':' {
		addr = hello.ServerName + addr
	}

	size := uint16(len(hello.Raw))
	data := make([]byte, 0, 5+size)
	data = append(data, 0x16, 0x03, 0x01, byte(size>>8), byte(size&0xff))
	data = append(data, hello.Raw...)

	var lconn net.Conn = &ConnWithData{
		Conn: hello.Conn,
		Data: data,
	}

	if terminate {
		cert, err := cm.GetCertificate(hello)
		if err != nil {
			return nil, err
		}

		config := &tls.Config{
			MaxVersion:               tls.VersionTLS13,
			MinVersion:               tls.VersionTLS10,
			Certificates:             []tls.Certificate{*cert},
			PreferServerCipherSuites: true,
		}

		tlsConn := tls.Server(lconn, config)
		err = tlsConn.Handshake()
		if err != nil {
			return nil, err
		}

		lconn = tlsConn
	}

	rconn, err := cm.Dial("tcp", addr)
	if err != nil {
		lconn.Close()
		return nil, err
	}

	glog.Infof("TLS: forward %#v to %#v, ssl_terminate=%v", lconn.RemoteAddr().String(), rconn.RemoteAddr().String(), terminate)
	go helpers.IOCopy(rconn, lconn)
	helpers.IOCopy(lconn, rconn)

	rconn.Close()
	lconn.Close()

	return nil, io.EOF
}

func (cm *CertManager) GetConfigForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	if cm.sni != nil {
		if addr, ok := cm.sni[hello.ServerName]; ok {
			return cm.Forward(hello, addr, cm.snits[hello.ServerName])
		}
	}

	if hello.ServerName == "" {
		if cm.RejectNilSni {
			hello.Conn.Close()
			return nil, nil
		}
		hello.ServerName = cm.hosts[0]
	}

	_, h2 := cm.h2[hello.ServerName]
	_, tls12 := cm.tls12[hello.ServerName]
	hasECC := helpers.HasECCCiphers(hello.CipherSuites)

	cacheKey := hello.ServerName
	if !hasECC {
		cacheKey += ",rsa"
	}

	if v, ok := cm.cache.GetNotStale(cacheKey); ok {
		return v.(*tls.Config), nil
	}

	GetCertificate := cm.GetCertificate
	if tls12 {
		GetCertificate = cm.ecc.GetCertificate
	}

	cert, err := GetCertificate(hello)
	if err != nil {
		return nil, err
	}

	config := &tls.Config{
		MaxVersion:               tls.VersionTLS13,
		MinVersion:               tls.VersionTLS10,
		Certificates:             []tls.Certificate{*cert},
		Max0RTTDataSize:          100 * 1024,
		Accept0RTTData:           true,
		PreferServerCipherSuites: true,
	}

	if p, ok := cm.cpools[hello.ServerName]; ok {
		config.ClientAuth = tls.RequireAndVerifyClientCert
		config.ClientCAs = p
	}

	if h2 {
		config.NextProtos = []string{"h2", "http/1.1"}
	}

	if tls12 {
		config.MinVersion = tls.VersionTLS12
		config.CurvePreferences = []tls.CurveID{
			tls.X25519,
			tls.CurveP521,
			tls.CurveP384,
			tls.CurveP256,
		}
	}

	cm.cache.Set(cacheKey, config, time.Now().Add(2*time.Hour))

	return config, nil
}
