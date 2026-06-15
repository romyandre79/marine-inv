@echo off
setlocal enabledelayedexpansion

set MODULE=github.com/fms-dms-backend
set OUT_DIR=bin
if not exist %OUT_DIR% mkdir %OUT_DIR%

echo [INFO] Starting multi-OS build for Server and Worker...

:: Define platforms to build: OS/ARCH
set PLATFORMS=windows/amd64 linux/amd64 

::darwin/amd64 darwin/arm64

for %%P in (%PLATFORMS%) do (
    for /f "tokens=1,2 delims=/" %%A in ("%%P") do (
        set GOOS=%%A
        set GOARCH=%%B
        
        set SUFFIX=
        if "!GOOS!"=="windows" set SUFFIX=.exe
        
        set TARGET_DIR=%OUT_DIR%\!GOOS!_!GOARCH!
        if not exist !TARGET_DIR! mkdir !TARGET_DIR!

        echo [BUILD] !GOOS!/!GOARCH! - Server...
        set "CGO_ENABLED=0"
        set "GOOS=!GOOS!"
        set "GOARCH=!GOARCH!"
        go build -ldflags="-s -w" -o !TARGET_DIR!\server!SUFFIX! .\cmd\server
        if !errorlevel! neq 0 (
            echo [ERROR] Server build failed for !GOOS!/!GOARCH!
        ) else (
            echo [OK] !TARGET_DIR!\server!SUFFIX!
        )

        echo [BUILD] !GOOS!/!GOARCH! - Migrate...
        go build -ldflags="-s -w" -o !TARGET_DIR!\migrate!SUFFIX! .\cmd\migrate
        if !errorlevel! neq 0 (
            echo [ERROR] Migrate build failed for !GOOS!/!GOARCH!
        ) else (
            echo [OK] !TARGET_DIR!\migrate!SUFFIX!
        )

        echo [BUILD] !GOOS!/!GOARCH! - Seeder...
        go build -ldflags="-s -w" -o !TARGET_DIR!\seed!SUFFIX! .\cmd\seed
        if !errorlevel! neq 0 (
            echo [ERROR] Seeder build failed for !GOOS!/!GOARCH!
        ) else (
            echo [OK] !TARGET_DIR!\seed!SUFFIX!
        )
    )
)

:: Reset environment variables
set GOOS=
set GOARCH=
set CGO_ENABLED=

echo.
echo [DONE] Multi-OS build complete. Binaries are in .\%OUT_DIR%\
endlocal
