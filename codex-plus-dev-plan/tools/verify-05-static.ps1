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
    "codex-plus-dev-plan/tools/verify-05-node.ps1",
    "codex-plus-dev-plan/tools/verify-05-go.ps1",
    "codex-plus-dev-plan/05-admin-operations/README.md",
    "codex-plus-dev-plan/05-admin-operations/task-admin-plan-management.md",
    "codex-plus-dev-plan/05-admin-operations/task-admin-model-management.md",
    "codex-plus-dev-plan/05-admin-operations/task-admin-usage-policy-management.md",
    "codex-plus-dev-plan/05-admin-operations/task-admin-user-entitlement-view.md",
    "codex-plus-dev-plan/05-admin-operations/reports/coordinator-admin-ops-verification.md",
    "codex-plus-dev-plan/05-admin-operations/reports/worker-plan-management-final.md",
    "codex-plus-dev-plan/05-admin-operations/reports/worker-model-management-final.md",
    "codex-plus-dev-plan/05-admin-operations/reports/worker-usage-policy-final.md",
    "codex-plus-dev-plan/05-admin-operations/reports/worker-user-entitlement-final.md",
    "sub2api-main/backend/internal/service/codexplus_admin_service.go",
    "sub2api-main/backend/internal/handler/admin/codexplus_handler.go",
    "sub2api-main/backend/internal/server/routes/codexplus_admin.go",
    "sub2api-main/frontend/src/api/admin/codexPlus.ts",
    "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusView.vue",
    "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusPlanCatalogPanel.vue",
    "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusModelCatalogPanel.vue",
    "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusUsagePolicyPanel.vue",
    "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusUserEntitlementPanel.vue"
)) {
    Test-RepoFile $file
}

$ledger = Read-RepoText "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
foreach ($stage in @("00-contract", "01-backend-config-center", "02-backend-client-api", "03-client-cloud-core", "04-client-user-experience")) {
    Add-Check "previous-stage-passed:$stage" ($ledger -match "\| ``$stage`` \|[^\r\n]*\| passed \|") "$stage must be passed before 05."
}
Add-Check "05-active-or-passed" ($ledger -match "\| ``05-admin-operations`` \|[^\r\n]*\| (active|passed) \|") "05-admin-operations must be active during verification or passed after exit."
Add-Check "06-sequential" ($ledger -match "\| ``06-commerce-and-enforcement`` \|[^\r\n]*\| (blocked|active) \|") "06 must be blocked during 05 or active after 05 passes."
Add-Check "07-blocked" ($ledger -match "\| ``07-integration-release`` \|[^\r\n]*\| blocked \|") "07 must remain blocked until 06 passes."

foreach ($taskFile in @(
    "codex-plus-dev-plan/05-admin-operations/task-admin-plan-management.md",
    "codex-plus-dev-plan/05-admin-operations/task-admin-model-management.md",
    "codex-plus-dev-plan/05-admin-operations/task-admin-usage-policy-management.md",
    "codex-plus-dev-plan/05-admin-operations/task-admin-user-entitlement-view.md"
)) {
    Test-RepoText $taskFile "## 解耦要求" "05-task-decoupling:$taskFile"
    Test-RepoText $taskFile "## 禁止改动范围" "05-task-forbidden-scope:$taskFile"
    Test-RepoText $taskFile "## 测试要求" "05-task-tests:$taskFile"
}

foreach ($route in @(
    "/config",
    "/config/validate",
    "/config/publish",
    "/config/versions",
    "/config/rollback",
    "/options",
    "/users/:id/entitlement"
)) {
    Test-RepoText "sub2api-main/backend/internal/server/routes/codexplus_admin.go" ([regex]::Escape($route)) "05-admin-route:$route"
}

foreach ($symbol in @(
    "GetConfig",
    "ValidateConfig",
    "PublishConfig",
    "ListConfigVersions",
    "RollbackConfig",
    "GetOptions",
    "GetUserEntitlement"
)) {
    Test-RepoText "sub2api-main/backend/internal/handler/admin/codexplus_handler.go" $symbol "05-handler-symbol:$symbol"
}

foreach ($symbol in @(
    "GetCurrentConfig",
    "ValidateConfig",
    "PublishConfig",
    "ListConfigVersions",
    "RollbackConfig",
    "GetOptions",
    "GetUserEntitlement"
)) {
    Test-RepoText "sub2api-main/backend/internal/service/codexplus_admin_service.go" $symbol "05-service-symbol:$symbol"
}

foreach ($symbol in @(
    "CodexPlusPlanCatalog",
    "CodexPlusModelCatalog",
    "CodexPlusUsagePolicy",
    "CodexPlusFeatureFlagsDoc",
    "CodexPlusUserEntitlement",
    "publishCodexPlusConfig",
    "getCodexPlusUserEntitlement"
)) {
    Test-RepoText "sub2api-main/frontend/src/api/admin/codexPlus.ts" $symbol "05-frontend-api:$symbol"
}

