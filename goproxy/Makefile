RELEASE = r$(shell git rev-list HEAD | wc -l | xargs)

PACKAGE = goproxy
REPO = $(shell git rev-parse --show-toplevel)
SOURCEDIR = $(REPO)/$(PACKAGE)
BUILDDIR = $(SOURCEDIR)/build
STAGEDIR = $(BUILDDIR)/stage
OBJECTDIR = $(BUILDDIR)/obj
DISTDIR = $(BUILDDIR)/dist

PACKAGE_GOOS ?= $(shell go env GOOS)
PACKAGE_GOARCH ?= $(shell go env GOARCH)

ifeq ($(PACKAGE_GOOS), windows)
	GOPROXY_EXE = $(PACKAGE).exe
	GOPROXY_STAGEDIR = $(STAGEDIR)
	GOPROXY_DISTCMD = 7za a -y -t7z
	GOPROXY_DISTEXT = .7z
else
	GOPROXY_EXE = $(PACKAGE)
	GOPROXY_STAGEDIR = $(STAGEDIR)/opt/goproxy
	GOPROXY_DISTCMD = tar cvjpf
	GOPROXY_DISTEXT = .tar.bz2
endif

OBJECTS :=
OBJECTS += $(OBJECTDIR)/$(GOPROXY_EXE)

SOURCES :=
#SOURCES += $(SOURCEDIR)/goproxy.key
#SOURCES += $(SOURCEDIR)/goproxy.pem
SOURCES += $(REPO)/README.md
SOURCES += $(SOURCEDIR)/main.json
SOURCES += $(wildcard $(REPO)/httpproxy/filters/*/*.json)
SOURCES += $(REPO)/httpproxy/filters/autoproxy/gfwlist.txt

ifeq ($(PACKAGE_GOOS), windows)
	SOURCES += $(REPO)/assets/gui/goagent.exe
	SOURCES += $(REPO)/assets/startup/addto-startup.js
else ifeq ($(PACKAGE_GOOS), darwin)
	SOURCES += $(REPO)/assets/gui/goagent-osx.command
else ifeq ($(PACKAGE_GOOS)_$(PACKAGE_GOARCH), linux_amd64)
	SOURCES += $(REPO)/assets/gui/goagent-gtk.py
	SOURCES += $(REPO)/assets/systemd/goproxy.service
	SOURCES += $(REPO)/assets/systemd/goproxy-cleanlog.service
	SOURCES += $(REPO)/assets/systemd/goproxy-cleanlog.timer
else
	SOURCES += $(REPO)/assets/gui/goagent-gtk.py
	SOURCES += $(REPO)/assets/startup/goproxy.sh
endif

GODEPS :=
GODEPS += github.com/cloudflare/golibs/lrucache
GODEPS += github.com/golang/glog
GODEPS += github.com/golang/groupcache
GODEPS += github.com/juju/ratelimit
GODEPS += github.com/miekg/dns
GODEPS += github.com/phuslu/http2
GODEPS += golang.org/x/net/publicsuffix

.PHONY: build
build: $(DISTDIR)/$(PACKAGE)_$(PACKAGE_GOOS)_$(PACKAGE_GOARCH)-$(RELEASE)$(GOPROXY_DISTEXT)

.PHONY: clean
clean:
	$(RM) -rf $(BUILDDIR)

.PHONY: release
release:
	ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no phuslu@vps.phus.tk sh /home/phuslu/GOPATH/src/github.com/phuslu/goproxy/assets/scripts/release.sh

$(DISTDIR)/$(PACKAGE)_$(PACKAGE_GOOS)_$(PACKAGE_GOARCH)-$(RELEASE)$(GOPROXY_DISTEXT): $(OBJECTS)
	mkdir -p $(DISTDIR)
	mkdir -p $(GOPROXY_STAGEDIR)/ && \
	cp $(OBJECTDIR)/$(GOPROXY_EXE) $(GOPROXY_STAGEDIR)/$(GOPROXY_EXE)
	for f in $(SOURCES) ; do cp $$f $(GOPROXY_STAGEDIR)/ ; done
	cd $(STAGEDIR) && $(GOPROXY_DISTCMD) $@ *

$(OBJECTDIR)/$(GOPROXY_EXE):
	for p in $(GODEPS) ; do go get -v $$p ; done
	mkdir -p $(OBJECTDIR)
	cp main.go main.go.orig
	sed "s/@VERSION@/$(RELEASE)/g" main.go.orig > main.go
	if type -p gox; then \
		gox -verbose -os=$(PACKAGE_GOOS) -arch=$(PACKAGE_GOARCH) -output="$(OBJECTDIR)/$(PACKAGE)" ; \
	else \
		go build -v -o $@ . ; \
	fi
	mv main.go.orig main.go
