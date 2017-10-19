package stripssl

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/phuslu/glog"

	"../../helpers"
	"../../storage"
)

const (
	rsaBits int = 2048
)

type RootCA struct {
	store    storage.Store
	name     string
	keyFile  string
	certFile string
	certDir  string
	mu       *sync.Mutex

	ca       *x509.Certificate
	priv     *rsa.PrivateKey
	derBytes []byte
}

func NewRootCA(name string, vaildFor time.Duration, certDir string, portable bool) (*RootCA, error) {
	keyFile := name + ".key"
	certFile := name + ".crt"

	var store storage.Store
	if portable {
		exe, err := os.Executable()
		if err != nil {
			glog.Fatalf("os.Executable() error: %+v", err)
		}
		store = &storage.FileStore{filepath.Dir(exe)}
	} else {
		store = &storage.FileStore{"."}
	}

	rootCA := &RootCA{
		store:    store,
		name:     name,
		keyFile:  keyFile,
		certFile: certFile,
		certDir:  certDir,
		mu:       new(sync.Mutex),
	}

	if storage.IsNotExist(store.Head(certFile)) {
		glog.Infof("Generating RootCA for %s/%s", keyFile, certFile)
		template := x509.Certificate{
			IsCA:         true,
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				CommonName:   name,
				Country:      []string{"US"},
				Province:     []string{"California"},
				Locality:     []string{"Los Angeles"},
				Organization: []string{name},
				ExtraNames: []pkix.AttributeTypeAndValue{
					{
						Type:  []int{2, 5, 4, 42},
						Value: name,
					},
				},
			},
			DNSNames: []string{name},

			NotBefore: time.Now().Add(-time.Duration(30 * 24 * time.Hour)),
			NotAfter:  time.Now().Add(vaildFor),

			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			// AuthorityKeyId:        sha1.New().Sum([]byte("phuslu")),
			// SubjectKeyId:          sha1.New().Sum([]byte("phuslu")),
		}

		priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
		if err != nil {
			return nil, err
		}

		derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
		if err != nil {
			return nil, err
		}

		ca, err := x509.ParseCertificate(derBytes)
		if err != nil {
			return nil, err
		}

		rootCA.ca = ca
		rootCA.priv = priv
		rootCA.derBytes = derBytes

		keypem := &pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootCA.priv)}
		rc := ioutil.NopCloser(bytes.NewReader(pem.EncodeToMemory(keypem)))
		if _, err = store.Put(keyFile, http.Header{}, rc); err != nil {
			return nil, err
		}

		certpem := &pem.Block{Type: "CERTIFICATE", Bytes: rootCA.derBytes}
		rc = ioutil.NopCloser(bytes.NewReader(pem.EncodeToMemory(certpem)))
		if _, err = store.Put(certFile, http.Header{}, rc); err != nil {
			return nil, err
		}
	} else {
		for _, name := range []string{keyFile, certFile} {
			resp, err := store.Get(name)
			if err != nil {
				return nil, err
			}

			data, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, err
			}

			var b *pem.Block
			for {
				b, data = pem.Decode(data)
				if b == nil {
					break
				}
				switch b.Type {
				case "CERTIFICATE":
					rootCA.derBytes = b.Bytes
					ca, err := x509.ParseCertificate(rootCA.derBytes)
					if err != nil {
						return nil, err
					}
					rootCA.ca = ca
				case "PRIVATE KEY", "PRIVATE RSA KEY":
					priv, err := x509.ParsePKCS1PrivateKey(b.Bytes)
					if err != nil {
						return nil, err
					}
					rootCA.priv = priv
				case "EC PRIVATE KEY":
					return nil, fmt.Errorf("unsupported %#v certificate, name=%#v", b.Type, name)
				}
			}
		}
	}

	if _, err := rootCA.ca.Verify(x509.VerifyOptions{}); err != nil {
		switch runtime.GOOS + "/" + runtime.GOARCH {
		case "windows/amd64", "windows/386":
			glog.Warningf("Verify RootCA(%#v) error: %v, try import to system root", name, err)
			if err = helpers.RemoveCAFromSystemRoot(rootCA.name); err != nil {
				glog.Errorf("Remove Old RootCA(%#v) error: %v", name, err)
			}
			if err = helpers.ImportCAToSystemRoot(rootCA.ca); err != nil {
				glog.Errorf("Import RootCA(%#v) error: %v", name, err)
			} else {
				glog.Infof("Import RootCA(%s) OK", certFile)
			}

			if fs, err := store.List(certDir); err == nil {
				for _, f := range fs {
					if _, err = store.Delete(f); err != nil {
						glog.Errorf("%T.Delete(%#v) error: %v", store, f, err)
					}
				}
			}
		case "darwin/amd64", "linux/amd64", "linux/386":
			glog.Infof("Verify RootCA(%#v) error: %v, please import %#v to system root", name, err, certFile)
		}
	}

	if fs, ok := store.(*storage.FileStore); ok {
		if storage.IsNotExist(store.Head(certDir)) {
			if err := os.MkdirAll(filepath.Join(fs.Dirname, certDir), 0777); err != nil {
				return nil, err
			}
		}
	}

	return rootCA, nil
}

