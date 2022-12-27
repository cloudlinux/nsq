export GOVERSION=go1.19.1

build:
	./build.sh build

install:
	BUILDROOT=$(DESTDIR) ./build.sh install
