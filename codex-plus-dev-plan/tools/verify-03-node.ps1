param(
    [string]$Root
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$managerRoot = Join-Path $Root "CodexPlusPlus-main\apps\codex-plus-manager"

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name is required for the 03-client-cloud-core Node gate but is not available on PATH."
    }
}

Require-Command "npm"

Push-Location $managerRoot
try {
    if (-not (Test-Path -LiteralPath "node_modules\.bin\tsc.cmd" -PathType Leaf)) {
        throw "TypeScript is not installed under node_modules. Run npm ci in CodexPlusPlus-main/apps/codex-plus-manager before this gate."
    }
    & npm run check
    if ($LASTEXITCODE -ne 0) {
        throw "03 Manager TypeScript check failed."
    }
} finally {
    Pop-Location
}

Write-Host "03-client-cloud-core Node/TypeScript gate passed." -ForegroundColor Green