foreach ($symbol in @(
    "price_amount_minor",
    "entitlement_sources",
    "subscription_group_ids",
    "api_key_group_ids",
    "copy_keys",
    "usage_policy_id",
    "rollout_channel",
    "fallback_model_id",
    "disabled_replacement_model_id",
    "monthly_quota",
    "device_policy",
    "strict_device_enforcement",
    "server_only"
)) {
    Test-RepoText "sub2api-main/frontend/src/api/admin/codexPlus.ts" $symbol "05-shared-api-field:$symbol"
}

foreach ($panel in @(
    "CodexPlusPlanCatalogPanel",
    "CodexPlusModelCatalogPanel",
    "CodexPlusUsagePolicyPanel",
    "CodexPlusUserEntitlementPanel"
)) {
    Test-RepoText "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusView.vue" $panel "05-view-panel:$panel"
}

foreach ($report in @(
    @{ Lane = "Plan"; File = "codex-plus-dev-plan/05-admin-operations/reports/worker-plan-management-final.md" },
    @{ Lane = "Model"; File = "codex-plus-dev-plan/05-admin-operations/reports/worker-model-management-final.md" },
    @{ Lane = "Usage"; File = "codex-plus-dev-plan/05-admin-operations/reports/worker-usage-policy-final.md" },
    @{ Lane = "Entitlement"; File = "codex-plus-dev-plan/05-admin-operations/reports/worker-user-entitlement-final.md" }
)) {
    Test-RepoText $report.File "Report status:\s*final" "05-worker-report-final:$($report.Lane)"
    Test-RepoText $report.File "Worker lane:\s*$($report.Lane)" "05-worker-report-lane:$($report.Lane)"
    Test-RepoText $report.File "## Verification" "05-worker-report-verification:$($report.Lane)"
}

foreach ($symbol in @(
    "price_amount_minor",
    "entitlement_sources",
    "usage_policy_id",
    "copy_keys"
)) {
    Test-RepoText "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusPlanCatalogPanel.vue" $symbol "05-plan-panel-field:$symbol"
}

foreach ($symbol in @(
    "rollout_channel",
    "quality_tier",
    "fallback_model_id",
    "disabled_replacement_model_id",
    "operator_tags"
)) {
    Test-RepoText "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusModelCatalogPanel.vue" $symbol "05-model-panel-field:$symbol"
}

foreach ($symbol in @(
    "monthly_quota",
    "device_policy",
    "copy_keys",
    "Policy preview"
)) {
    Test-RepoText "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusUsagePolicyPanel.vue" $symbol "05-usage-panel-field:$symbol"
}

foreach ($symbol in @(
    "strict_device_enforcement",
    "server-only"
)) {
    Test-RepoText "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusFeatureFlagsPanel.vue" $symbol "05-flags-panel-field:$symbol"
}

foreach ($symbol in @(
    "safeMaskedKey",
    "integration_status",
    "managed_provider_key",
    "recent_events",
    "usage_summary"
)) {
    Test-RepoText "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusUserEntitlementPanel.vue" $symbol "05-entitlement-panel-field:$symbol"
}

foreach ($scriptSymbol in @(
    "npm run typecheck",
    "npm run build",
    "05-admin-operations Node/TypeScript gate passed"
)) {
    Test-RepoText "codex-plus-dev-plan/tools/verify-05-node.ps1" $scriptSymbol "05-node-script:$scriptSymbol"
}

foreach ($scriptSymbol in @(
    "gofmt -l",
    "go test ./internal/service ./internal/handler/admin ./internal/server/routes -run",
    "05-admin-operations Go gate passed"
)) {
    Test-RepoText "codex-plus-dev-plan/tools/verify-05-go.ps1" $scriptSymbol "05-go-script:$scriptSymbol"
}

Test-RepoText "codex-plus-dev-plan/05-admin-operations/reports/coordinator-admin-ops-verification.md" "Report status:\s*final" "05-report-final"
Test-RepoText "codex-plus-dev-plan/05-admin-operations/reports/coordinator-admin-ops-verification.md" "verify-05-static\.ps1" "05-report-static-script"
Test-RepoText "codex-plus-dev-plan/05-admin-operations/reports/coordinator-admin-ops-verification.md" "verify-05-node\.ps1" "05-report-node-script"
Test-RepoText "codex-plus-dev-plan/05-admin-operations/reports/coordinator-admin-ops-verification.md" "verify-05-go\.ps1" "05-report-go-script"

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "05 static audit failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "05 static audit passed."
exit 0
