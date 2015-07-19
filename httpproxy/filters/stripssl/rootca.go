package stripssl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	// "github.com/golang/glog"
)

type RootCA struct {
	name     string
	keyFile  string
	certFile string
	rsaBits  int
	certDir  string
	mu       *sync.Mutex

	ca       *x509.Certificate
	priv     *rsa.PrivateKey
	derBytes []byte
}

func NewRootCA(name string, vaildFor time.Duration, rsaBits int, certDir string) (*RootCA, error) {
	keyFile := name + ".key"
	certFile := name + ".crt"

	rootCA := &RootCA{
		name:     name,
		keyFile:  keyFile,
		certFile: certFile,
		rsaBits:  rsaBits,
		certDir:  certDir,
		mu:       new(sync.Mutex),
	}

	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		template := x509.Certificate{
			IsCA:         true,
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				Organization: []string{name},
			},
			NotBefore: time.Now().Add(-time.Duration(5 * time.Minute)),
			NotAfter:  time.Now().Add(vaildFor),

			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
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

		outFile1, err := os.Create(keyFile)
		defer outFile1.Close()
		if err != nil {
			return nil, err
		}
		pem.Encode(outFile1, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootCA.priv)})

		outFile2, err := os.Create(certFile)
		defer outFile2.Close()
		if err != nil {
			return nil, err
		}
		pem.Encode(outFile2, &pem.Block{Type: "CERTIFICATE", Bytes: rootCA.derBytes})
	} else {
		data, err := ioutil.ReadFile(keyFile)
		if err != nil {
			return nil, err
		}

		var b *pem.Block
		for {
			b, data = pem.Decode(data)
			if b == nil {
				break
			}
			if b.Type == "CERTIFICATE" {
				rootCA.derBytes = b.Bytes
				ca, err := x509.ParseCertificate(rootCA.derBytes)
				if err != nil {
					return nil, err
				}
				rootCA.ca = ca
			} else if b.Type == "RSA PRIVATE KEY" {
				priv, err := x509.ParsePKCS1PrivateKey(b.Bytes)
				if err != nil {
					return nil, err
				}
				rootCA.priv = priv
			}
		}

		data, err = ioutil.ReadFile(certFile)
		if err != nil {
			return nil, err
		}

		for {
			b, data = pem.Decode(data)
			if b == nil {
				break
			}
			if b.Type == "CERTIFICATE" {
				rootCA.derBytes = b.Bytes
				ca, err := x509.ParseCertificate(rootCA.derBytes)
				if err != nil {
					return nil, err
				}
				rootCA.ca = ca
			} else if b.Type == "RSA PRIVATE KEY" {
				priv, err := x509.ParsePKCS1PrivateKey(b.Bytes)
				if err != nil {
					return nil, err
				}
				rootCA.priv = priv
			}
		}
	}

	return rootCA, nil
}

func (c *RootCA) issue(commonName string, vaildFor time.Duration, rsaBits int) error {
	keyFile := c.toFilename(commonName, ".key")
	certFile := c.toFilename(commonName, ".crt")

	csrTemplate := &x509.CertificateRequest{
		Signature: []byte(commonName),
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Organization:       []string{commonName},
			OrganizationalUnit: []string{c.name},
			CommonName:         commonName,
		},
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
		PublicKeyAlgorithm: csr.PublicKeyAlgorithm,
		PublicKey:          csr.PublicKey,
		SerialNumber:       big.NewInt(time.Now().UnixNano()),
		SignatureAlgorithm: x509.SHA256WithRSA,
		NotBefore:          time.Now().Add(-time.Duration(10 * time.Minute)).UTC(),
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

	outFile1, err := os.Create(keyFile)
	defer outFile1.Close()
	if err != nil {
		return err
	}
	pem.Encode(outFile1, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	outFile2, err := os.Create(certFile)
	defer outFile2.Close()
	if err != nil {
		return err
	}
	pem.Encode(outFile2, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})

	return nil
}

func GetCommonName(domain string) string {
	parts := strings.Split(domain, ".")
	switch len(parts) {
	case 1, 2:
		break
	case 3:
		if len(parts[len(parts)-1]) >= 2 && len(parts[len(parts)-2]) >= 4 {
			domain = "*." + strings.Join(parts[1:], ".")
		}
	default:
		domain = "*." + strings.Join(parts[1:], ".")
	}
	return domain
}

func (c *RootCA) RsaBits() int {
	return c.rsaBits
}

func (c *RootCA) toFilename(commonName, suffix string) string {
	if strings.HasPrefix(commonName, "*.") {
		commonName = commonName[1:]
	}
	return c.certDir + "/" + commonName + suffix
}

func (c *RootCA) Issue(commonName string, vaildFor time.Duration, rsaBits int) (*tls.Certificate, error) {
	certFile := c.toFilename(commonName, ".crt")
	keyFile := c.toFilename(commonName, ".key")

	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		c.mu.Lock()
		defer c.mu.Unlock()
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			if err = c.issue(commonName, vaildFor, rsaBits); err != nil {
				return nil, err
			}
		}
	}

	tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}
