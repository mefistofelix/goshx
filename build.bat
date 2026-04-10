@echo off
setlocal
set ROOT=%~dp0
cd /d "%ROOT%"
if not exist build_cache mkdir build_cache
if not exist build_cache\go.mod echo module buildcache>build_cache\go.mod
if not exist build_cache\tools mkdir build_cache\tools
if not exist build_cache\downloads mkdir build_cache\downloads
if not exist build_cache\gocache mkdir build_cache\gocache
if not exist build_cache\gomodcache mkdir build_cache\gomodcache
if not exist build_cache\tools\go1.25.8\go\bin\go.exe curl.exe -L https://go.dev/dl/go1.25.8.windows-amd64.zip -o build_cache\downloads\go1.25.8.windows-amd64.zip
if not exist build_cache\tools\go1.25.8\go\bin\go.exe tar -xf build_cache\downloads\go1.25.8.windows-amd64.zip -C build_cache\tools\go1.25.8
set GOROOT=%ROOT%build_cache\tools\go1.25.8\go
set PATH=%GOROOT%\bin;%PATH%
set GOCACHE=%ROOT%build_cache\gocache
set GOMODCACHE=%ROOT%build_cache\gomodcache
if not exist bin mkdir bin
set GOOS=windows
set GOARCH=amd64
if /I "%1"=="linux-amd64" set GOOS=linux
if /I "%1"=="windows-amd64" set GOOS=windows
set OUT=bin\goshx.exe
if /I "%GOOS%"=="linux" set OUT=bin\goshx
go build -trimpath -ldflags "-s -w" -o "%OUT%" .\src
if errorlevel 1 exit /b 1
echo Built %OUT%
