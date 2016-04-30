package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"./httpproxy"
	"./httpproxy/helpers"
)

var version = "r9999"

func main() {

	logToStderr := true
	for i := 1; i < len(os.Args); i++ {
		if strings.HasPrefix(os.Args[i], "-logtostderr=") {
			logToStderr = false
			break
		}
	}
	if logToStderr {
		flag.Set("logtostderr", "true")
	}
	flag.Parse()

	gover := strings.Split(strings.Replace(runtime.Version(), "devel +", "devel+", 1), " ")[0]

	fmt.Fprintf(os.Stderr, `------------------------------------------------------
GoProxy Version    : %s (go/%s %s/%s)`,
		version, gover, runtime.GOOS, runtime.GOARCH)
	for profile, config := range httpproxy.Config {
		if !config.Enabled {
			continue
		}
		fmt.Fprintf(os.Stderr, `
GoProxy Profile    : %s
Listen Address     : %s
Enabled Filters    : %v`, profile,
			config.Address,
			fmt.Sprintf("%s|%s|%s", strings.Join(config.RequestFilters, ","), strings.Join(config.RoundTripFilters, ","), strings.Join(config.ResponseFilters, ",")))
		for _, fn := range config.RoundTripFilters {
			if fn == "autoproxy" {
				fmt.Fprintf(os.Stderr, `
Pac Server         : http://%s/proxy.pac`,
					config.Address)
			}
		}
		go httpproxy.ServeProfile(profile)
	}
	fmt.Fprintf(os.Stderr, "\n------------------------------------------------------\n")

	if runtime.GOOS == "windows" {
		helpers.SetConsoleTitle(fmt.Sprintf("GoProxy %s (go/%s)", version, gover))
	}

	select {}
}
