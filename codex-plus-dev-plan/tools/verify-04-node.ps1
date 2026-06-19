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

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw "npm is required for the 04-client-user-experience Node gate but is not available on PATH."
}

Push-Location $managerRoot
try {
    if (-not (Test-Path -LiteralPath "node_modules\typescript\bin\tsc" -PathType Leaf)) {
        npm ci
        if ($LASTEXITCODE -ne 0) {
            throw "04 npm ci failed."
        }
    }

    npm run check
    if ($LASTEXITCODE -ne 0) {
        throw "04 npm run check failed."
    }
} finally {
    Pop-Location
}

Write-Host "04-client-user-experience Node/TypeScript gate passed." -ForegroundColor Green
