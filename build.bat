@echo off

cd .\server\static\webroot

where node >nul 2>nul
if errorlevel 1 (
    echo Node.js not found, please install Node.js.
    exit /b 1
)

where go >nul 2>nul
if errorlevel 1 (
    echo Go not found, please install Golang.
    exit /b 1
)

for /f "tokens=3" %%v in ('node -v') do set "node_version=%%v"
echo Node.js Version: %node_version:~1%

for /f "tokens=2 delims=." %%v in ('node -v') do set "node_major_version=%%v"
if %node_major_version% lss 20 (
    echo Node.js version is less than 20, check your Node.js version.
    exit /b 1
)

for /f "tokens=3" %%v in ('go version') do set "go_version=%%v"
echo Go Version: %go_version:~2%

for /f "tokens=2 delims=." %%v in ('go version') do set "go_minor_version=%%v"
if %go_minor_version% lss 20 (
    echo Go version is less than 1.20, check your Golang version.
    exit /b 1
)

npm install
npm run build

cd ..\..\..

set CGO_ENABLED=0
go build -o painter
