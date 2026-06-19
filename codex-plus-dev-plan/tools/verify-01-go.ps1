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
    "internal\codexplus\configregistry\common.go",
    "internal\codexplus\configregistry\plan_catalog.go",
    "internal\codexplus\configregistry\plan_catalog_test.go",
    "internal\codexplus\configregistry\model_catalog.go",
    "internal\codexplus\configregistry\model_catalog_test.go",
    "internal\codexplus\configregistry\usage_policy.go",
    "internal\codexplus\configregistry\usage_policy_test.go",
    "internal\codexplus\configregistry\feature_flags.go",
    "internal\codexplus\configregistry\feature_flags_test.go",
    "internal\service\codexplus_config_service.go",
    "internal\service\codexplus_config_service_test.go"
)

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "$Name is required for the 01-backend-config-center compile gate but is not available on PATH."
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

    & go test ./internal/codexplus/configregistry ./internal/service -run CodexPlus
    if ($LASTEXITCODE -ne 0) {
        throw "targeted Go tests failed."
    }
} finally {
    Pop-Location
}

Write-Host "01-backend-config-center Go compile gate passed." -ForegroundColor Green
