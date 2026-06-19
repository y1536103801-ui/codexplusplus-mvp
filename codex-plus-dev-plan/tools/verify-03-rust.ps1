param(
    [string]$Root
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$desktopRoot = Join-Path $Root "CodexPlusPlus-main"

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name is required for the 03-client-cloud-core Rust gate but is not available on PATH."
    }
}

Require-Command "cargo"
Require-Command "rustfmt"

Push-Location $desktopRoot
try {
    & cargo fmt --check -p codex-plus-core
    if ($LASTEXITCODE -ne 0) {
        throw "03 Rust format gate failed."
    }

    & cargo test -p codex-plus-core codexplus_cloud
    if ($LASTEXITCODE -ne 0) {
        throw "03 codexplus_cloud Rust tests failed."
    }

    foreach ($filter in @("relay_config", "protocol_proxy")) {
        & cargo test -p codex-plus-core $filter
        if ($LASTEXITCODE -ne 0) {
            throw "03 managed provider/local helper Rust tests failed for $filter."
        }
    }
} finally {
    Pop-Location
}

Write-Host "03-client-cloud-core Rust gate passed." -ForegroundColor Green
