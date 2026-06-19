param(
    [string]$Root
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$frontendRoot = Join-Path $Root "sub2api-main\frontend"
if (-not (Test-Path -LiteralPath (Join-Path $frontendRoot "package.json") -PathType Leaf)) {
    throw "Missing sub2api frontend package.json at $frontendRoot"
}

$npm = Get-Command npm.cmd -ErrorAction SilentlyContinue
if (-not $npm) {
    $npm = Get-Command npm -ErrorAction Stop
}

Push-Location $frontendRoot
try {
    Write-Host "Running: npm run typecheck"
    & $npm.Source run typecheck
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    Write-Host "Running: npm run build"
    & $npm.Source run build
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

Write-Host "05-admin-operations Node/TypeScript gate passed." -ForegroundColor Green
exit 0
