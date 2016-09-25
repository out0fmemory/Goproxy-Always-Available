package stripssl

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
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

type RootCA struct {
	store    storage.Store
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

func NewRootCA(name string, vaildFor time.Duration, rsaBits int, certDir string, portable bool) (*RootCA, error) {
	keyFile := name + ".key"
	certFile := name + ".crt"

	var store storage.Store
	if portable {
		store = &storage.FileStore{filepath.Dir(os.Args[0])}
	} else {
		store = &storage.FileStore{"."}
	}

	rootCA := &RootCA{
		store:    store,
		name:     name,
		keyFile:  keyFile,
		certFile: certFile,
		rsaBits:  rsaBits,
		certDir:  certDir,
		mu:       new(sync.Mutex),
	}

	if storage.NotExist(store, certFile) {
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
				}
			}
		}
	}

	switch runtime.GOOS {
	case "windows", "darwin":
		if _, err := rootCA.ca.Verify(x509.VerifyOptions{}); err != nil {
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
		}
	}

	if fs, ok := store.(*storage.FileStore); ok {
		if storage.NotExist(store, certDir) {
			if err := os.Mkdir(filepath.Join(fs.Dirname, certDir), 0777); err != nil {
				return nil, err
			}
		}
	}

	return rootCA, nil
}

func (c *RootCA) issue(commonName string, vaildFor time.Duration, rsaBits int) error {
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

	if storage.NotExist(c.store, certFile) {
		glog.V(2).Infof("Issue %s certificate for %#v...", c.name, commonName)
		c.mu.Lock()
		defer c.mu.Unlock()
		if storage.NotExist(c.store, certFile) {
			if err := c.issue(commonName, vaildFor, rsaBits); err != nil {
				return nil, err
			}
		}
	}

	resp, err := c.store.Get(certFile)
	if err != nil {
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
