package helpers

import (
	"io"
	"io/ioutil"
	"testing"
	"time"
)

func TestFragmentPipeWriteTo1(t *testing.T) {
	p := NewFragmentPipe(6)
	p.WriteString("foo", 0)
	p.WriteString("bar", 3)
	n, err := io.Copy(ioutil.Discard, p)
	if err != nil {
		t.Errorf("io.Copy(%T) error: %v", p, err)
	}
	t.Logf("io.Copy(%T) return: n=%v, err=%v", p, n, err)
}

func TestFragmentPipeWriteTo2(t *testing.T) {
	p := NewFragmentPipe(6)
	go p.WriteString("foo", 0)
	go p.WriteString("bar", 3)
	n, err := io.Copy(ioutil.Discard, p)
	if err != nil {
		t.Errorf("io.Copy(%T) error: %v", p, err)
	}
	t.Logf("io.Copy(%T) return: n=%v, err=%v", p, n, err)
}

func TestFragmentPipeWriteTo3(t *testing.T) {
	p := NewFragmentPipe(6)
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.WriteString("foo", 0)
	}()
	go func() {
		p.WriteString("bar", 3)
	}()
	n, err := io.Copy(ioutil.Discard, p)
	if err != nil {
		t.Errorf("io.Copy(%T) error: %v", p, err)
	}
	t.Logf("io.Copy(%T) return: n=%v, err=%v", p, n, err)
}

func TestFragmentPipeWriteTo4(t *testing.T) {
	p := NewFragmentPipe(6)
	go func() {
		p.WriteString("foo", 0)
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		p.WriteString("bar", 3)
	}()
	n, err := io.Copy(ioutil.Discard, p)
	if err != nil {
		t.Errorf("io.Copy(%T) error: %v", p, err)
	}
	t.Logf("io.Copy(%T) return: n=%v, err=%v", p, n, err)
}

func TestFragmentPipeWriteTo5(t *testing.T) {
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
		t.Logf("io.Copy(%T) return:err=%v", p, err)
	}
}

func TestFragmentPipeWriteTo6(t *testing.T) {
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
	n, err := io.Copy(ioutil.Discard, p)
	if err != nil {
		t.Errorf("io.Copy(%T) error: %v", p, err)
	}
	t.Logf("io.Copy(%T) return: n=%v, err=%v", p, n, err)
}
