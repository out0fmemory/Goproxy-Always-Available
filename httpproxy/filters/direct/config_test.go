package direct

import (
	"testing"
)

func TestConfig(t *testing.T) {
	filename := filterName + ".json"

	config, err := NewConfig("file://.", filename)
	if err != nil {
		t.Fatalf("NewConfig error: %s", filename, err)
	}

	if config.Dialer.Timeout <= 0 {
		t.Fatalf("NewConfig Dialer.Timeout failed")
	}

	if config.Transport.MaxIdleConnsPerHost <= 0 {
		t.Fatalf("NewConfig Transport.MaxIdleConnsPerHost failed")
	}

	if config.Transport.TLSHandshakeTimeout <= 0 {
		t.Fatalf("NewConfig Transport.TLSHandshakeTimeout failed")
	}

	if config.RateLimit.Threshold <= 0 {
		t.Fatalf("NewConfig RateLimit.Threshold failed")
	}

	// if config.RateLimit.Capacity <= 0 {
	// 	t.Fatalf("NewConfig RateLimit.Capacity failed")
	// }

	// if config.RateLimit.Rate <= 0 {
	// 	t.Fatalf("NewConfig RateLimit.Rate failed")
	// }

	if config.DNSCache.Size <= 0 {
		t.Fatalf("NewConfig DNSCache.Size failed")
	}

	if config.DNSCache.Expires <= 0 {
		t.Fatalf("NewConfig DNSCache.Expires failed")
	}

}
