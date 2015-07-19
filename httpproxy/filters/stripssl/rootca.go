package stripssl

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
)

type RootCA struct {
	name     string
	keyFile  string
	certFile string
	rsaBits  int
	certDir  string
	mu       *sync.Mutex
}

func NewRootCA(name string, vaildFor time.Duration, rsaBits int, certDir string) (*RootCA, error) {
	if err := prepare(); err != nil {
		return nil, err
	}

	keyFile := name + ".key"
	certFile := name + ".crt"

	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		cmd := exec.Command("openssl",
			"req",
			"-new",
			"-newkey",
			fmt.Sprintf("rsa:%d", rsaBits),
			"-days",
			strconv.Itoa(int(vaildFor/(24*time.Hour))),
			"-nodes",
			"-x509",
			"-subj",
			fmt.Sprintf("/C=CN/S=Internet/L=Cernet/O=%s/OU=%s/CN=%s", name, name, name),
			"-keyout",
			keyFile,
			"-out",
			certFile)

		if err := cmd.Run(); err != nil {
			glog.Errorf("exec.Command(%v) error: %v", cmd.Args, err)
			return nil, err
		}
	}

	return &RootCA{
		name:     name,
		keyFile:  keyFile,
		certFile: certFile,
		rsaBits:  rsaBits,
		certDir:  certDir,
		mu:       new(sync.Mutex),
	}, nil
}

func (c *RootCA) issue(commonName string, vaildFor time.Duration, rsaBits int) (err error) {
	certFile := c.toFilename(commonName, ".crt")
	keyFile := c.toFilename(commonName, ".key")
	csrFile := c.toFilename(commonName, ".csr")
	extFile := c.toFilename(commonName, ".ext")

	extData := `extensions = x509v3
[ x509v3 ]
nsCertType              = server
keyUsage                = digitalSignature,nonRepudiation,keyEncipherment
extendedKeyUsage        = msSGC,nsSGC,serverAuth
`
	err = ioutil.WriteFile(extFile, []byte(extData), 0644)
	if err != nil {
		return
	}

	subj := fmt.Sprintf("/C=CN/ST=Internet/L=Cernet/OU=%s/O=%s/CN=%s", c.name, strings.TrimPrefix(commonName, "*."), commonName)
	input := fmt.Sprintf(`req -new -nodes -sha256 -newkey rsa:%d -subj "%s" -keyout %s -out %s
x509 -req -sha256 -days %d -CA %s -CAkey %s -extfile %s -set_serial %d -in %s -out %s
quit
`, rsaBits, subj, keyFile, csrFile,
		vaildFor/(24*time.Hour), c.certFile, c.keyFile, extFile, time.Now().UnixNano(), csrFile, certFile)
	glog.V(2).Infof("openssl input: %#v", input)

	cmd := exec.Command("openssl")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}

	if err = cmd.Start(); err != nil {
		return
	}

	stdin.Write([]byte(input))
	stdin.Close()

	if err = cmd.Wait(); err != nil {
		return
	}

	for _, filename := range []string{csrFile, extFile} {
		if err = os.Remove(filename); err != nil {
			return
		}
	}

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

func prepare() error {
	const ENV_OPENSSL_CONF = "OPENSSL_CONF"

	if runtime.GOOS == "windows" {
		p, err := exec.LookPath("openssl.exe")
		if err != nil {
			return fmt.Errorf("Unable locate openssl.exe: %v", err)
		}
		dirname1 := filepath.Dir(p)
		dirname2 := filepath.Join(dirname1, "../ssl")
		for _, d := range []string{dirname1, dirname2} {
			filename := filepath.Join(d, "openssl.cnf")
			if _, err := os.Stat(filename); err == nil {
				os.Setenv(ENV_OPENSSL_CONF, filename)
			}
		}

		conf := os.Getenv(ENV_OPENSSL_CONF)
		if conf == "" {
			return fmt.Errorf("%s is not set.", ENV_OPENSSL_CONF)
		} else {
			glog.V(1).Infof("set %s=%s", ENV_OPENSSL_CONF, conf)
		}
	}
	return nil
}
