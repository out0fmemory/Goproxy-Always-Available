RELEASE = r$(shell git rev-list HEAD | wc -l | xargs)

PACKAGE = goproxy
REPO = $(shell git rev-parse --show-toplevel)
SOURCEDIR = $(REPO)/
BUILDDIR = $(REPO)/build
STAGEDIR = $(BUILDDIR)/stage
OBJECTDIR = $(BUILDDIR)/obj
DISTDIR = $(BUILDDIR)/dist

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

ifeq ($(GOOS), windows)
	GOPROXY_EXE = $(PACKAGE).exe
	GOPROXY_STAGEDIR = $(STAGEDIR)
	GOPROXY_DISTCMD = 7za a -y -t7z -mx=9
	GOPROXY_DISTEXT = .7z
else ifeq ($(GOOS), darwin)
	GOPROXY_EXE = $(PACKAGE)
	GOPROXY_STAGEDIR = $(STAGEDIR)
	GOPROXY_DISTCMD = zip -r
	GOPROXY_DISTEXT = .zip
else
	GOPROXY_EXE = $(PACKAGE)
	GOPROXY_STAGEDIR = $(STAGEDIR)/opt/goproxy
	GOPROXY_DISTCMD = BZIP=-9 tar cvjpf
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

ifeq ($(GOOS), windows)
	SOURCES += $(REPO)/assets/gui/goagent.exe
	SOURCES += $(REPO)/assets/certmgr/certmgr.exe
	SOURCES += $(REPO)/assets/startup/addto-startup.js
else ifeq ($(GOOS), darwin)
	SOURCES += $(REPO)/assets/gui/goagent-osx.command
else ifeq ($(GOOS)_$(GOARCH), linux_amd64)
	SOURCES += $(REPO)/assets/gui/goagent-gtk.py
	SOURCES += $(REPO)/assets/systemd/goproxy.service
	SOURCES += $(REPO)/assets/systemd/goproxy-cleanlog.service
	SOURCES += $(REPO)/assets/systemd/goproxy-cleanlog.timer
else
	SOURCES += $(REPO)/assets/gui/goagent-gtk.py
	SOURCES += $(REPO)/assets/startup/goproxy.sh
endif

.PHONY: build
build: $(DISTDIR)/$(PACKAGE)_$(GOOS)_$(GOARCH)-$(RELEASE)$(GOPROXY_DISTEXT)

.PHONY: clean
clean:
	$(RM) -rf $(BUILDDIR)

.PHONY: release
release:
	ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no phuslu@vps.phus.lu sh /home/phuslu/goproxy/assets/scripts/release.sh

$(DISTDIR)/$(PACKAGE)_$(GOOS)_$(GOARCH)-$(RELEASE)$(GOPROXY_DISTEXT): $(OBJECTS)
	mkdir -p $(DISTDIR)
	mkdir -p $(GOPROXY_STAGEDIR)/ && \
	cp $(OBJECTDIR)/$(GOPROXY_EXE) $(GOPROXY_STAGEDIR)/$(GOPROXY_EXE)
	for f in $(SOURCES) ; do cp $$f $(GOPROXY_STAGEDIR)/ ; done
	cd $(STAGEDIR) && $(GOPROXY_DISTCMD) $@ *

$(OBJECTDIR)/$(GOPROXY_EXE):
	mkdir -p $(OBJECTDIR)
	go build -v -ldflags="-X main.version=$(RELEASE)" -o $@ .
