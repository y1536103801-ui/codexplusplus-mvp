param(
    [string]$Root
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$results = New-Object System.Collections.Generic.List[object]

function Add-Check {
    param(
        [string]$Name,
        [bool]$Passed,
        [string]$Detail
    )
    $script:results.Add([pscustomobject]@{
        Check = $Name
        Result = if ($Passed) { "PASS" } else { "FAIL" }
        Detail = $Detail
    })
}

function Read-RepoText {
    param([string]$RelativePath)
    return Get-Content -Raw -LiteralPath (Join-Path $Root $RelativePath)
}

function Test-RepoFile {
    param([string]$RelativePath)
    Add-Check "file-exists:$RelativePath" (Test-Path -LiteralPath (Join-Path $Root $RelativePath) -PathType Leaf) $RelativePath
}

function Test-RepoText {
    param(
        [string]$RelativePath,
        [string]$Pattern,
        [string]$CheckName
    )
    $text = Read-RepoText $RelativePath
    Add-Check $CheckName ($text -match $Pattern) "$Pattern in $RelativePath"
}

foreach ($file in @(
    "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
    "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
    "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md",
    "codex-plus-dev-plan/tools/verify-06-go.ps1",
    "codex-plus-dev-plan/06-commerce-and-enforcement/README.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/task-payment-entitlement-flow.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/task-gateway-policy-enforcement.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/task-device-management.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/task-audit-and-risk-control.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/reports/README.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/reports/coordinator-commerce-enforcement-verification.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-payment-entitlement-final.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-gateway-enforcement-final.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-device-management-final.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-audit-risk-final.md",
    "sub2api-main/backend/internal/service/codexplus_commerce_entitlement.go",
    "sub2api-main/backend/internal/service/codexplus_commerce_entitlement_test.go",
    "sub2api-main/backend/internal/service/codexplus_device_management.go",
    "sub2api-main/backend/internal/service/codexplus_device_management_test.go",
    "sub2api-main/backend/internal/service/codexplus_audit_risk.go",
    "sub2api-main/backend/internal/service/codexplus_audit_risk_test.go",
    "sub2api-main/backend/internal/service/codexplus_gateway_policy_service.go",
    "sub2api-main/backend/internal/service/codexplus_gateway_policy_service_test.go",
    "sub2api-main/backend/internal/handler/codexplus_gateway_policy.go",
    "sub2api-main/backend/internal/handler/codexplus_gateway_policy_test.go",
    "sub2api-main/backend/internal/service/codexplus_foundation.go",
    "sub2api-main/backend/internal/repository/codexplus_foundation_repo.go",
    "sub2api-main/backend/internal/repository/codexplus_foundation_repo_test.go",
    "sub2api-main/backend/internal/service/codexplus_config_service.go",
    "sub2api-main/backend/internal/service/codexplus_client.go"
)) {
    Test-RepoFile $file
}

$ledger = Read-RepoText "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
foreach ($stage in @("00-contract", "01-backend-config-center", "02-backend-client-api", "03-client-cloud-core", "04-client-user-experience", "05-admin-operations")) {
    Add-Check "previous-stage-passed:$stage" ($ledger -match "\| ``$stage`` \|[^\r\n]*\| passed \|") "$stage must be passed before 06."
}
Add-Check "06-active-or-passed" ($ledger -match "\| ``06-commerce-and-enforcement`` \|[^\r\n]*\| (active|passed) \|") "06-commerce-and-enforcement must be active during verification or passed after exit."
Add-Check "07-sequential" ($ledger -match "\| ``07-integration-release`` \|[^\r\n]*\| (blocked|active) \|") "07 must be blocked during 06 or active after 06 passes."

foreach ($taskFile in @(
    "codex-plus-dev-plan/06-commerce-and-enforcement/task-payment-entitlement-flow.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/task-gateway-policy-enforcement.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/task-device-management.md",
    "codex-plus-dev-plan/06-commerce-and-enforcement/task-audit-and-risk-control.md"
)) {
    Test-RepoText $taskFile "## 解耦要求" "06-task-decoupling:$taskFile"
    Test-RepoText $taskFile "## 禁止改动范围" "06-task-forbidden-scope:$taskFile"
    Test-RepoText $taskFile "## 测试要求" "06-task-tests:$taskFile"
}

foreach ($symbol in @("Status:\s*(active|passed)", "05-admin-operations", "07-integration-release")) {
    Test-RepoText "codex-plus-dev-plan/06-commerce-and-enforcement/README.md" $symbol "06-readme:$symbol"
}

foreach ($report in @(
    @{ Lane = "Payment"; File = "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-payment-entitlement-final.md" },
    @{ Lane = "Gateway"; File = "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-gateway-enforcement-final.md" },
    @{ Lane = "Device"; File = "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-device-management-final.md" },
    @{ Lane = "Audit"; File = "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-audit-risk-final.md" }
)) {
    Test-RepoText $report.File "Report status:\s*final" "06-worker-report-final:$($report.Lane)"
    Test-RepoText $report.File "Worker lane:\s*$($report.Lane)" "06-worker-report-lane:$($report.Lane)"
    Test-RepoText $report.File "## Verification" "06-worker-report-verification:$($report.Lane)"
}

foreach ($symbol in @("CodexPlusCommerceEntitlementService", "ResolveSubscriptionOrder", "AlreadyGranted", "RecordGrant")) {
    Test-RepoText "sub2api-main/backend/internal/service/codexplus_commerce_entitlement.go" $symbol "06-payment-symbol:$symbol"
}

foreach ($symbol in @("CodexPlusGatewayPolicyService", "Evaluate", "StrictDeviceEnforcement", "CodexPlusGatewayPolicyDecision", "CodexPlusEventCreate")) {
    Test-RepoText "sub2api-main/backend/internal/service/codexplus_gateway_policy_service.go" $symbol "06-gateway-policy-symbol:$symbol"
}

foreach ($symbol in @("resolveCodexPlusGatewayPolicy", "checkUsagePolicy", "BuildCodexPlusGatewayPolicyUsagePayload", "NormalizeCodexPlusAuditRiskPayload")) {
    Test-RepoText "sub2api-main/backend/internal/service/codexplus_gateway_policy_service.go" $symbol "06-gateway-enforcement-symbol:$symbol"
}

foreach ($symbol in @("CodexPlusDevice", "CodexPlusEvent", "CodexPlusDeviceRepository", "CodexPlusEventRepository")) {
    Test-RepoText "sub2api-main/backend/internal/service/codexplus_foundation.go" $symbol "06-foundation-symbol:$symbol"
}

foreach ($symbol in @("CodexPlusAdminService", "RevokeUserDevice", "RestoreUserDevice", "ListUserDevices")) {
    Test-RepoText "sub2api-main/backend/internal/service/codexplus_device_management.go" $symbol "06-device-management-symbol:$symbol"
}

foreach ($symbol in @("CodexPlusAuditRiskEventCreateInput", "RecordCodexPlusAuditRiskEvent", "QueryCodexPlusAuditRiskEvents", "RedactCodexPlusAuditRiskMetadata")) {
    Test-RepoText "sub2api-main/backend/internal/service/codexplus_audit_risk.go" $symbol "06-audit-risk-symbol:$symbol"
}

foreach ($symbol in @("entitlement_sources", "usage_policy_id", "strict_device_enforcement", "device_policy")) {
    Test-RepoText "sub2api-main/backend/internal/service/codexplus_config_service.go" $symbol "06-config-symbol:$symbol"
}

Test-RepoText "codex-plus-dev-plan/06-commerce-and-enforcement/reports/coordinator-commerce-enforcement-verification.md" "Report status:\s*final" "06-report-final"
Test-RepoText "codex-plus-dev-plan/06-commerce-and-enforcement/reports/coordinator-commerce-enforcement-verification.md" "verify-06-static\.ps1" "06-report-static-script"
Test-RepoText "codex-plus-dev-plan/06-commerce-and-enforcement/reports/coordinator-commerce-enforcement-verification.md" "verify-06-go\.ps1" "06-report-go-script"

foreach ($scriptSymbol in @(
    "gofmt -l",
    "go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository -run",
    "06-commerce-and-enforcement Go gate passed"
)) {
    Test-RepoText "codex-plus-dev-plan/tools/verify-06-go.ps1" $scriptSymbol "06-go-script:$scriptSymbol"
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "06 static audit failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "06 static audit passed."
exit 0
