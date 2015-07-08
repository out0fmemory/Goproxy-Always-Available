package stripssl

import (
	"encoding/json"

	"../../../storage"
)

type Config struct {
	RootCA struct {
		Filename string
		Dirname  string
		Name     string
		Duration int
		RsaBits  int
	}
	Sites []string
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
