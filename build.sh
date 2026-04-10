#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"
mkdir -p build_cache build_cache/tools build_cache/downloads bin
if [ ! -f build_cache/go.mod ]; then printf "module buildcache\n" > build_cache/go.mod; fi
mkdir -p build_cache/gocache build_cache/gomodcache
if [ ! -x build_cache/tools/go1.25.8/go/bin/go ]; then
curl -L https://go.dev/dl/go1.25.8.linux-amd64.tar.gz -o build_cache/downloads/go1.25.8.linux-amd64.tar.gz
rm -rf build_cache/tools/go1.25.8
mkdir -p build_cache/tools/go1.25.8
tar -xzf build_cache/downloads/go1.25.8.linux-amd64.tar.gz -C build_cache/tools/go1.25.8
fi
export GOROOT="$(pwd)/build_cache/tools/go1.25.8/go"
export PATH="$GOROOT/bin:$PATH"
export GOCACHE="$(pwd)/build_cache/gocache"
export GOMODCACHE="$(pwd)/build_cache/gomodcache"
export GOOS=linux
export GOARCH=amd64
if [ "$1" = "windows-amd64" ]; then export GOOS=windows; fi
if [ "$1" = "linux-amd64" ]; then export GOOS=linux; fi
OUT="bin/goshx"
if [ "$GOOS" = "windows" ]; then OUT="bin/goshx.exe"; fi
go build -trimpath -ldflags "-s -w" -o "$OUT" ./src
echo "Built $OUT"