func (c *RootCA) issueECC(commonName string, vaildFor time.Duration) error {
	certFile := c.toFilename(commonName, true)

	csrTemplate := &x509.CertificateRequest{
		Signature: []byte(commonName),
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Organization:       []string{commonName},
			OrganizationalUnit: []string{c.name},
			CommonName:         commonName,
		},
		DNSNames: []string{commonName},
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, priv)
	if err != nil {
		return err
	}

	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return err
	}

	certTemplate := &x509.Certificate{
		Subject:            csr.Subject,
		DNSNames:           []string{commonName},
		PublicKeyAlgorithm: csr.PublicKeyAlgorithm,
		PublicKey:          csr.PublicKey,
		SerialNumber:       big.NewInt(time.Now().UnixNano()),
		NotBefore:          time.Now().Add(-time.Duration(30 * 24 * time.Hour)),
		NotAfter:           time.Now().Add(vaildFor),
		KeyUsage:           x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, c.ca, csr.PublicKey, c.priv)
	if err != nil {
		return err
	}

	b := new(bytes.Buffer)
	pem.Encode(b, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

	b1, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	pem.Encode(b, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b1})

	if _, err = c.store.Put(certFile, http.Header{}, ioutil.NopCloser(b)); err != nil {
		return err
	}

	return nil
}

func (c *RootCA) issueRSA(commonName string, vaildFor time.Duration) error {
	certFile := c.toFilename(commonName, false)

	csrTemplate := &x509.CertificateRequest{
		Signature: []byte(commonName),
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Organization:       []string{commonName},
			OrganizationalUnit: []string{c.name},
			CommonName:         commonName,
		},
		DNSNames:           []string{commonName},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return err
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, priv)
	if err != nil {
		return err
	}

	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return err
	}

	certTemplate := &x509.Certificate{
		Subject:            csr.Subject,
		DNSNames:           []string{commonName},
		PublicKeyAlgorithm: csr.PublicKeyAlgorithm,
		PublicKey:          csr.PublicKey,
		SerialNumber:       big.NewInt(time.Now().UnixNano()),
		SignatureAlgorithm: x509.SHA256WithRSA,
		NotBefore:          time.Now().Add(-time.Duration(30 * 24 * time.Hour)),
		NotAfter:           time.Now().Add(vaildFor),
		KeyUsage:           x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, c.ca, csr.PublicKey, c.priv)
	if err != nil {
		return err
	}

	b := new(bytes.Buffer)
	pem.Encode(b, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	pem.Encode(b, &pem.Block{Type: "PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	if _, err = c.store.Put(certFile, http.Header{}, ioutil.NopCloser(b)); err != nil {
		return err
	}

	return nil
}

func GetCommonName(domain string) string {
	if ip := net.ParseIP(domain); ip != nil {
		if ip.To4() == nil {
			return strings.Replace(ip.String(), ":", "-", -1)
		} else {
			return domain
		}
	}

	parts := strings.Split(domain, ".")
	switch len(parts) {
	case 1, 2:
		break
	case 3:
		len1 := len(parts[len(parts)-1])
		len2 := len(parts[len(parts)-2])
		switch {
		case len1 >= 3 || len2 >= 4:
			domain = "*." + strings.Join(parts[1:], ".")
		}
	default:
		domain = "*." + strings.Join(parts[1:], ".")
	}
	return domain
}

func (c *RootCA) toFilename(commonName string, ecc bool) string {
	if strings.HasPrefix(commonName, "*.") {
		commonName = commonName[1:]
	}

	var sepDir string
	if ecc {
		sepDir = "/ecc/"
	} else {
		sepDir = "/rsa/"
	}

	return c.certDir + sepDir + commonName + ".crt"
}

func (c *RootCA) Issue(commonName string, vaildFor time.Duration, ecc bool) (*tls.Certificate, error) {
	certFile := c.toFilename(commonName, ecc)

	resp, err := c.store.Get(certFile)
	switch {
	case err == nil && resp.StatusCode == http.StatusOK:
		t, err := time.Parse(storage.DateFormat, resp.Header.Get("Last-Modified"))
		if err == nil && time.Now().Sub(t) < 3*30*24*time.Hour {
			break
		}
		resp.Body.Close()
		c.store.Delete(certFile)
		fallthrough
	case storage.IsNotExist(resp, err):
		glog.V(2).Infof("Issue %s certificate for %#v...", c.name, commonName)
		c.mu.Lock()
		if storage.IsNotExist(c.store.Head(certFile)) {
			var err error

			if ecc {
				err = c.issueECC(commonName, vaildFor)
			} else {
				err = c.issueRSA(commonName, vaildFor)
			}
			if err != nil {
				c.mu.Unlock()
				return nil, err
			}
		}
		c.mu.Unlock()
		resp, err = c.store.Get(certFile)
		if err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}

	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	tlsCert, err := tls.X509KeyPair(data, data)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}
