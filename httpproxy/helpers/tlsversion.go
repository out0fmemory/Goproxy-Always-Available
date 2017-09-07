package helpers

import (
	"crypto/tls"
	"fmt"
)

func TLSVersion(name string) uint16 {
	switch name {
	case "TLS13", "TLSv1.3", "TLSv13":
		return tls.VersionTLS13
	case "TLS12", "TLSv1.2", "TLSv12":
		return tls.VersionTLS12
	case "TLS11", "TLSv1.1", "TLSv11":
		return tls.VersionTLS11
	case "TLS1", "TLSv1.0", "TLSv10":
		return tls.VersionTLS10
	case "SSL3", "SSLv3.0", "SSLv30":
		return tls.VersionSSL30
	case "Q039":
		return 39
	case "Q038":
		return 38
	case "Q037":
		return 37
	case "Q036":
		return 36
	case "Q035":
		return 35
	}
	return 0
}

func TLSVersionName(value uint16) string {
	switch value {
	case tls.VersionTLS13, tls.VersionTLS13Draft18:
		return "TLSv13"
	case tls.VersionTLS12:
		return "TLSv12"
	case tls.VersionTLS11:
		return "TLSv11"
	case tls.VersionTLS10:
		return "TLSv1"
	case 39:
		return "Q039"
	case 38:
		return "Q038"
	case 37:
		return "Q037"
	case 36:
		return "Q036"
	case 35:
		return "Q035"
	}
	return fmt.Sprintf("0x%x", value)
}
