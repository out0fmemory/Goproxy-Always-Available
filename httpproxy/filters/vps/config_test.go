package vps

import (
	"testing"
)

func TestConfig(t *testing.T) {
	filename := filterName + ".json"

	config, err := NewConfig("file://.", filename)
	if err != nil {
		t.Fatalf("NewConfig error: %s", filename, err)
	}

	if config.FetchServers == "" || len(config.FetchServers) <= 0 {
		t.Fatalf("NewConfig FetchServers failed")
	}

	if config.Sites == nil || len(config.Sites) <= 0 {
		t.Fatalf("NewConfig Sites failed")
	}

	if config.Transport == "" || len(config.Transport) <= 0 {
		t.Fatalf("NewConfig Transport failed")
	}

}
