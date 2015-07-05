package gae

import (
	"testing"
)

func TestConfig(t *testing.T) {
	filename := filterName + ".json"

	config, err := NewConfig("file://.", filename)
	if err != nil {
		t.Fatalf("NewConfig error: %s", filename, err)
	}

	if config.AppIds == nil || len(config.AppIds) <= 0 {
		t.Fatalf("NewConfig AppIds failed")
	}

	if config.Domain == "" || len(config.Domain) <= 0 {
		t.Fatalf("NewConfig Domain failed")
	}

	if config.Scheme == "" || len(config.Scheme) <= 0 {
		t.Fatalf("NewConfig Domain failed")
	}

	if config.Path == "" || len(config.Path) <= 0 {
		t.Fatalf("NewConfig Path failed")
	}

	if config.Transport == "" || len(config.Transport) <= 0 {
		t.Fatalf("NewConfig Transport failed")
	}

	if config.Sites == nil || len(config.Sites) <= 0 {
		t.Fatalf("NewConfig Sites failed")
	}
}
