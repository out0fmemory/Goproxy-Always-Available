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

	hint(map[string]string{
		"logtostderr": "true",
		"v":           "2",
	})

	flag.Parse()

	gover := strings.Split(strings.Replace(runtime.Version(), "devel +", "devel+", 1), " ")[0]

	switch runtime.GOARCH {
	case "386", "amd64":
		helpers.SetConsoleTitle(fmt.Sprintf("GoProxy %s (go/%s)", version, gover))
	}

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
Enabled Filters    : %v`,
			profile,
			config.Address,
			fmt.Sprintf("%s|%s|%s", strings.Join(config.RequestFilters, ","), strings.Join(config.RoundTripFilters, ","), strings.Join(config.ResponseFilters, ",")))
		for _, fn := range config.RoundTripFilters {
			switch fn {
			case "autoproxy":
				fmt.Fprintf(os.Stderr, `
Pac Server         : http://%s/proxy.pac`, config.Address)
			}
		}
		go httpproxy.ServeProfile(profile)
	}
	fmt.Fprintf(os.Stderr, "\n------------------------------------------------------\n")

	select {}
}

func hint(v map[string]string) {
	v1 := map[string]bool{}

	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-version" {
			fmt.Print(version)
			os.Exit(0)
		}

		for key, _ := range v {
			if strings.HasPrefix(os.Args[i], "-"+key+"=") {
				v1[key] = true
			}
		}
	}

	for key, value := range v {
		if seen, ok := v1[key]; !ok || (ok && !seen) {
			flag.Set(key, value)
		}
	}
}
