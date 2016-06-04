package helpers

import (
	"io"
	"io/ioutil"
	"testing"
	"time"
)

func TestFragmentPipeRead1(t *testing.T) {
	p := NewFragmentPipe(6)
	p.WriteString("foo", 0)
	p.WriteString("bar", 3)
	data, err := ioutil.ReadAll(p)
	if err != nil {
		t.Errorf("ioutil.ReadAll(%T) error: %v", p, err)
	}
	t.Logf("ioutil.ReadAll(%T) return: %#v", p, string(data))
}

func TestFragmentPipeRead2(t *testing.T) {
	p := NewFragmentPipe(6)
	go p.WriteString("foo", 0)
	go p.WriteString("bar", 3)
	data, err := ioutil.ReadAll(p)
	if err != nil {
		t.Errorf("ioutil.ReadAll(%T) error: %v", p, err)
	}
	t.Logf("ioutil.ReadAll(%T) return: %#v", p, string(data))
}

func TestFragmentPipeRead3(t *testing.T) {
	p := NewFragmentPipe(6)
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.WriteString("foo", 0)
	}()
	go func() {
		p.WriteString("bar", 3)
	}()
	data, err := ioutil.ReadAll(p)
	if err != nil {
		t.Errorf("ioutil.ReadAll(%T) error: %v", p, err)
	}
	t.Logf("ioutil.ReadAll(%T) return: %#v", p, string(data))
}

func TestFragmentPipeRead4(t *testing.T) {
	p := NewFragmentPipe(6)
	go func() {
		p.WriteString("foo", 0)
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.WriteString("bar", 3)
	}()
	data, err := ioutil.ReadAll(p)
	if err != nil {
		t.Errorf("ioutil.ReadAll(%T) error: %v", p, err)
	}
	t.Logf("ioutil.ReadAll(%T) return: %#v", p, string(data))
}

func TestFragmentPipeRead5(t *testing.T) {
	p := NewFragmentPipe(6)
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.CloseWithError(io.ErrClosedPipe)
	}()
	go func() {
		p.WriteString("bar", 3)
	}()
	_, err := ioutil.ReadAll(p)
	if err == nil {
		t.Errorf("ioutil.ReadAll(%#v) should not return nil err", p)
	} else {
		t.Logf("ioutil.ReadAll(%T) return:err=%v", p, err)
	}
}

func TestFragmentPipeRead6(t *testing.T) {
	p := NewFragmentPipe(12)
	go func() {
		time.Sleep(10 * time.Millisecond)
		p.WriteString("foo", 0)
	}()
	go func() {
		time.Sleep(0 * time.Millisecond)
		p.WriteString("bar", 3)
	}()
	go func() {
		time.Sleep(40 * time.Millisecond)
		p.WriteString("foo", 6)
	}()
	go func() {
		time.Sleep(10 * time.Millisecond)
		p.WriteString("bar", 9)
	}()
	data, err := ioutil.ReadAll(p)
	if err != nil {
		t.Errorf("ioutil.ReadAll(%T) error: %v", p, err)
	}
	t.Logf("ioutil.ReadAll(%T) return: %#v", p, string(data))
}
