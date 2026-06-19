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
$filesToFormat = @(
    "internal\service\codexplus_client.go",
    "internal\service\codexplus_client_test.go",
    "internal\service\wire.go",
    "internal\handler\client\client_handler.go",
    "internal\handler\dto\codexplus_client.go",
    "internal\server\routes\client.go"
)

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name is required for the 02-backend-client-api compile gate but is not available on PATH."
    }
}

Require-Command "go"
Require-Command "gofmt"

Push-Location $backendRoot
try {
    $env:GOTOOLCHAIN = "local"
    $requiredGoLine = Get-Content -LiteralPath "go.mod" | Where-Object { $_ -match "^go\s+\d+\.\d+(\.\d+)?" } | Select-Object -First 1
    $requiredGoVersion = ($requiredGoLine -replace "^go\s+", "").Trim()
    $localGoVersion = (& go env GOVERSION).Trim() -replace "^go", ""
    if ($requiredGoVersion -and $localGoVersion -and ([version]$localGoVersion -lt [version]$requiredGoVersion)) {
        throw "Go $requiredGoVersion or newer is required by go.mod, but local Go is $localGoVersion. Install the required toolchain or run this gate in CI with Go $requiredGoVersion+."
    }

    $formatOutput = & gofmt -l $filesToFormat
    if ($LASTEXITCODE -ne 0) {
        throw "gofmt -l failed."
    }
    if ($formatOutput) {
        Write-Host "The following files need gofmt:" -ForegroundColor Red
        $formatOutput | ForEach-Object { Write-Host "  $_" }
        exit 1
    }

    & go test ./internal/service ./internal/handler/client ./internal/handler/dto ./internal/server/routes -run "CodexPlus|Client"
    if ($LASTEXITCODE -ne 0) {
        throw "02 targeted Go tests failed."
    }

    & go test ./internal/service -run "CodexPlusClient|CodexPlusConfig|CodexPlusGateway"
    if ($LASTEXITCODE -ne 0) {
        throw "02 config and gateway integration tests failed."
    }
} finally {
    Pop-Location
}

Write-Host "02-backend-client-api Go compile gate passed." -ForegroundColor Green
