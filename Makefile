REVSION = $(shell git rev-list HEAD | wc -l | xargs)

PACKAGE = goproxy-vps

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOEXE ?= $(shell test $(GOOS) = "windows" && echo ".exe")
CGO_ENABLED ?= 0
GOARM ?= 5

ifeq ($(GOOS), windows)
	GOPROXY_VPS_EXE = $(PACKAGE).exe
	GOPROXY_VPS_DISTCMD = 7za a -y -mx=9 -m0=lzma -mfb=128 -md=64m -ms=on
	GOPROXY_VPS_DISTEXT = .7z
else ifeq ($(GOOS), darwin)
	GOPROXY_VPS_EXE = $(PACKAGE)
	GOPROXY_VPS_DISTCMD = BZIP=-9 tar cvjpf
	GOPROXY_VPS_DISTEXT = .tar.bz2
else ifneq (,$(findstring mips,$(GOARCH)))
	GOPROXY_VPS_EXE = $(PACKAGE)
	GOPROXY_VPS_DISTCMD = GZIP=-9 tar cvzpf
	GOPROXY_VPS_DISTEXT = .tar.gz
else
	GOPROXY_VPS_EXE = $(PACKAGE)
	GOPROXY_VPS_DISTCMD = XZ_OPT=-9 tar cvJpf
	GOPROXY_VPS_DISTEXT = .tar.xz
endif

GOPROXY_VPS_DIST = $(PACKAGE)_$(GOOS)_$(GOARCH)-r$(REVSION)$(GOPROXY_VPS_DISTEXT)

SOURCES =
SOURCES += goproxy-vps.toml
SOURCES += goproxy-vps.sh
SOURCES += pwauth
SOURCES += get-latest-goproxy-vps.sh

CHANGELOG = changelog.txt

.PHONY: build
build: $(GOPROXY_VPS_DIST)
	ls -lht $^

.PHONY: clean
clean:
	$(RM) -rf $(GOPROXY_VPS_EXE) $(GOPROXY_VPS_DIST) $(CHANGELOG)

$(PACKAGE)_$(GOOS)_$(GOARCH)-r$(REVSION)$(GOPROXY_VPS_DISTEXT): $(SOURCES) $(CHANGELOG) $(GOPROXY_VPS_EXE)
	$(GOPROXY_VPS_DISTCMD) $@ $^

$(CHANGELOG):
	git log --after="3 months ago" --pretty="%ci (%an) %s" >$@

$(GOPROXY_VPS_EXE): $(SOURCES)
	env GOOS=$(GOOS) GOARCH=$(GOARCH) GOARM=$(GOARM) CGO_ENABLED=$(CGO_ENABLED) CC=$(CC) \
	go build -v -ldflags="-s -w -X main.version=r$(REVSION)" -o $@ .
