package helpers

import (
	"io/ioutil"
	"testing"
)

func TestFragmentPipe1(t *testing.T) {
	p := NewFragmentPipe(6)
	p.WriteString("foo", 0)
	p.WriteString("bar", 3)
	data, err := ioutil.ReadAll(p)
	if err != nil {
		t.Errorf("ioutil.ReadAll(%T) error: %v", p, err)
	}
	t.Logf("ioutil.ReadAll(%T) return: %#v", p, string(data))
}

func TestFragmentPipe2(t *testing.T) {
	p := NewFragmentPipe(6)
	go p.WriteString("foo", 0)
	go p.WriteString("bar", 3)
	data, err := ioutil.ReadAll(p)
	if err != nil {
		t.Errorf("ioutil.ReadAll(%T) error: %v", p, err)
	}
	t.Logf("ioutil.ReadAll(%T) return: %#v", p, string(data))
}
