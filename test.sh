#!/usr/bin/env bash
set -e
cd "$(dirname "$0")"
mkdir -p build_cache build_cache/tools build_cache/downloads
if [ ! -f build_cache/go.mod ]; then printf "module buildcache\n" > build_cache/go.mod; fi
mkdir -p build_cache/gocache build_cache/gomodcache
if [ ! -x build_cache/tools/go1.26.2/go/bin/go ]; then
curl -L https://go.dev/dl/go1.26.2.linux-amd64.tar.gz -o build_cache/downloads/go1.26.2.linux-amd64.tar.gz
rm -rf build_cache/tools/go1.26.2
mkdir -p build_cache/tools/go1.26.2
tar -xzf build_cache/downloads/go1.26.2.linux-amd64.tar.gz -C build_cache/tools/go1.26.2
fi
export GOROOT="$(pwd)/build_cache/tools/go1.26.2/go"
export PATH="$GOROOT/bin:$PATH"
export GOCACHE="$(pwd)/build_cache/gocache"
export GOMODCACHE="$(pwd)/build_cache/gomodcache"
rm -rf test_cache
mkdir -p test_cache
go build -trimpath -o test_cache/goshx ./src
test_cache/goshx -c "pwd" > test_cache/pwd.txt
grep -F "$(pwd)" test_cache/pwd.txt >/dev/null
test_cache/goshx -c "mkdir -p test_cache/work/sub; echo hello > test_cache/work/hello.txt; cp test_cache/work/hello.txt test_cache/work/copy.txt; cat test_cache/work/copy.txt" > test_cache/cat.txt
grep -F "hello" test_cache/cat.txt >/dev/null
test_cache/goshx -c "find test_cache/work -name '*.txt'" > test_cache/find.txt
grep -F "hello.txt" test_cache/find.txt >/dev/null
test_cache/goshx -c "export HELLO_VAR=world; echo \$HELLO_VAR" > test_cache/env.txt
grep -F "world" test_cache/env.txt >/dev/null
test_cache/goshx -c "echo shell-data | base64 | cat" > test_cache/b64.txt
grep -F "c2hlbGwtZGF0YQo=" test_cache/b64.txt >/dev/null
test_cache/goshx -c "echo linkme > test_cache/ln_source.txt; ln test_cache/ln_source.txt test_cache/ln_copy.txt; cat test_cache/ln_copy.txt" > test_cache/ln.txt
grep -F "linkme" test_cache/ln.txt >/dev/null
test_cache/goshx -c "echo zipme > test_cache/gzip.txt; gzip test_cache/gzip.txt"
test -f test_cache/gzip.txt.gz
test_cache/goshx -c "mkdir -p test_cache/hx_out; hx test_cache/gzip.txt.gz test_cache/hx_out" > test_cache/hx.txt
test -f test_cache/hx_out/gzip.txt
grep -F "zipme" test_cache/hx_out/gzip.txt >/dev/null
grep -F "gzip.txt" test_cache/hx.txt >/dev/null
echo "Tests passed"
