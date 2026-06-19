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

Push-Location $backendRoot
try {
    $goFiles = @(
        "internal\service\codexplus_gateway_policy_service.go",
        "internal\service\codexplus_gateway_policy_service_test.go",
        "internal\service\codexplus_foundation.go",
        "internal\service\codexplus_client.go",
        "internal\service\codexplus_admin_service.go",
        "internal\handler\codexplus_gateway_policy.go",
        "internal\handler\codexplus_gateway_policy_test.go",
        "internal\handler\admin\codexplus_handler.go",
        "internal\server\routes\codexplus_admin.go",
        "internal\repository\codexplus_foundation_repo.go",
        "internal\repository\codexplus_foundation_repo_test.go"
    )

    $optionalFiles = @(
        "internal\service\codexplus_commerce_entitlement.go",
        "internal\service\codexplus_commerce_entitlement_test.go",
        "internal\service\codexplus_device_management.go",
        "internal\service\codexplus_device_management_test.go",
        "internal\service\codexplus_audit_risk.go",
        "internal\service\codexplus_audit_risk_test.go",
        "internal\service\codexplus_usage_enforcement.go",
        "internal\service\codexplus_usage_enforcement_test.go"
    )
    foreach ($file in $optionalFiles) {
        if (Test-Path -LiteralPath $file -PathType Leaf) {
            $goFiles += $file
        }
    }

    $unformatted = & $gofmt.Source -l @goFiles
    if ($unformatted.Count -gt 0) {
        Write-Host "gofmt -l found unformatted files:" -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host $_ }
        exit 1
    }

    $env:GOTOOLCHAIN = "local"
    Write-Host "Running: go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository -run CodexPlus|Payment|Subscription|Gateway|Device|Audit|Risk"
    & $go.Source test ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository -run "CodexPlus|Payment|Subscription|Gateway|Device|Audit|Risk"
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Pop-Location
}

Write-Host "06-commerce-and-enforcement Go gate passed." -ForegroundColor Green
exit 0
