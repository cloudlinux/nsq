REPO_ROOT := $(shell pwd)

build: install-go build-nsqd

install: SBIN_DIR ?= /usr/sbin
install:
	mkdir -p "$(DESTDIR)/${SBIN_DIR}"
	install -m 0755 ${REPO_ROOT}/bin/cloudlinux-nsqd $(DESTDIR)${SBIN_DIR}/cloudlinux-nsqd

install-go: GOVERSION ?= go1.19.1
install-go:
	curl -s "https://dl.google.com/go/${GOVERSION}.linux-amd64.tar.gz" | tar xz

build-nsqd:
	go/bin/go build -o bin/cloudlinux-nsqd ./apps/nsqd/...
