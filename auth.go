package main

import (
	"errors"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
)

type SimpleAuth struct {
	Mode      string
	CacheSize uint
	Builtin   map[string]string

	path  string
	cache lrucache.Cache
	once  sync.Once
}

func (p *SimpleAuth) init() {
	if p.Mode == "builtin" {
		return
	}

	p.cache = lrucache.NewLRUCache(p.CacheSize)

	exe, err := os.Executable()
	if err != nil {
		glog.Fatalf("os.Executable() error: %+v", err)
	}

	p.path = filepath.Join(filepath.Dir(exe), "pwauth")
	if _, err := os.Stat(p.path); err != nil {
		glog.Fatalf("os.Stat(%#v) error: %+v", p.path, err)
	}

	if syscall.Geteuid() != 0 {
		glog.Warningf("Please run as root if you want to use pam auth")
	}
}

func (p *SimpleAuth) Authenticate(username, password string) error {
	p.once.Do(p.init)

	if p.Builtin != nil {
		if v, ok := p.Builtin[username]; ok && v == password {
			return nil
		} else {
			err := errors.New("wrong username or password")
			glog.Warningf("SimpleAuth: builtin username=%v password=%v error: %+v", username, password, err)
			time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
			return err
		}
	}

	auth := p.Mode + ":" + username + ":" + password

	if _, ok := p.cache.GetNotStale(auth); ok {
		return nil
	}

	cmd := exec.Command(p.path, p.Mode)
	//glog.Infof("SimpleAuth exec cmd=%#v", cmd)
	cmd.Stdin = strings.NewReader(username + "\n" + password + "\n")
	err := cmd.Run()

	if err != nil {
		glog.Warningf("SimpleAuth: username=%v password=%v error: %+v", username, password, err)
		time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
		return err
	}

	p.cache.Set(auth, struct{}{}, time.Now().Add(2*time.Hour))
	return nil
}
