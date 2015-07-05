package main

import (
	"encoding/json"

	"../storage"
)

type Config struct {
	LogToStderr bool
	Addr        string
	Http        struct {
		Mode            string
		KeepAlivePeriod int
		ReadTimeout     int
		WriteTimeout    int
		Certificate     string
		PrivateKey      string
	}
	GroupCache struct {
		Addr  string
		Peers []string
	}
	Filters struct {
		Request   []string
		RoundTrip []string
		Response  []string
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

	config := &Config{}
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
