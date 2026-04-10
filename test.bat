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
if not exist build_cache\tools\go1.26.2\go\bin\go.exe curl.exe -L https://go.dev/dl/go1.26.2.windows-amd64.zip -o build_cache\downloads\go1.26.2.windows-amd64.zip
if not exist build_cache\tools\go1.26.2\go\bin\go.exe tar -xf build_cache\downloads\go1.26.2.windows-amd64.zip -C build_cache\tools\go1.26.2
set GOROOT=%ROOT%build_cache\tools\go1.26.2\go
set PATH=%GOROOT%\bin;%PATH%
set GOCACHE=%ROOT%build_cache\gocache
set GOMODCACHE=%ROOT%build_cache\gomodcache
if exist test_cache rmdir /s /q test_cache
mkdir test_cache
go build -trimpath -o test_cache\goshx.exe .\src
if errorlevel 1 exit /b 1
for /f "usebackq delims=" %%i in (`test_cache\goshx.exe -c "pwd"`) do set PWD_OUT=%%i
if /I not "%PWD_OUT%"=="%CD%" echo pwd test failed & exit /b 1
test_cache\goshx.exe -c "mkdir -p test_cache/work/sub; echo hello > test_cache/work/hello.txt; cp test_cache/work/hello.txt test_cache/work/copy.txt; cat test_cache/work/copy.txt" > test_cache\cat.txt
if errorlevel 1 exit /b 1
set /p CAT_OUT=<test_cache\cat.txt
if not "%CAT_OUT%"=="hello" echo cp or cat test failed & exit /b 1
test_cache\goshx.exe -c "find test_cache/work -name '*.txt'" > test_cache\find.txt
if errorlevel 1 exit /b 1
findstr /c:"hello.txt" test_cache\find.txt >nul
if errorlevel 1 echo find test failed & exit /b 1
test_cache\goshx.exe -c "export HELLO_VAR=world; echo $HELLO_VAR" > test_cache\env.txt
if errorlevel 1 exit /b 1
set /p ENV_OUT=<test_cache\env.txt
if not "%ENV_OUT%"=="world" echo export test failed & exit /b 1
test_cache\goshx.exe -c "echo shell-data | base64 | cat" > test_cache\b64.txt
if errorlevel 1 exit /b 1
findstr /c:"c2hlbGwtZGF0YQo=" test_cache\b64.txt >nul
if errorlevel 1 echo base64 test failed & exit /b 1
test_cache\goshx.exe -c "echo zipme > test_cache/gzip.txt; gzip test_cache/gzip.txt"
if errorlevel 1 exit /b 1
if not exist test_cache\gzip.txt.gz echo gzip test failed & exit /b 1
echo Tests passed
