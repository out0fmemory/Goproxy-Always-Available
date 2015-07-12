package httpproxy

import (
	"path"
	"strings"
)

type HostMatcher struct {
	all0       interface{}
	list1_keys []string
	list1_map  map[string]interface{}
	list2_keys []string
	list2_map  map[string]interface{}
	list3_keys []string
	list3_map  map[string]interface{}
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

func NewHostMatcherWithValue(values map[string]interface{}) *HostMatcher {
	hm := &HostMatcher{
		all0:       nil,
		list1_keys: make([]string, 0),
		list1_map:  make(map[string]interface{}),
		list2_keys: make([]string, 0),
		list2_map:  make(map[string]interface{}),
		list3_keys: make([]string, 0),
		list3_map:  make(map[string]interface{}),
	}

	for key, value := range values {
		switch {
		case key == "*":
			hm.all0 = value
		case !strings.Contains(key, "*"):
			hm.list1_keys = append(hm.list1_keys, key)
			hm.list1_map[key] = value
		case strings.HasPrefix(key, "*") && !strings.Contains(key[1:], "*"):
			hm.list2_keys = append(hm.list2_keys, key[1:])
			hm.list2_map[key[1:]] = value
		default:
			hm.list3_keys = append(hm.list3_keys, key)
			hm.list3_map[key] = value
		}
	}

	return hm
}

func (hm *HostMatcher) Match(host string) bool {
	_, ok := hm.Lookup(host)
	return ok
}

func (hm *HostMatcher) Lookup(host string) (interface{}, bool) {
	if hm.all0 != nil {
		return hm.all0, true
	}

	if value, ok := hm.list1_map[host]; ok {
		return value, true
	}

	for _, key := range hm.list2_keys {
		if strings.HasSuffix(host, key) {
			return hm.list2_map[key], true
		}
	}

	for _, key := range hm.list3_keys {
		if matched, _ := path.Match(key, host); matched {
			return hm.list3_map[key], true
		}
	}

	return nil, false
}
