package helpers

import (
	"testing"
)

var hosts []string = []string{
	"*.ahcdn.com",
	"*.atm.youku.com",
	"*.c.docs.google.com",
	"*.c.youtube.com",
	"*.d.rncdn3.com",
	"*.edgecastcdn.net",
	"*.mms.vlog.xuite.net",
	"*.xvideos.com",
	"*av.vimeo.com",
	"archive.rthk.hk",
	"av.voanews.com",
	"cdn*.public.extremetube.phncdn.com",
	"cdn*.public.tube8.com",
	"cdn*.video.pornhub.phncdn.com",
	"s*.last.fm",
	"smile-*.nicovideo.jp",
	"video*.modimovie.com",
	"video.*.fbcdn.net",
	"videos.flv*.redtubefiles.com",
	"vs*.thisav.com",
	"x*.last.fm",
}

var matcher *HostMatcher = NewHostMatcher(hosts)

func TestAutoRangeMatch(t *testing.T) {
	for _, host := range []string{
		"x1.last.fm",
		"av.voanews.com",
		"test.c.docs.google.com",
	} {
		ok := matcher.Match(host)
		if !ok {
			t.Errorf("matcher.Match(%#v) return %#v", host, ok)
		} else {
			t.Logf("matcher.Match(%#v) return %#v", host, ok)
		}
	}

}

func BenchmarkAutoRangeMatch(b *testing.B) {
	for i := 0; i < 50000; i++ {
		for _, host := range []string{
			"x1.last.fm",
			"av.voanews.com",
			"test.c.docs.google.com",
		} {
			ok := matcher.Match(host)
			if !ok {
				b.Errorf("matcher.Match(%#v) return %#v", host, ok)
			}
		}
	}
}

func BenchmarkAutoRangeLookup(b *testing.B) {
	for i := 0; i < 50000; i++ {
		for _, host := range []string{
			"x1.last.fm",
			"av.voanews.com",
			"test.c.docs.google.com",
		} {
			_, ok := matcher.Lookup(host)
			if !ok {
				b.Errorf("matcher.Match(%#v) return %#v", host, ok)
			}
		}
	}
}
