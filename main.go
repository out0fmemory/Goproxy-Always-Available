package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"./helpers"
	"./httpproxy"
)

var version = "r9999"

func main() {

	if runtime.GOOS == "windows" {
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
	}
	flag.Parse()

	gover := strings.Split(strings.Replace(runtime.Version(), "devel +", "devel+", 1), " ")[0]

	for profile, config := range httpproxy.Config {
		if !config.Enabled {
			continue
		}
		fmt.Fprintf(os.Stderr, `------------------------------------------------------
GoProxy Version    : %s (go/%s %s/%s)
GoProxy Profile    : %s
Listen Address     : %s
Enabled Filters    : %v
Pac Server         : http://%s/proxy.pac
------------------------------------------------------
`, version, gover, runtime.GOOS, runtime.GOARCH,
			profile,
			config.Address,
			fmt.Sprintf("%s|%s|%s", strings.Join(config.RequestFilters, ","), strings.Join(config.RoundTripFilters, ","), strings.Join(config.ResponseFilters, ",")),
			config.Address)
		go httpproxy.ServeProfile(profile)
	}

	if runtime.GOOS == "windows" {
		helpers.SetConsoleTitle(fmt.Sprintf("GoProxy %s (go/%s)", version, gover))
	}

	select {}
}
