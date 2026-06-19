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

$cloudHomePath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudHomeScreen.tsx"
$statusPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudStatusPanel.tsx"
$loginPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudLoginPanel.tsx"
$usagePath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudUsagePanel.tsx"
$installPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudInstallAssistant.tsx"
$tutorialPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudTutorialPanel.tsx"
$diagnosticsPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudDiagnosticsPanel.tsx"
$cssPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/cloud.css"
$commandsPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/cloudCommands.ts"
$typesPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/types.ts"
$reportPath = "codex-plus-dev-plan/04-client-user-experience/reports/coordinator-ui-verification.md"
$ledgerPath = "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"

foreach ($path in @(
    $cloudHomePath,
    $statusPath,
    $loginPath,
    $usagePath,
    $installPath,
    $tutorialPath,
    $diagnosticsPath,
    $cssPath,
    $commandsPath,
    $typesPath,
    $reportPath,
    $ledgerPath,
    "codex-plus-dev-plan/04-client-user-experience/README.md",
    "codex-plus-dev-plan/04-client-user-experience/task-home-dashboard-ui.md",
    "codex-plus-dev-plan/04-client-user-experience/task-login-binding-ui.md",
    "codex-plus-dev-plan/04-client-user-experience/task-install-assistant-ui.md",
    "codex-plus-dev-plan/04-client-user-experience/task-new-user-tutorial-ui.md"
)) {
    Add-StaticCheck "file-exists:$path" (Test-Path -LiteralPath (Join-Path $Root $path) -PathType Leaf) $path
}

Test-RepoText $ledgerPath "\| ``03-client-cloud-core`` \|[^\r\n]*\| passed \|" "03-passed-before-04" "03 must be passed before 04 UI verification."
Test-RepoText $ledgerPath "\| ``04-client-user-experience`` \|[^\r\n]*\| (active|passed) \|" "04-active-or-passed" "04 must be active during UI verification or passed after completion."
foreach ($stageNumber in 5..7) {
    $stagePrefix = "{0:D2}" -f $stageNumber
    $allowed = if ($stageNumber -eq 5) { "(blocked|active)" } else { "blocked" }
    Test-RepoText $ledgerPath "\| ``$stagePrefix-[^``]+`` \|[^\r\n]*\| $allowed \|" "stage-$stagePrefix-sequential" "$stagePrefix must respect sequential gate state after 04 verification."
}

foreach ($symbol in @(
    "CloudStatusPanel",
    "CloudLoginPanel",
    "CloudUsagePanel",
    "CloudInstallAssistant",
    "CloudTutorialPanel",
    "CloudDiagnosticsPanel",
    "feature_flags"
)) {
    Test-RepoText $cloudHomePath $symbol "04-home-symbol:$symbol" "$symbol must be wired into the Cloud home shell."
}

foreach ($symbol in @(
    "advanced_provider_config",
    "renew_action",
    "service?.support_url",
    "data?.plan.renew_url",
    "onLaunch",
    "onOpenAdvancedProviders"
)) {
    Test-RepoText $statusPath $symbol "04-status-symbol:$symbol" "$symbol must be present in service status handling."
}

foreach ($symbol in @(
    "startBrowserHandoff",
    "browserHandoffVerificationCode",
    "pendingTwoFactor",
    "maxLength=\{6\}",
    "replace\(/\\D/g",
    "兼容登录"
)) {
    Test-RepoText $loginPath $symbol "04-login-symbol:$symbol" "$symbol must be present in login binding UI."
}

foreach ($symbol in @(
    "balance_display",
    "period_usage_display",
    "renew_action",
    "model.label",
    "model.disabled_reason"
)) {
    Test-RepoText $usagePath $symbol "04-usage-symbol:$symbol" "$symbol must be present in usage/model display."
}

foreach ($symbol in @(
    "savedAppPath",
    "silentShortcut",
    "managementShortcut",
    "watcherEnabled",
    "onChooseCodexAppPath",
    "onOpenMaintenance"
)) {
    Test-RepoText $installPath $symbol "04-install-symbol:$symbol" "$symbol must be present in install assistant."
}

foreach ($symbol in @(
    "storageKey",
    "localStorage",
    "修 bug",
    "加功能",
    "解释代码"
)) {
    Test-RepoText $tutorialPath $symbol "04-tutorial-symbol:$symbol" "$symbol must be present in tutorial UI."
}

foreach ($symbol in @(
    "grid-template-columns",
    "overflow-wrap",
    "@media",
    "max-width: 980px",
    "max-width: 760px"
)) {
    Test-RepoText $cssPath $symbol "04-css-symbol:$symbol" "$symbol must be present for responsive layout."
}

foreach ($path in @($cloudHomePath, $statusPath, $loginPath, $usagePath, $installPath, $tutorialPath, $diagnosticsPath, $cssPath)) {
    Test-RepoTextNot $path "(?i)\b(TODO|NOT FINAL|not implemented)\b" "no-placeholder:$path" "$path must not contain unfinished markers."
}

foreach ($path in @($cloudHomePath, $statusPath, $loginPath, $usagePath, $installPath, $tutorialPath, $diagnosticsPath)) {
    Test-RepoTextNot $path 'sk-[A-Za-z0-9_-]{12,}|Authorization:\s*Bearer|api_key\s*[:=]\s*[''"]' "no-secret-example:$path" "$path must not contain secret-looking examples."
}

foreach ($reportSymbol in @("Report status:", "verify-04-static.ps1", "verify-04-node.ps1", "04-client-user-experience", "05-admin-operations")) {
    Test-RepoText $reportPath $reportSymbol "04-report:$reportSymbol" "$reportSymbol must be recorded."
}

$checks | Format-Table -AutoSize

$failed = @($checks | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "04 static audit failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "04 static audit passed." -ForegroundColor Green
exit 0
