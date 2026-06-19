param(
    [string]$Root
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$checks = New-Object System.Collections.Generic.List[object]

function Add-StaticCheck {
    param(
        [string]$Name,
        [bool]$Passed,
        [string]$Detail
    )
    $script:checks.Add([pscustomobject]@{
        Check = $Name
        Result = if ($Passed) { "PASS" } else { "FAIL" }
        Detail = $Detail
    })
}

function Read-RepoText {
    param([string]$Path)
    return Get-Content -Raw -LiteralPath (Join-Path $Root $Path)
}

function Test-RepoText {
    param(
        [string]$Path,
        [string]$Pattern,
        [string]$Name,
        [string]$Detail
    )
    $text = Read-RepoText $Path
    Add-StaticCheck $Name ($text -match $Pattern) $Detail
}

function Test-RepoTextNot {
    param(
        [string]$Path,
        [string]$Pattern,
        [string]$Name,
        [string]$Detail
    )
    $text = Read-RepoText $Path
    Add-StaticCheck $Name ($text -notmatch $Pattern) $Detail
}

$servicePath = "sub2api-main/backend/internal/service/codexplus_config_service.go"
$serviceTestPath = "sub2api-main/backend/internal/service/codexplus_config_service_test.go"
$commonPath = "sub2api-main/backend/internal/codexplus/configregistry/common.go"
$coordinatorReportPath = "codex-plus-dev-plan/01-backend-config-center/reports/coordinator-integration-static-gate.md"
$ledgerPath = "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"

foreach ($path in @($servicePath, $serviceTestPath, $commonPath, $coordinatorReportPath, $ledgerPath)) {
    Add-StaticCheck "file-exists:$path" (Test-Path -LiteralPath (Join-Path $Root $path) -PathType Leaf) $path
}

foreach ($symbol in @(
    "DefaultPlanCatalog",
    "DefaultModelCatalog",
    "DefaultUsagePolicyCatalog",
    "DefaultFeatureFlags",
    "ValidatePlanCatalog",
    "ValidateModelCatalog",
    "ValidateUsagePolicyCatalog",
    "ValidateFeatureFlags",
    "codexPlusAlignDefaultConfigReferences"
)) {
    Test-RepoText $servicePath $symbol "service-symbol:$symbol" "$symbol must be present in the 01 config service integration."
}

Test-RepoText $servicePath "DisplayPrice:\s*strings\.TrimSpace\(plan\.DisplayPrice\)" "display-price-required" "display_price must be preserved for registry validation, not silently filled."
Test-RepoTextNot $servicePath "DisplayPrice:\s*codexPlusFallbackString\(plan\.DisplayPrice" "display-price-no-fallback" "display_price must not use fallback when building registry catalog."
Test-RepoTextNot $servicePath "display_price\.pending" "display-price-no-pending-copy" "No pending display price copy key should bypass Plan Catalog validation."

foreach ($symbol in @("DraftStatusEditing", "DraftStatusReadyForReview", "DraftStatusApproved")) {
    Test-RepoText $commonPath $symbol "draft-status:$symbol" "$symbol must be recognized by shared configregistry status helpers."
}
Test-RepoText $commonPath "draft_status must be draft, editing, ready_for_review, approved, published, or archived" "draft-status-error-message" "Shared validation message must match the frozen lifecycle."

foreach ($testName in @(
    "TestCodexPlusConfigDefaultUsesRegistryCatalogs",
    "TestCodexPlusConfigValidationUsesRegistryPlanRules",
    "TestCodexPlusConfigValidationUsesRegistryFeatureExposureRules"
)) {
    Test-RepoText $serviceTestPath $testName "service-test:$testName" "$testName must exist."
}

foreach ($risk in @(
    "02-backend-client-api",
    "05-admin-operations",
    "06-commerce-and-enforcement",
    "backend copy keys"
)) {
    Test-RepoText $coordinatorReportPath $risk "downstream-risk:$risk" "$risk must remain recorded as a downstream gate risk."
}

Test-RepoText $ledgerPath "\| ``01-backend-config-center`` \|[^\r\n]*\| (active|passed) \|" "01-active-or-passed" "01 must be active during integration or passed after the Go compile gate."
Test-RepoText $ledgerPath "\| ``02-backend-client-api`` \|[^\r\n]*\| (blocked|active) \|" "stage-02-sequential" "02 may be blocked during 01 integration or active after 01 passes."
foreach ($stageNumber in 3..7) {
    $stagePrefix = "{0:D2}" -f $stageNumber
    Test-RepoText $ledgerPath "\| ``$stagePrefix-[^``]+`` \|[^\r\n]*\| blocked \|" "stage-$stagePrefix-blocked" "$stagePrefix must remain blocked until its prior gate passes."
}

$checks | Format-Table -AutoSize

$failed = @($checks | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "01 static audit failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "01 static audit passed." -ForegroundColor Green
exit 0
