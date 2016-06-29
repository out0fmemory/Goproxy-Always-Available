package autoproxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

type AutoProxy2Pac struct {
	Sites    []string
	template string
}

func (a *AutoProxy2Pac) Read(r io.Reader) error {
	scanner := bufio.NewScanner(r)

	sites := make(map[string]struct{}, 0)

	for scanner.Scan() {
		s := strings.TrimSpace(scanner.Text())

		if s == "" ||
			strings.HasPrefix(s, "[") ||
			strings.HasPrefix(s, "!") ||
			strings.HasPrefix(s, "||!") ||
			strings.HasPrefix(s, "@@") {
			continue
		}

		switch {
		case strings.HasPrefix(s, "||"):
			site := strings.Split(s[2:], "/")[0]
			switch {
			case strings.Contains(site, "*."):
				parts := strings.Split(site, "*.")
				site = parts[len(parts)-1]
			case strings.HasPrefix(site, "*"):
				parts := strings.SplitN(site, ".", 2)
				site = parts[len(parts)-1]
			}
			sites[site] = struct{}{}
		case strings.HasPrefix(s, "|http://"):
			if u, err := url.Parse(s[1:]); err == nil {
				site := u.Host
				switch {
				case strings.Contains(site, "*."):
					parts := strings.Split(site, "*.")
					site = parts[len(parts)-1]
				case strings.HasPrefix(site, "*"):
					parts := strings.SplitN(site, ".", 2)
					site = parts[len(parts)-1]
				}
				sites[site] = struct{}{}
			}
		case strings.HasPrefix(s, "."):
			site := strings.Split(strings.Split(s[1:], "/")[0], "*")[0]
			sites[site] = struct{}{}
		case !strings.ContainsAny(s, "*"):
			site := strings.Split(s, "/")[0]
			if regexp.MustCompile(`^[a-zA-Z0-9\.\_\-]+$`).MatchString(site) {
				sites[site] = struct{}{}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	sites1 := make([]string, 0)
	for s, _ := range sites {
		sites1 = append(sites1, s)
	}
	sort.Strings(sites1)

	var b bytes.Buffer
	var w io.Writer = &b

	io.WriteString(w, "var sites = {\n")

	for _, s := range sites1 {
		fmt.Fprintf(w, "'%s': 1,\n", s)
	}

	for i, s := range a.Sites {
		if i == len(a.Sites)-1 {
			fmt.Fprintf(w, "'%s': 1", s)
		} else {
			fmt.Fprintf(w, "'%s': 1,\n", s)
		}
	}

	io.WriteString(w, `
};

function FindProxyForURL(url, host) {
    if (isPlainHostName(host) ||
        host.indexOf('127.') == 0 ||
        host.indexOf('192.168.') == 0 ||
        host.indexOf('10.') == 0 ||
        shExpMatch(host, 'localhost.*')) {
        return 'DIRECT';
    }

    proxy = MyFindProxyForURL(url, host)
    if (proxy != "DIRECT") {
        return proxy
    }

    var lastPos;
    do {
        if (sites.hasOwnProperty(host)) {
            return '%s';
        }
        lastPos = host.indexOf('.') + 1;
        host = host.slice(lastPos);
    } while (lastPos >= 1);
    return 'DIRECT';
}`)

	a.template = b.String()

	return nil
}

func (a *AutoProxy2Pac) GeneratePac(host string) string {
	if a.template == "" {
		panic(fmt.Errorf("%T(%#v) has a empty template", a, a))
	}

	return fmt.Sprintf(a.template, "PROXY "+host)
}
