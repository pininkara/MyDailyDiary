param(
    [string]$OutputDir = "dist-package",
    [string]$Goos = "linux",
    [string]$Goarch = "amd64",
    [switch]$SkipFrontendInstall,
    [switch]$Zip
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
$OutPath = Join-Path $Root $OutputDir
$FrontendPath = Join-Path $Root "frontend"
$BinaryName = "diary"
$BinaryPath = Join-Path $OutPath $BinaryName

Write-Host "==> Cleaning output: $OutPath"
if (Test-Path $OutPath) {
    Remove-Item $OutPath -Recurse -Force
}
New-Item -ItemType Directory -Path $OutPath | Out-Null

Write-Host "==> Building frontend"
Push-Location $FrontendPath
try {
    if (-not $SkipFrontendInstall) {
        if (Test-Path "package-lock.json") {
            npm ci
        } else {
            npm install
        }
    }
    npm run build
} finally {
    Pop-Location
}

Write-Host "==> Building Go binary for $Goos/$Goarch"
Push-Location $Root
try {
    $env:GOOS = $Goos
    $env:GOARCH = $Goarch
    $env:CGO_ENABLED = "0"
    go build -trimpath -ldflags "-s -w" -o $BinaryPath .
} finally {
    Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
    Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}

Write-Host "==> Copying runtime assets"
New-Item -ItemType Directory -Path (Join-Path $OutPath "frontend") | Out-Null
Copy-Item -Path (Join-Path $FrontendPath "dist") -Destination (Join-Path $OutPath "frontend\dist") -Recurse
Copy-Item -Path (Join-Path $Root "config.sample.toml") -Destination (Join-Path $OutPath "config.sample.toml")
Copy-Item -Path (Join-Path $Root "Dockerfile.prebuilt") -Destination (Join-Path $OutPath "Dockerfile")

$Readme = @"
# Diary prebuilt package

Build image on server:

```bash
docker build -t diary:latest .
```

Run:

```bash
mkdir -p data
cp config.sample.toml data/config.toml
# edit data/config.toml
docker run -d --name diary -p 8080:8080 -v `$PWD/data:/app/data diary:latest
```
"@
Set-Content -Path (Join-Path $OutPath "README.deploy.md") -Value $Readme -Encoding UTF8

if ($Zip) {
    $ZipPath = Join-Path $Root "$OutputDir.zip"
    if (Test-Path $ZipPath) {
        Remove-Item $ZipPath -Force
    }
    Write-Host "==> Creating zip: $ZipPath"
    Compress-Archive -Path (Join-Path $OutPath "*") -DestinationPath $ZipPath
}

Write-Host "==> Done"
Write-Host "Upload this folder to your server: $OutPath"
if ($Zip) {
    Write-Host "Upload this archive to your server: $(Join-Path $Root "$OutputDir.zip")"
}
