package helpers

import (
	"fmt"
	"path"
	"strings"
)

type HostMatcher struct {
	starValue    interface{}
	strictMap    map[string]interface{}
	prefixList   []string
	prefixMap    map[string]interface{}
	wildcardList []string
	wildcardMap  map[string]interface{}
}

func (hm *HostMatcher) add(host string, value interface{}) {
	switch {
	case strings.Contains(host, "/"):
		panic(fmt.Sprintf("invalid host(%#v) for HostMatcher", host))
	case host == "*":
		hm.starValue = value
	case !strings.Contains(host, "*"):
		if hm.strictMap == nil {
			hm.strictMap = make(map[string]interface{})
		}
		hm.strictMap[host] = value
	case strings.HasPrefix(host, "*") && !strings.Contains(host[1:], "*"):
		if hm.prefixList == nil {
			hm.prefixList = make([]string, 0)
		}
		if hm.prefixMap == nil {
			hm.prefixMap = make(map[string]interface{})
		}
		hm.prefixList = append(hm.prefixList, host[1:])
		hm.prefixMap[host[1:]] = value
	default:
		if hm.wildcardList == nil {
			hm.wildcardList = make([]string, 0)
		}
		if hm.wildcardMap == nil {
			hm.wildcardMap = make(map[string]interface{})
		}
		hm.wildcardList = append(hm.wildcardList, host)
		hm.wildcardMap[host] = value
	}
}

func NewHostMatcher(hosts []string) *HostMatcher {
	values := make(map[string]interface{}, len(hosts))
	for _, host := range hosts {
		values[host] = struct{}{}
	}
	return NewHostMatcherWithValue(values)
}

func NewHostMatcherWithString(hosts map[string]string) *HostMatcher {
	values := make(map[string]interface{}, len(hosts))
	for host, value := range hosts {
		values[host] = value
	}
	return NewHostMatcherWithValue(values)
}

func NewHostMatcherWithStrings(hosts map[string][]string) *HostMatcher {
	values := make(map[string]interface{}, len(hosts))
	for host, value := range hosts {
		values[host] = value
	}
	return NewHostMatcherWithValue(values)
}

func NewHostMatcherWithValue(values map[string]interface{}) *HostMatcher {
	hm := &HostMatcher{}

	for host, value := range values {
		hm.add(host, value)
	}

	return hm
}

func (hm *HostMatcher) AddHost(host string) {
	hm.AddHostWithValue(host, struct{}{})
}

func (hm *HostMatcher) AddHostWithValue(host string, value interface{}) {
	hm.add(host, value)
}

func (hm *HostMatcher) Match(host string) bool {
	_, ok := hm.Lookup(host)
	return ok
}

func (hm *HostMatcher) Lookup(host string) (interface{}, bool) {
	if hm.starValue != nil {
		return hm.starValue, true
	}

	if hm.strictMap != nil {
		if value, ok := hm.strictMap[host]; ok {
			return value, true
		}
	}

	if hm.prefixList != nil {
		for _, key := range hm.prefixList {
			if strings.HasSuffix(host, key) {
				return hm.prefixMap[key], true
			}
		}
	}

	if hm.wildcardList != nil {
		for _, key := range hm.wildcardList {
			if matched, _ := path.Match(key, host); matched {
				return hm.wildcardMap[key], true
			}
		}
	}

	return nil, false
}
