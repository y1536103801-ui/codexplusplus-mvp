param(
    [string]$Root
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$backendRoot = Join-Path $Root "sub2api-main\backend"
if (-not (Test-Path -LiteralPath (Join-Path $backendRoot "go.mod") -PathType Leaf)) {
    throw "Missing sub2api backend go.mod at $backendRoot"
}

$go = Get-Command go -ErrorAction Stop
$gofmt = Get-Command gofmt -ErrorAction Stop
$goFiles = @(
    "internal\service\codexplus_admin_service.go",
    "internal\handler\admin\codexplus_handler.go",
    "internal\server\routes\codexplus_admin.go"
)

Push-Location $backendRoot
try {
    $unformatted = & $gofmt.Source -l @goFiles
    if ($unformatted.Count -gt 0) {
        Write-Host "gofmt -l found unformatted files:" -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host $_ }
        exit 1
    }

    $env:GOTOOLCHAIN = "local"
    Write-Host "Running: go test ./internal/service ./internal/handler/admin ./internal/server/routes -run CodexPlus|Admin"
    & $go.Source test ./internal/service ./internal/handler/admin ./internal/server/routes -run "CodexPlus|Admin"
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

Write-Host "05-admin-operations Go gate passed." -ForegroundColor Green
exit 0
