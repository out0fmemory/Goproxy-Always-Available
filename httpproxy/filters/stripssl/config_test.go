package stripssl

import (
	"testing"
)

func TestConfig(t *testing.T) {
	filename := filterName + ".json"

	config, err := NewConfig("file://.", filename)
	if err != nil {
		t.Fatalf("NewConfig error: %s", filename, err)
	}

	if config.RootCA.Name == "" {
		t.Fatalf("NewConfig RootCA.Name failed")
	}

	if config.RootCA.Dirname == "" {
		t.Fatalf("NewConfig RootCA.Dirname failed")
	}

	if config.RootCA.RsaBits < 0 {
		t.Fatalf("NewConfig RootCA.RsaBits failed")
	}

	if config.RootCA.Duration < 86400 {
		t.Fatalf("NewConfig RootCA.Duration failed")
	}

	if config.RootCA.RsaBits < 0 {
		t.Fatalf("NewConfig RootCA.RsaBits failed")
	}

	if config.Sites == nil {
		t.Fatalf("NewConfig Sites failed")
	}
}
