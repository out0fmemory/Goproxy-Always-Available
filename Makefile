RELEASE = r$(shell git rev-list HEAD | wc -l | xargs)

PACKAGE = govps
REPO = $(shell git rev-parse --show-toplevel)
SOURCEDIR = $(REPO)/fetchserver/vps
BUILDDIR = $(SOURCEDIR)/build
STAGEDIR = $(BUILDDIR)/stage
OBJECTDIR = $(BUILDDIR)/obj
DISTDIR = $(BUILDDIR)/dist

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

GOVPS_EXE = $(PACKAGE)
GOVPS_STAGEDIR = $(STAGEDIR)/opt/govps
GOVPS_DISTCMD = env BZIP=-9 tar cvjpf
GOVPS_DISTEXT = .tar.bz2

OBJECTS :=
OBJECTS += $(OBJECTDIR)/$(GOVPS_EXE)

SOURCES :=
SOURCES += $(REPO)/assets/systemd/govps.service


.PHONY: build
build: $(DISTDIR)/$(PACKAGE)_$(GOOS)_$(GOARCH)-$(RELEASE)$(GOVPS_DISTEXT)

.PHONY: clean
clean:
	$(RM) -rf $(BUILDDIR)

$(DISTDIR)/$(PACKAGE)_$(GOOS)_$(GOARCH)-$(RELEASE)$(GOVPS_DISTEXT): $(OBJECTS)
	mkdir -p $(DISTDIR)
	mkdir -p $(GOVPS_STAGEDIR)/ && \
	cp $(OBJECTDIR)/$(GOVPS_EXE) $(GOVPS_STAGEDIR)/$(GOVPS_EXE)
	for f in $(SOURCES) ; do cp $$f $(GOVPS_STAGEDIR)/ ; done
	cd $(STAGEDIR) && $(GOVPS_DISTCMD) $@ *

$(OBJECTDIR)/$(GOVPS_EXE):
	mkdir -p $(OBJECTDIR)
	cp govps.go govps.go.orig
	sed "s/@VERSION@/$(RELEASE)/g" govps.go.orig > govps.go
	go build -v -o $@ . ; \
	mv govps.go.orig govps.go
