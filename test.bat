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
test_cache\goshx.exe -c "echo permtest > test_cache/chmod.txt"
if errorlevel 1 exit /b 1
test_cache\goshx.exe -c "chmod 644 test_cache/chmod.txt; cat test_cache/chmod.txt" > test_cache\chmod_out.txt
if errorlevel 1 exit /b 1
set /p CHMOD_OUT=<test_cache\chmod_out.txt
if not "%CHMOD_OUT%"=="permtest" echo chmod test failed & exit /b 1
test_cache\goshx.exe -c "echo linkme > test_cache/ln_source.txt; ln test_cache/ln_source.txt test_cache/ln_copy.txt; cat test_cache/ln_copy.txt" > test_cache\ln.txt
if errorlevel 1 exit /b 1
set /p LN_OUT=<test_cache\ln.txt
if not "%LN_OUT%"=="linkme" echo ln test failed & exit /b 1
test_cache\goshx.exe -c "ln -s test_cache/ln_source.txt test_cache/ln_symlink.txt; cat test_cache/ln_symlink.txt" > test_cache\lns_out.txt
if errorlevel 1 exit /b 1
set /p LNS_OUT=<test_cache\lns_out.txt
if not "%LNS_OUT%"=="linkme" echo ln -s test failed & exit /b 1
test_cache\goshx.exe -c "echo zipme > test_cache/gzip.txt; gzip test_cache/gzip.txt"
if errorlevel 1 exit /b 1
if not exist test_cache\gzip.txt.gz echo gzip test failed & exit /b 1
test_cache\goshx.exe -c "mkdir -p test_cache/hx_out; hx test_cache/gzip.txt.gz test_cache/hx_out" > test_cache\hx.txt
if errorlevel 1 exit /b 1
if not exist test_cache\hx_out\gzip.txt echo hx extract test failed & exit /b 1
set /p HX_OUT=<test_cache\hx_out\gzip.txt
if not "%HX_OUT%"=="zipme" echo hx extract content test failed & exit /b 1
findstr /c:"gzip.txt" test_cache\hx.txt >nul
if errorlevel 1 echo hx test failed & exit /b 1
test_cache\goshx.exe -c "shasum test_cache/ln_source.txt" > test_cache\shasum.txt
if errorlevel 1 exit /b 1
findstr /c:"test_cache/ln_source.txt" test_cache\shasum.txt >nul
if errorlevel 1 echo shasum test failed & exit /b 1
test_cache\goshx.exe -c "echo one two | xargs -n 1 cmd /c echo" > test_cache\xargs.txt
if errorlevel 1 exit /b 1
findstr /c:"one" test_cache\xargs.txt >nul
if errorlevel 1 echo xargs first item test failed & exit /b 1
findstr /c:"two" test_cache\xargs.txt >nul
if errorlevel 1 echo xargs second item test failed & exit /b 1
if exist test_cache\.goshx rmdir /s /q test_cache\.goshx
(echo echo hist-one& echo missing-history-command& echo.) | test_cache\goshx.exe >nul 2>nul
if errorlevel 1 exit /b 1
if not exist test_cache\.goshx\history echo history file test failed & exit /b 1
findstr /c:"echo hist-one" test_cache\.goshx\history >nul
if errorlevel 1 echo history append success test failed & exit /b 1
findstr /c:"missing-history-command" test_cache\.goshx\history >nul
if errorlevel 1 echo history append failure test failed & exit /b 1
if exist test_cache\.goshx rmdir /s /q test_cache\.goshx
(echo echo no-history& echo.) | test_cache\goshx.exe --no-history >nul 2>nul
if errorlevel 1 exit /b 1
if exist test_cache\.goshx echo no-history flag test failed & exit /b 1
echo Tests passed
