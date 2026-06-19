param(
    [string]$Root,
    [string]$OutputRoot,
    [string]$Timestamp,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $Root "codex-plus-dev-plan\test-runs"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot = Join-Path $Root $OutputRoot
}

if ([string]::IsNullOrWhiteSpace($Timestamp)) {
    $Timestamp = Get-Date -Format "yyyyMMdd-HHmm"
}

if ($Timestamp -match "^\d{8}-\d{4}-e2e$") {
    $runName = $Timestamp
} elseif ($Timestamp -match "^\d{8}-\d{4}$") {
    $runName = "$Timestamp-e2e"
} else {
    throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-e2e."
}

$runPath = Join-Path $OutputRoot $runName
if ((Test-Path -LiteralPath $runPath) -and -not $Force) {
    throw "Evidence run already exists: $runPath. Use -Force only when intentionally regenerating placeholders."
}

New-Item -ItemType Directory -Force -Path $runPath | Out-Null

function Write-EvidenceFile {
    param(
        [string]$Name,
        [string]$Title,
        [string]$Body
    )
    $content = @"
# $Title

Run folder: $runName
Status: TODO

$Body
"@
    Set-Content -LiteralPath (Join-Path $runPath $Name) -Encoding UTF8 -Value $content
}

Write-EvidenceFile "00-environment.md" "00 Environment" @"
## Required Evidence

- TODO: Backend version, gateway version, admin frontend version, desktop build version, contract version, and config version.
- TODO: Base URLs with secrets removed.
- TODO: Test date, executor, environment owner, and Turnstile status.

## Redaction

- TODO: Confirm no real API Key, JWT, refresh token, Authorization header, upstream provider key, poll token, or session token is present.
"@

Write-EvidenceFile "01-test-accounts.md" "01 Test Accounts" @"
## Required Evidence

- TODO: Sanitized account matrix for admin_test, user_active, user_not_purchased, user_expired, user_low_balance, user_device_revoked, and user_model_denied.
- TODO: Entitlement state and missing setup blockers for each account.

## Notes

- TODO: Use test accounts only; do not include real emails unless redacted.
"@

Write-EvidenceFile "02-contract-checks.md" "02 Contract Checks" @"
Result: pending

## Required Evidence

- TODO: Client API contract checks for bootstrap, usage, devices, and redeem.
- TODO: Browser handoff start, complete, and poll contract checks.
- TODO: Any contract mismatch and release impact.
"@

Write-EvidenceFile "03-admin-setup.md" "03 Admin Setup" @"
Result: pending

## Required Evidence

- TODO: Test plan, model policy, default model, usage policy, feature flags, and config version.
- TODO: Admin-visible entitlement, device, and managed key summaries with secrets removed.
"@

Write-EvidenceFile "04-client-api-e2e.md" "04 Client API E2E" @"
Result: pending

## Required Evidence

- TODO: Browser handoff start, browser complete, and desktop poll evidence.
- TODO: Exact `/api/v1/auth/desktop/start`, `/api/v1/auth/desktop/complete`, and `/api/v1/auth/desktop/poll` route observations.
- TODO: Confirm `poll_token` is not present in `authorize_url` and the 6 digit verification code was confirmed in the browser.
- TODO: Bootstrap status for each test user state.
- TODO: Device registration or refresh behavior.
"@

Write-EvidenceFile "05-gateway-policy-e2e.md" "05 Gateway Policy E2E" @"
Result: pending

## Required Evidence

- TODO: One successful user_active request through Sub2API gateway.
- TODO: Structured rejection evidence for no entitlement, expired, insufficient balance, revoked device, and unauthorized model, including safe `request_id`, `GATEWAY_POLICY_*` error code, service status, body parse status, and admin-audit correlation readiness.
- TODO: Confirm blocked users do not get unexpected HTTP success.
"@

Write-EvidenceFile "06-desktop-manager-e2e.md" "06 Desktop Manager E2E" @"
Result: pending

## Required Evidence

- TODO: Manager login, bootstrap fetch, Codex++ Cloud write or repair, and launch evidence.
- TODO: Manual provider preservation evidence.
- TODO: Missing local Codex or config write failure behavior, if applicable.
"@

Write-EvidenceFile "07-package-install-check.md" "07 Package Install Check" @"
Result: pending

## Required Evidence

- TODO: Windows setup exe install, overwrite install, uninstall, and reinstall evidence.
- TODO: macOS x64 and arm64 DMG build, mount, install, launch, overwrite, uninstall, and reinstall evidence.
- TODO: Missing-Codex first-run assistant evidence.
"@

Write-EvidenceFile "08-compatibility-migration.md" "08 Compatibility Migration" @"
Result: pending

## Required Evidence

- TODO: Old user and manual provider compatibility checks.
- TODO: Cloud login, refresh, logout, repair, provider sync, and rollback behavior.
- TODO: Confirm password login is compatibility-only for Turnstile-enabled production-equivalent flow.
"@

Write-EvidenceFile "09-usage-events-audit.md" "09 Usage Events Audit" @"
Result: pending

## Required Evidence

- TODO: Usage rows and admin/audit events for success and rejection paths, including `usage_recorded`, `gateway_policy_rejected`, `GATEWAY_POLICY_*`, `request_id`, `config_version`, and `redaction_applied` signals.
- TODO: Redaction confirmation for every attached log, screenshot, and exported row.

## Redaction

- TODO: Confirm event evidence is redacted.
"@

Write-EvidenceFile "10-rollback-notes.md" "10 Rollback Notes" @"
Result: pending

## Required Evidence

- TODO: Config rollback evidence.
- TODO: Backend rollback evidence.
- TODO: Desktop rollback evidence.
- TODO: Entitlement correction evidence.
- TODO: Leaked user-side Key response process.
- TODO: Failed provider write recovery evidence.
"@

Write-EvidenceFile "11-defects.md" "11 Defects" @"
## Required Evidence

- TODO: P0 defects, owner, impact, mitigation, and release decision effect.
- TODO: P1 defects, owner, impact, mitigation, and release decision effect.
- TODO: P2 defects, owner, impact, mitigation, and release decision effect.
- TODO: P3 defects, owner, impact, mitigation, and release decision effect.
"@

Write-EvidenceFile "12-release-gate-report.md" "12 Release Gate Report" @"
Final recommendation: no-go
Level 3 result: fail

## Commands Executed

- TODO: List exact commands, environments, and result summaries.

## Evidence Links

- TODO: Link sanitized evidence files, screenshots, CI artifacts, or external run records.

## Remaining Risks

- TODO: Summarize P0/P1/P2/P3 risks and release impact.

## Rollback Notes

- TODO: Link rollback notes and owner decisions.
"@

Write-Host "Created 07 E2E evidence scaffold: $runPath"
Write-Host "Fill all TODO items with sanitized execution evidence, then run:"
Write-Host "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-evidence.ps1 -EvidenceDir $runPath"
