PRODUCT_VERSION = 1.0

GOBUILD = go build
GOFLAGS = -v
GOEXE = $(shell go env GOEXE)
GOOS = $(shell go env GOOS)

PACKAGE = goproxy
REPO = $(shell git rev-parse --show-toplevel)
SOURCEDIR = $(REPO)/$(PACKAGE)
BUILDDIR = $(SOURCEDIR)/build
STAGEDIR = $(BUILDDIR)/stage
OBJECTDIR = $(BUILDDIR)/obj
DISTDIR = $(BUILDDIR)/dist


ifeq ($(GOOS), windows)
	GOPROXY_STAGEDIR = $(STAGEDIR)
	GOPROXY_DISTCMD = 7za a -y -t7z
	GOPROXY_DISTEXT = .7z
else
	GOPROXY_STAGEDIR = $(STAGEDIR)/opt/goproxy
	GOPROXY_DISTCMD = tar cvjpf
	GOPROXY_DISTEXT = .tar.bz2
endif

VERSION = $(PRODUCT_VERSION)
RELEASE = $(shell git rev-list HEAD | wc -l | xargs)
ARCH := $(shell go env GOARCH)
GOPROXY = $(PACKAGE)$(GOEXE)

OBJECTS :=
OBJECTS += $(OBJECTDIR)/$(GOPROXY)

SOURCES :=
#SOURCES += $(SOURCEDIR)/goproxy.key
#SOURCES += $(SOURCEDIR)/goproxy.pem
SOURCES += $(REPO)/README.md
SOURCES += $(SOURCEDIR)/main.json
SOURCES += $(wildcard $(REPO)/httpproxy/filters/*/*.json)
SOURCES += $(REPO)/httpproxy/filters/autoproxy/gfwlist.txt

ifeq ($(GOOS), windows)
	SOURCES += $(REPO)/assets/gui/goagent.exe
	SOURCES += $(REPO)/assets/openssl/openssl.exe
	SOURCES += $(REPO)/assets/openssl/openssl.cnf
	SOURCES += $(REPO)/assets/openssl/libcrypto.dll
	SOURCES += $(REPO)/assets/openssl/libssl.dll
else ifeq ($(GOOS), darwin)
	SOURCES += $(REPO)/assets/gui/goagent-osx.command
else ifeq ($(GOOS), linux)
	SOURCES += $(REPO)/assets/gui/goagent-gtk.py
	SOURCES += $(REPO)/assets/systemd/goproxy.service
	SOURCES += $(REPO)/assets/systemd/goproxy-cleanlog.service
	SOURCES += $(REPO)/assets/systemd/goproxy-cleanlog.timer
else
	SOURCES += $(REPO)/assets/gui/goagent-gtk.py
	SOURCES += $(REPO)/assets/init.d/goproxy.sh
endif


.PHONY: build
build: $(DISTDIR)/$(PACKAGE)-$(VERSION)-$(RELEASE)_$(ARCH)$(GOPROXY_DISTEXT)

.PHONY: clean
clean:
	$(RM) -rf $(BUILDDIR)

$(DISTDIR)/$(PACKAGE)-$(VERSION)-$(RELEASE)_$(ARCH)$(GOPROXY_DISTEXT): $(OBJECTS)
	( mkdir -p $(DISTDIR) ; \
	  mkdir -p $(GOPROXY_STAGEDIR)/ ; \
	  cp $(OBJECTDIR)/$(GOPROXY) $(GOPROXY_STAGEDIR)/$(GOPROXY) ; \
	  cd $(STAGEDIR) ; \
	  for f in $(SOURCES) ; do cp $$f $(GOPROXY_STAGEDIR)/ ; done ; \
	  $(GOPROXY_DISTCMD) $@ * \
	)

$(OBJECTDIR)/$(GOPROXY):
	mkdir -p $(OBJECTDIR)
	( cd $(SOURCEDIR) ; \
	  cp main.go main.go.orig ; \
	  sed -i "s/@VERSION@/$(VERSION)-$(RELEASE)/g" main.go ; \
	  $(GOBUILD) $(GOFLAGS) -o $@ . ; \
	  mv main.go.orig main.go ; \
	)
