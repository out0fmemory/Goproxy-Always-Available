package helpers

import (
	"flag"
	"os"
	"strings"
)

func HintFlagValues(m map[string]string) {
	seen := map[string]struct{}{}

	for i := 1; i < len(os.Args); i++ {

		for key, _ := range m {
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
