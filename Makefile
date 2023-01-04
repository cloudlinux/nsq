REPO_ROOT := $(shell pwd)

build: install-go build-nsqd

install: SBIN_DIR ?= /usr/sbin
install:
	mkdir -p "$(DESTDIR)/var/lib/imunify-nsqd"
	mkdir -p "$(DESTDIR)${SBIN_DIR}"
	install -m 0755 ${REPO_ROOT}/bin/imunify-nsqd $(DESTDIR)${SBIN_DIR}/imunify-nsqd

install-go: GOVERSION ?= go1.19.4
install-go:
	curl -s "https://dl.google.com/go/${GOVERSION}.linux-amd64.tar.gz" | tar xz

build-nsqd:
	go/bin/go build -o bin/imunify-nsqd ./apps/nsqd/...

github-release: TARGET ?= nsq-1.2.1-1.linux-amd64.go1.19.1
github-release:
	mkdir -p ${TARGET}
	go build -o ${TARGET}/bin/nsqd ./apps/nsqd/...
	go build -o ${TARGET}/bin/nsqlookupd ./apps/nsqlookupd/...
	tar -zcvf ${TARGET}.tar.gz ${TARGET}
