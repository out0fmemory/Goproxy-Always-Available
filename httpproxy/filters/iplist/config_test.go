package iplist

import (
	"testing"
)

func TestConfig(t *testing.T) {
	filename := filterName + ".json"

	config, err := NewConfig("file://.", filename)
	if err != nil {
		t.Fatalf("NewConfig error: %s", filename, err)
	}

	if config.Dialer.Window <= 0 {
		t.Fatalf("NewConfig Dialer.Window failed")
	}

	if config.Dialer.Timeout <= 0 {
		t.Fatalf("NewConfig Dialer.Timeout failed")
	}

	// if config.Transport.MaxIdleConnsPerHost < 0 {
	// 	t.Fatalf("NewConfig Transport.MaxIdleConnsPerHost failed")
	// }

	if config.Transport.TLSHandshakeTimeout <= 0 {
		t.Fatalf("NewConfig Transport.TLSHandshakeTimeout failed")
	}

	if config.Hosts == nil && len(config.Hosts) == 0 {
		t.Fatalf("NewConfig Hosts failed")
	}

	if config.Iplist == nil || len(config.Iplist) == 0 {
		t.Fatalf("NewConfig Iplist failed")
	}

	if l, ok := config.Iplist["google_hk"]; !ok || len(l) == 0 {
		t.Fatalf("NewConfig config.Iplist google_hk failed")
	}

	if config.DNS.Servers == nil || len(config.DNS.Servers) == 0 {
		t.Fatalf("NewConfig DNS.Servers failed")
	}

	if config.DNS.Expand == nil {
		t.Fatalf("NewConfig DNS.Expand failed")
	}

	if config.DNS.Blacklist == nil || len(config.DNS.Blacklist) == 0 {
		t.Fatalf("NewConfig DNS.Blacklist failed")
	}

}
