package helpers

import (
	"flag"
	"os"
	"strings"

	"github.com/kardianos/osext"
)

func FixOSArgs0() {
	if p, err := osext.Executable(); err == nil {
		os.Args[0] = p
	}
}

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
