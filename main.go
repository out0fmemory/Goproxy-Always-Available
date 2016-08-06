package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"

	"./httpproxy"
	"./httpproxy/helpers"
)

var (
	version  = "r9999"
	http2rev = "?????"
)

func main() {

	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Print(version)
		return
	}

	helpers.HintFlagValues(map[string]string{
		"logtostderr": "true",
		"v":           "2",
	})

	flag.Parse()

	gover := strings.Split(strings.TrimPrefix(runtime.Version(), "devel +"), " ")[0]

	switch runtime.GOARCH {
	case "386", "amd64":
		helpers.SetConsoleTitle(fmt.Sprintf("GoProxy %s (go/%s)", version, gover))
	}

	fmt.Fprintf(os.Stderr, `------------------------------------------------------
GoProxy Version    : %s (go/%s http2/%s %s/%s)`,
		version, gover, http2rev, runtime.GOOS, runtime.GOARCH)
	for profile, config := range httpproxy.Config {
		if !config.Enabled {
			continue
		}
		fmt.Fprintf(os.Stderr, `
GoProxy Profile    : %s
Listen Address     : %s
Enabled Filters    : %v`,
			profile,
			config.Address,
			fmt.Sprintf("%s|%s|%s", strings.Join(config.RequestFilters, ","), strings.Join(config.RoundTripFilters, ","), strings.Join(config.ResponseFilters, ",")))
		for _, fn := range config.RoundTripFilters {
			switch fn {
			case "autoproxy":
				addr := config.Address
				if strings.HasPrefix(addr, ":") {
					ip := "127.0.0.1"
					port := addr[1:]
					if ips, err := helpers.LocalInterfaceIPs(); err == nil && len(ips) > 0 {
						ip = ips[0].String()
					}
					addr = net.JoinHostPort(ip, port)
				}
				fmt.Fprintf(os.Stderr, `
Pac Server         : http://%s/proxy.pac`, addr)
			}
		}
		go httpproxy.ServeProfile(profile)
	}
	fmt.Fprintf(os.Stderr, "\n------------------------------------------------------\n")

	select {}
}
