package auth

import (
	"testing"
)

func TestConfig(t *testing.T) {
	filename := filterName + ".json"

	config, err := NewConfig("file://.", filename)
	if err != nil {
		t.Fatalf("NewConfig error: %s", filename, err)
	}

	if config.CacheSize == 0 {
		t.Fatalf("NewConfig config.CacheSize failed")
	}

	if config.Basic == nil || len(config.Basic) == 0 {
		t.Fatalf("NewConfig config.Basic failed")
	}

	if config.WhiteList == nil || len(config.WhiteList) == 0 {
		t.Fatalf("NewConfig config.WhiteList failed")
	}

}
