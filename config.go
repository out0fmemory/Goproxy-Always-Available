package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/naoina/toml"
	"github.com/phuslu/glog"
)

type Config struct {
	Default struct {
		LogLevel            int
		DaemonStderr        string
		RejectNilSni        bool
		PreferIpv4          bool
		DnsTtl              int
		IdleConnTimeout     int
		MaxIdleConnsPerHost int
	}
	HTTP2 []struct {
		DisableHttp2     bool
		DisableLegacySsl bool

		Network string
		Listen  string

		ServerName []string

		Keyfile  string
		Certfile string
		PEM      string

		ClientAuthFile string
		ClientAuthPem  string

		ParentProxy string

		ProxyFallback string
		DisableProxy  bool

		ProxyAuthMethod  string
		ProxyBuiltinAuth map[string]string
	}
	TLS []struct {
		ServerName []string
		Backend    string
		Terminate  bool
	}
	HTTP struct {
		Network string
		Listen  string

		ParentProxy string

		ProxyFallback string
		DisableProxy  bool

		ProxyAuthMethod  string
		ProxyBuiltinAuth map[string]string
	}
}

func NewConfig(filename string) (*Config, error) {
	var tomlData []byte
	var err error
	switch {
	case strings.HasPrefix(filename, "data:text/x-toml;"):
		parts := strings.SplitN(filename, ",", 2)
		switch parts[0] {
		case "data:text/x-toml;base64":
			tomlData, err = base64.StdEncoding.DecodeString(parts[1])
		case "data:text/x-toml;utf8":
			tomlData = []byte(parts[1])
		default:
			err = fmt.Errorf("Unkown data scheme: %#v", parts[0])
		}
		if err != nil {
			return nil, fmt.Errorf("Parse(%+v) error: %+v", parts[1], err)
		}
	case strings.HasPrefix(filename, "["):
		tomlData = []byte(filename)
	case os.Getenv("GOPROXY_VPS_CONFIG_URL") != "":
		filename = os.Getenv("GOPROXY_VPS_CONFIG_URL")
		fallthrough
	case strings.HasPrefix(filename, "https://"):
		glog.Infof("http.Get(%+v) ...", filename)
		resp, err := http.Get(filename)
		if err != nil {
			return nil, fmt.Errorf("http.Get(%+v) error: %+v", filename, err)
		}
		defer resp.Body.Close()
		tomlData, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("ioutil.ReadAll(%+v) error: %+v", resp.Body, err)
		}
	case filename == "":
		if _, err := os.Stat("goproxy-vps.user.toml"); err == nil {
			filename = "goproxy-vps.user.toml"
		} else {
			filename = "goproxy-vps.toml"
		}
		fallthrough
	default:
		tomlData, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("ioutil.ReadFile(%+v) error: %+v", filename, err)
		}
	}

	tomlData = bytes.Replace(tomlData, []byte("\r\n"), []byte("\n"), -1)
	tomlData = bytes.Replace(tomlData, []byte("\n[[https]]"), []byte("\n[[http2]]\ndisable_http2=true"), -1)

	var config Config
	if err = toml.Unmarshal(tomlData, &config); err != nil {
		return nil, fmt.Errorf("toml.Decode(%s) error: %+v\n", tomlData, err)
	}

	return &config, nil
}
