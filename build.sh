#!/bin/bash
# Build package artifacts as part of RPM/DEB build process.
# Expected environment variables:
#
# For build action:
#     GOVERSION: value like "go1.14.1" that specifies what version of Go
#                will be downloaded and installed IF there is no Go found.
# For install action:
#     BUILDROOT: target directory for all packaged files (artifacts).

set -e -x
echo 'Going to cd into directory'
cd $(dirname $0)

ACTION="$1"
REPO_ROOT=$(pwd)

do_build() {
        # ensure go is installed
        if ! go version >& /dev/null; then
                test -n "$GOVERSION"
                curl -s "https://dl.google.com/go/${GOVERSION}.linux-amd64.tar.gz" | tar xz
                PATH="$REPO_ROOT/go/bin:$PATH"
                GOROOT="$REPO_ROOT/go"
                export PATH
                export GOROOT
        fi

        echo "Using $(go version)"

        # ensure GOPATH is set
        if [[ -z "$GOPATH" ]]; then
                GOPATH="$REPO_ROOT/go"
                mkdir -p "$GOPATH"
                export GOPATH
        fi

        local version=$(cat build_version)
        go install \
                -ldflags="-X main.buildVersion=$version" \
                ./apps/nsqd/...
}

do_install() {
        test -n "$BUILDROOT"  # ensure var is set
        if [ -z "$GOPATH" -a -d go ]; then
                GOPATH="$REPO_ROOT/go"
                export GOPATH
        fi
        SBIN_DIR=${SBIN_DIR:-/usr/sbin}
        mkdir -p "$BUILDROOT/$SBIN_DIR"
        install -m 0755 "$GOPATH/bin/nsqd" "$BUILDROOT/$SBIN_DIR/cloudlinux-nsqd"
}

case "$ACTION" in
        build)
                do_build
                ;;
        install)
                do_install
                ;;
        *)
                echo "Usage: ./$(basename $0) {build|install}"
                exit 1
                ;;
esac
