package autoproxy

import (
	"testing"
)

func TestConfig(t *testing.T) {
	filename := filterName + ".json"

	config, err := NewConfig("file://.", filename)
	if err != nil {
		t.Fatalf("NewConfig error: %s", filename, err)
	}

	if config.Sites == nil || len(config.Sites) == 0 {
		t.Fatalf("NewConfig config.Sites failed")
	}

	if config.GFWList.URL == "" {
		t.Fatalf("NewConfig config.URL failed")
	}

	if config.GFWList.File == "" {
		t.Fatalf("NewConfig config.File failed")
	}

	if config.GFWList.Encoding == "" {
		t.Fatalf("NewConfig config.Encoding failed")
	}

	if config.GFWList.Duration < 0 {
		t.Fatalf("NewConfig config.Duration failed")
	}

}
