package httpproxy

import (
	"path"
	"strings"
)

type HostMatcher struct {
	matchAll bool
	list1    map[string]struct{}
	list2    []string
}

func NewHostMatcher(rules []string) *HostMatcher {
	sm := &HostMatcher{
		list1: make(map[string]struct{}),
		list2: make([]string, 0),
	}

	for _, s := range rules {
		switch {
		case s == "*":
			sm.matchAll = true
		case strings.Contains(s, "*"):
			sm.list2 = append(sm.list2, s)
		default:
			sm.list1[s] = struct{}{}
		}
	}

	return sm
}

func (sm *HostMatcher) Match(host string) bool {
	if sm.matchAll {
		return true
	}

	if _, ok := sm.list1[host]; ok {
		return true
	}

	for _, s := range sm.list2 {
		if matched, _ := path.Match(s, host); matched {
			return true
		}
	}

	return false
}
