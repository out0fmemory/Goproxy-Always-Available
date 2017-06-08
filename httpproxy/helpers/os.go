package helpers

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func SetFlagsIfAbsent(m map[string]string) {
	seen := map[string]struct{}{}

	for i := 1; i < len(os.Args); i++ {
		for key := range m {
			if strings.HasPrefix(os.Args[i], "-"+key+"=") {
				seen[key] = struct{}{}
			}
		}
	}

	for key, value := range m {
		if _, ok := seen[key]; !ok {
			flag.Set(key, value)
		}
	}
}

// copy/paste from https://github.com/jamiealquiza/envy/
// Parse takes a string p that is used
// as the environment variable prefix
// for each flag configured.
func SetFlagsFromEnv(prefix string) {
	// Build a map of explicitly set flags.
	set := map[string]bool{}
	flag.CommandLine.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})

	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		// Create an env var name
		// based on the supplied prefix.
		envVar := fmt.Sprintf("%s_%s", prefix, strings.ToUpper(f.Name))
		envVar = strings.Replace(envVar, "-", "_", -1)

		// Update the Flag.Value if the
		// env var is non "".
		if val := os.Getenv(envVar); val != "" {
			// Update the value if it hasn't
			// already been set.
			if defined := set[f.Name]; !defined {
				flag.CommandLine.Set(f.Name, val)
			}
		}

		// Append the env var to the
		// Flag.Usage field.
		f.Usage = fmt.Sprintf("%s [%s]", f.Usage, envVar)
	})
}
