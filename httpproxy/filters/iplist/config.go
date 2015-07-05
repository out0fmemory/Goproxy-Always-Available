package iplist

import (
	"encoding/json"
	"io/ioutil"

	"../../../storage"
)

type Config struct {
	Dialer struct {
		Window    int
		Timeout   int
		KeepAlive int
		DualStack bool
	}
	Transport struct {
		DisableKeepAlives   bool
		DisableCompression  bool
		TLSHandshakeTimeout int
		MaxIdleConnsPerHost int
	}
	Hosts  map[string]string
	Iplist map[string][]string
	DNS    struct {
		Servers   []string
		Blacklist []string
	}
}

func NewConfig(uri, path string) (*Config, error) {
	store, err := storage.OpenURI(uri)
	if err != nil {
		return nil, err
	}

	object, err := store.GetObject(path, -1, -1)
	if err != nil {
		return nil, err
	}

	rc := object.Body()
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	config := new(Config)
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
