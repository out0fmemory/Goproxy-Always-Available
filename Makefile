REVSION = $(shell git rev-list HEAD | wc -l | xargs)

PACKAGE = goproxy-vps
REPO = $(shell git rev-parse --show-toplevel)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
GOEXE ?= $(shell go env GOEXE)
GOPROXY_VPS_EXE = $(PACKAGE)$(GOEXE)
GOPROXY_VPS_DISTCMD = XZ_OPT=-9 tar cvJpf
GOPROXY_VPS_DISTEXT = .tar.xz

SOURCES =
SOURCES += $(REPO)/goproxy-vps.go
SOURCES += $(REPO)/goproxy-vps.service

.PHONY: build
build: $(PACKAGE)_$(GOOS)_$(GOARCH)-r$(REVSION)$(GOPROXY_VPS_DISTEXT)
	ls -lht

.PHONY: clean
clean:
	$(RM) -rf $(BUILDROOT)

$(PACKAGE)_$(GOOS)_$(GOARCH)-r$(REVSION)$(GOPROXY_VPS_DISTEXT): $(SOURCES) $(GOPROXY_VPS_EXE)
	$(GOPROXY_VPS_DISTCMD) $@ $^

$(GOPROXY_VPS_EXE): $(SOURCES)
	go build -v -ldflags="-s -w -X main.version=r$(REVSION)" -o $@ .
