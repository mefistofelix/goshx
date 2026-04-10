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
set GONOSUMDB=github.com/mefistofelix/*
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
test_cache\goshx.exe -c "echo one two three | wc -w" > test_cache\wc.txt
if errorlevel 1 exit /b 1
findstr /c:"3" test_cache\wc.txt >nul
if errorlevel 1 echo wc test failed & exit /b 1
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
test_cache\goshx.exe -c "echo alpha beta > test_cache/sed.txt; sed 's/beta/gamma/' test_cache/sed.txt" > test_cache\sed_out.txt
if errorlevel 1 exit /b 1
findstr /c:"alpha gamma" test_cache\sed_out.txt >nul
if errorlevel 1 echo sed test failed & exit /b 1
test_cache\goshx.exe -c "echo line1 > test_cache/tail.txt; echo line2 >> test_cache/tail.txt; echo line3 >> test_cache/tail.txt; tail -n 2 test_cache/tail.txt" > test_cache\tail_out.txt
if errorlevel 1 exit /b 1
findstr /c:"line2" test_cache\tail_out.txt >nul
if errorlevel 1 echo tail first line test failed & exit /b 1
findstr /c:"line3" test_cache\tail_out.txt >nul
if errorlevel 1 echo tail second line test failed & exit /b 1
test_cache\goshx.exe -c "cd test_cache; mkdir -p tar_src; echo packed > tar_src/item.txt; tar -cf archive.tar tar_src; mkdir -p tar_out; tar -xf archive.tar tar_out; cat tar_out/tar_src/item.txt" > test_cache\tar.txt
if errorlevel 1 exit /b 1
set /p TAR_OUT=<test_cache\tar.txt
if not "%TAR_OUT%"=="packed" echo tar test failed & exit /b 1
if exist test_cache\wget.html del /q test_cache\wget.html
test_cache\goshx.exe -c "wget -O test_cache/wget.html https://example.com/"
if errorlevel 1 exit /b 1
findstr /c:"Example Domain" test_cache\wget.html >nul
if errorlevel 1 echo wget test failed & exit /b 1
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
echo "echo C:\\new" > test_cache\.goshx\history
(echo echo hist-reload& echo.) | test_cache\goshx.exe >nul 2>nul
if errorlevel 1 exit /b 1
findstr /c:"\"echo C:\\\\new\"" test_cache\.goshx\history >nul
if errorlevel 1 echo history escape format test failed & exit /b 1
if exist test_cache\.goshx rmdir /s /q test_cache\.goshx
(echo echo no-history& echo.) | test_cache\goshx.exe --no-history >nul 2>nul
if errorlevel 1 exit /b 1
if exist test_cache\.goshx echo no-history flag test failed & exit /b 1
echo {"command":"echo jsontest"} | test_cache\goshx.exe --json > test_cache\json_pretty.txt
if errorlevel 1 exit /b 1
findstr /c:"exit_code" test_cache\json_pretty.txt >nul
if errorlevel 1 echo json mode exit_code field test failed & exit /b 1
findstr /c:"jsontest" test_cache\json_pretty.txt >nul
if errorlevel 1 echo json mode stdout test failed & exit /b 1
findstr /c:"duration_ms" test_cache\json_pretty.txt >nul
if errorlevel 1 echo json mode duration_ms field test failed & exit /b 1
echo {"command":"echo oneline"} | test_cache\goshx.exe --json --json-out-oneline > test_cache\json_compact.txt
if errorlevel 1 exit /b 1
findstr /c:"exit_code" test_cache\json_compact.txt >nul
if errorlevel 1 echo json compact mode test failed & exit /b 1
for /f "usebackq delims=" %%L in (test_cache\json_compact.txt) do (
  set JSON_LINE=%%L
)
if not "%JSON_LINE:~0,1%"=="{" echo json compact not single line test failed & exit /b 1
echo {"command":"exit 7"} | test_cache\goshx.exe --json > test_cache\json_exitcode.txt
if not errorlevel 7 exit /b 1
findstr /c:"\"exit_code\": 7" test_cache\json_exitcode.txt >nul
if errorlevel 1 echo json exit_code value test failed & exit /b 1
echo {"command":"pwd","cwd":"%CD:\=/%/test_cache"} | test_cache\goshx.exe --json --json-out-oneline > test_cache\json_cwd.txt
if errorlevel 1 exit /b 1
findstr /c:"test_cache" test_cache\json_cwd.txt >nul
if errorlevel 1 echo json cwd override test failed & exit /b 1
echo {"command":"echo $JSON_ENV_VAR","env":{"JSON_ENV_VAR":"json-ok"}} | test_cache\goshx.exe --json --json-out-oneline > test_cache\json_env.txt
if errorlevel 1 exit /b 1
findstr /c:"json-ok" test_cache\json_env.txt >nul
if errorlevel 1 echo json env override test failed & exit /b 1
echo {"command":"cat","stdin":"json-stdin"} | test_cache\goshx.exe --json --json-out-oneline > test_cache\json_stdin.txt
if errorlevel 1 exit /b 1
findstr /c:"json-stdin" test_cache\json_stdin.txt >nul
if errorlevel 1 echo json stdin test failed & exit /b 1
echo {"args":["echo","json-args"]} | test_cache\goshx.exe --json --json-out-oneline > test_cache\json_args.txt
if errorlevel 1 exit /b 1
findstr /c:"json-args" test_cache\json_args.txt >nul
if errorlevel 1 echo json args array test failed & exit /b 1
if exist test_cache\.goshx rmdir /s /q test_cache\.goshx
echo {"command":"echo json-history"} | test_cache\goshx.exe --json >nul
if errorlevel 1 exit /b 1
if not exist test_cache\.goshx\history echo json history file test failed & exit /b 1
findstr /c:"echo json-history" test_cache\.goshx\history >nul
if errorlevel 1 echo json history append test failed & exit /b 1
if exist test_cache\.goshx rmdir /s /q test_cache\.goshx
echo {"command":"echo no-json-history"} | test_cache\goshx.exe --json --no-history >nul
if errorlevel 1 exit /b 1
if exist test_cache\.goshx echo json no-history flag test failed & exit /b 1
echo {"command":"echo out; missing-json-cmd","merge_output":true} | test_cache\goshx.exe --json --json-out-oneline > test_cache\json_merge.txt
if not errorlevel 127 exit /b 1
findstr /c:"out" test_cache\json_merge.txt >nul
if errorlevel 1 echo json merge_output stdout test failed & exit /b 1
findstr /c:"missing-json-cmd" test_cache\json_merge.txt >nul
if errorlevel 1 echo json merge_output stderr test failed & exit /b 1
test_cache\goshx.exe --json-out-oneline > test_cache\json_badflag.txt 2>&1
if not errorlevel 2 exit /b 1
findstr /c:"--json-out-oneline requires --json" test_cache\json_badflag.txt >nul
if errorlevel 1 echo json oneline flag validation test failed & exit /b 1
test_cache\goshx.exe --compact > test_cache\json_renamed_flag.txt 2>&1
if not errorlevel 2 exit /b 1
findstr /c:"--compact has been renamed to --json-out-oneline" test_cache\json_renamed_flag.txt >nul
if errorlevel 1 echo json renamed compact flag validation test failed & exit /b 1
test_cache\goshx.exe -c "printf 'apple\nbanana\ncherry\n' | grep an" > test_cache\grep.txt
if errorlevel 1 exit /b 1
findstr /c:"banana" test_cache\grep.txt >nul
if errorlevel 1 echo grep match test failed & exit /b 1
test_cache\goshx.exe -c "printf 'banana\napple\ncherry\n' | sort" > test_cache\sort.txt
if errorlevel 1 exit /b 1
findstr /c:"apple" test_cache\sort.txt >nul
if errorlevel 1 echo sort test failed & exit /b 1
test_cache\goshx.exe -c "printf 'a\na\nb\nb\nb\nc\n' | uniq -c" > test_cache\uniq.txt
if errorlevel 1 exit /b 1
findstr /c:"2 a" test_cache\uniq.txt >nul
if errorlevel 1 echo uniq count test failed & exit /b 1
findstr /c:"3 b" test_cache\uniq.txt >nul
if errorlevel 1 echo uniq count b test failed & exit /b 1
test_cache\goshx.exe -c "printf 'one:two:three\nfour:five:six\n' | cut -d: -f2" > test_cache\cut.txt
if errorlevel 1 exit /b 1
findstr /c:"two" test_cache\cut.txt >nul
if errorlevel 1 echo cut fields test failed & exit /b 1
findstr /c:"five" test_cache\cut.txt >nul
if errorlevel 1 echo cut fields2 test failed & exit /b 1
test_cache\goshx.exe -c "echo hello | tee test_cache/tee_file.txt" > test_cache\tee_stdout.txt
if errorlevel 1 exit /b 1
findstr /c:"hello" test_cache\tee_file.txt >nul
if errorlevel 1 echo tee file test failed & exit /b 1
findstr /c:"hello" test_cache\tee_stdout.txt >nul
if errorlevel 1 echo tee stdout test failed & exit /b 1
test_cache\goshx.exe -c "printf 'hello\n' | tr 'a-z' 'A-Z'" > test_cache\tr.txt
if errorlevel 1 exit /b 1
findstr /c:"HELLO" test_cache\tr.txt >nul
if errorlevel 1 echo tr test failed & exit /b 1
test_cache\goshx.exe -c "sleep 0.1"
if errorlevel 1 echo sleep test failed & exit /b 1
test_cache\goshx.exe -c "date +%%Y" > test_cache\date.txt
if errorlevel 1 exit /b 1
findstr /c:"2026" test_cache\date.txt >nul
if errorlevel 1 echo date test failed & exit /b 1
echo Tests passed
