package direct

import (
	"encoding/json"

	"../../../storage"
)

type Config struct {
	Dialer struct {
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
	RateLimit struct {
		Threshold int
		Rate      int
		Capacity  int
	}
	DNSCache struct {
		Size    int
		Expires int
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

	data, err := storage.ReadJson(rc)
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
