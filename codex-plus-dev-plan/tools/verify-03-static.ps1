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

$apiPath = "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/api.rs"
$localStatePath = "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/local_state.rs"
$providerWriterPath = "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/provider_writer.rs"
$redactionPath = "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/redaction.rs"
$tauriCommandsPath = "CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/src/cloud_commands.rs"
$cloudCommandsPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/cloudCommands.ts"
$typesPath = "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/types.ts"
$protocolProxyPath = "CodexPlusPlus-main/crates/codex-plus-core/src/protocol_proxy.rs"
$relayConfigPath = "CodexPlusPlus-main/crates/codex-plus-core/src/relay_config.rs"
$reportPath = "codex-plus-dev-plan/03-client-cloud-core/reports/coordinator-static-verification.md"
$ledgerPath = "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"

foreach ($path in @(
    $apiPath,
    $localStatePath,
    $providerWriterPath,
    $redactionPath,
    $tauriCommandsPath,
    $cloudCommandsPath,
    $typesPath,
    $protocolProxyPath,
    $relayConfigPath,
    $reportPath,
    $ledgerPath,
    "codex-plus-dev-plan/03-client-cloud-core/README.md",
    "codex-plus-dev-plan/03-client-cloud-core/task-bootstrap-consumer.md",
    "codex-plus-dev-plan/03-client-cloud-core/task-managed-provider-writer.md",
    "codex-plus-dev-plan/03-client-cloud-core/task-local-session-store.md",
    "codex-plus-dev-plan/03-client-cloud-core/task-error-and-log-redaction.md"
)) {
    Add-StaticCheck "file-exists:$path" (Test-Path -LiteralPath (Join-Path $Root $path) -PathType Leaf) $path
}

foreach ($symbol in @(
    "BootstrapSnapshot",
    "message_key",
    "commerce_action",
    "action_copy_key",
    "strict_device_enforcement",
    "force_update_prompt",
    "balance_display",
    "usage_display",
    "X-CodexPlus-Device-Id"
)) {
    Test-RepoText $apiPath $symbol "api-contract:$symbol" "$symbol must be represented by the desktop bootstrap/usage consumer."
}

foreach ($symbol in @(
    "CloudLocalStore",
    "session_from_login",
    "session_from_pending_handoff",
    "state_with_usage",
    "json_string_from_keys",
    "message_key",
    "action_copy_key",
    "strict_device_enforcement",
    "state_projects_backend_action_fields_without_provider_key"
)) {
    Test-RepoText $localStatePath $symbol "local-state:$symbol" "$symbol must be present in the local state projection."
}

foreach ($symbol in @(
    "MANAGED_PROVIDER_ID",
    "Codex\+\+ Cloud",
    "auth_contents",
    "OPENAI_API_KEY",
    "upstream_base_url",
    "local_responses_proxy_base_url",
    "managed_profile_routes_codex_through_local_proxy",
    "managed_profile_does_not_serialize_api_key_field"
)) {
    Test-RepoText $providerWriterPath $symbol "provider-writer:$symbol" "$symbol must be present in managed provider writing."
}

foreach ($symbol in @(
    "codexplus_proxy_auth_api_key",
    "X-CodexPlus-Device-Id",
    "codexplus_upstream_base_url"
)) {
    Test-RepoText $protocolProxyPath $symbol "protocol-proxy:$symbol" "$symbol must be present for local helper enforcement."
}

foreach ($symbol in @(
    "polltoken",
    "sessiontoken",
    "authorization",
    "sk-",
    "redacts_token_patterns_and_url_query_values"
)) {
    Test-RepoText $redactionPath $symbol "redaction:$symbol" "$symbol must be covered by redaction utilities."
}

foreach ($symbol in @(
    "codexplus_cloud_refresh_bootstrap",
    "codexplus_cloud_apply_managed_provider",
    "codexplus_cloud_read_redacted_diagnostics",
    "redact_error"
)) {
    Test-RepoText $tauriCommandsPath $symbol "tauri-command:$symbol" "$symbol must be exposed to the desktop shell."
}

foreach ($symbol in @(
    "message_key",
    "commerce_action",
    "action_copy_key",
    "strict_device_enforcement",
    "actionUrl",
    "actionLabel"
)) {
    Test-RepoText $cloudCommandsPath $symbol "manager-adapter:$symbol" "$symbol must be bridged into the Manager adapter."
}
foreach ($symbol in @(
    "message_key",
    "commerce_action",
    "action_copy_key",
    "force_update_prompt",
    "strict_device_enforcement"
)) {
    Test-RepoText $typesPath $symbol "manager-types:$symbol" "$symbol must be typed for downstream UX."
}

foreach ($path in @($apiPath, $localStatePath, $providerWriterPath, $redactionPath, $cloudCommandsPath, $typesPath)) {
    Test-RepoTextNot $path "(?i)\b(TODO|PLACEHOLDER)\b" "no-placeholder:$path" "$path must not contain unfinished markers."
}

Test-RepoText $ledgerPath "\| ``02-backend-client-api`` \|[^\r\n]*\| passed \|" "02-passed-before-03" "02 must be passed before 03."
Test-RepoText $ledgerPath "\| ``03-client-cloud-core`` \|[^\r\n]*\| (active|passed) \|" "03-active-or-passed" "03 must be active during verification or passed after completion."
Test-RepoText $ledgerPath "\| ``04-client-user-experience`` \|[^\r\n]*\| (blocked|active) \|" "04-sequential" "04 must remain blocked during 03 or become active after 03 passes."

$checks | Format-Table -AutoSize

$failed = @($checks | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "03 static audit failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "03 static audit passed." -ForegroundColor Green
exit 0
