$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$env:CARGO_HOME = Join-Path $projectRoot '.cache\cargo'
$env:CARGO_TARGET_DIR = Join-Path $projectRoot '.cache\cargo-target'
$env:GOPATH = Join-Path $projectRoot '.cache\gopath'
$env:GOMODCACHE = Join-Path $projectRoot '.cache\gomod'
$env:GOCACHE = Join-Path $projectRoot '.cache\go-build'

New-Item -ItemType Directory -Path (Join-Path $projectRoot 'bin') -Force | Out-Null
Push-Location $projectRoot
try {
    cargo install egglog --version 0.4.0 --locked --root (Join-Path $projectRoot '.tools')
    if ($LASTEXITCODE -ne 0) { throw 'cargo install egglog failed' }

    go build -o (Join-Path $projectRoot 'bin\egglog-helper.exe') ./cmd/egglog-helper
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }

    Copy-Item -LiteralPath (Join-Path $projectRoot '.tools\bin\egglog.exe') -Destination (Join-Path $projectRoot 'bin\egglog.exe') -Force
    Write-Output "Built $projectRoot\bin\egglog-helper.exe"
} finally {
    Pop-Location
}
