package stripssl

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/phuslu/goproxy/storage"
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

func NewConfig(uri, filename string) (*Config, error) {
	store, err := storage.OpenURI(uri)
	if err != nil {
		return nil, err
	}

	config := new(Config)

	fileext := path.Ext(filename)
	filename1 := strings.TrimSuffix(filename, fileext) + ".user" + fileext

	for i, name := range []string{filename, filename1} {
		object, err := store.GetObject(name, -1, -1)
		if err != nil {
			if i == 0 {
				return nil, err
			} else {
				continue
			}
		}

		rc := object.Body()
		defer rc.Close()

		data, err := storage.ReadJson(rc)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(data, config)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}
