GOVERSION := go1.19.1
SHELL := /bin/bash
REPO_ROOT := $(shell pwd)
SBIN_DIR ?= /usr/sbin

export PATH=${REPO_ROOT}/go/bin:$(shell printenv PATH)
export GOPATH=${REPO_ROOT}/go

build: install-go build-nsqd

install:
	mkdir -p "$(DESTDIR)/${SBIN_DIR}"
	install -m 0755 ${REPO_ROOT}/go/bin/nsqd $(DESTDIR)${SBIN_DIR}/cloudlinux-nsqd

install-go:
	curl -s "https://dl.google.com/go/${GOVERSION}.linux-amd64.tar.gz" | tar xz

build-nsqd:
	go install ./apps/nsqd/...

