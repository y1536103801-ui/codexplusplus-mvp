param(
    [string]$Root,
    [string]$OutputRoot,
    [switch]$KeepArtifacts,
    [switch]$SkipHandoff
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $env:TEMP ("codexplus-07-evidence-tooling-test-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
} elseif (-not [System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot = Join-Path $Root $OutputRoot
}

$OutputRoot = [System.IO.Path]::GetFullPath($OutputRoot)
$TempRoot = [System.IO.Path]::GetFullPath($env:TEMP).TrimEnd('\', '/')
if (-not $OutputRoot.StartsWith($TempRoot, [System.StringComparison]::OrdinalIgnoreCase) -and -not $KeepArtifacts) {
    throw "Refusing to auto-delete non-temp output root without -KeepArtifacts: $OutputRoot"
}

if (Test-Path -LiteralPath $OutputRoot) {
    Remove-Item -LiteralPath $OutputRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null

$results = New-Object System.Collections.Generic.List[object]

function Add-Result {
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

function Invoke-Tool {
    param(
        [string]$Name,
        [string]$ScriptName,
        [string[]]$Arguments,
        [int[]]$ExpectedExitCodes
    )
    $scriptPath = Join-Path $PSScriptRoot $ScriptName
    $logPath = Join-Path $OutputRoot ($Name + ".log")
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath @Arguments *> $logPath
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    $exitCode = $LASTEXITCODE
    $passed = $ExpectedExitCodes -contains $exitCode
    Add-Result $Name $passed "exit=$exitCode; expected=$($ExpectedExitCodes -join ','); log=$logPath"
    if (-not $passed) {
        $tail = Get-Content -LiteralPath $logPath -Tail 80
        throw "$Name failed unexpectedly. Log tail:`n$($tail -join [Environment]::NewLine)"
    }
}

function Write-EvidenceFile {
    param(
        [string]$Directory,
        [string]$Name,
        [string]$Content
    )
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null
    Set-Content -LiteralPath (Join-Path $Directory $Name) -Encoding UTF8 -Value $Content
}

function Update-RunFolderDeclarations {
    param(
        [string]$Directory,
        [string]$RunFolder
    )
    Get-ChildItem -LiteralPath $Directory -Recurse -File -Filter "*.md" | ForEach-Object {
        $text = Get-Content -Raw -LiteralPath $_.FullName
        $updated = $text -replace "(?im)^\s*Run folder\s*:\s*\S+\s*$", "Run folder: $RunFolder"
        if ($updated -ne $text) {
            Set-Content -LiteralPath $_.FullName -Encoding UTF8 -Value $updated
        }
    }
}

function Write-ValidE2EFixture {
    param([string]$Directory)
    $files = @{
        "00-environment.md" = @"
# 00 Environment

Run folder: 20260618-1911-e2e
Status: executed

Backend version: test-build-a
Gateway version: test-build-a
Admin frontend version: test-build-a
Desktop build version: test-build-a
Contract version: contract-a
Config version: config-a
Base URLs are redacted.
Test date, executor, environment owner, and Turnstile enabled state recorded.
Redaction reviewed: no real API Key, JWT, refresh token, Authorization header, upstream provider key, poll token, or session token is present.
"@
        "01-test-accounts.md" = @"
# 01 Test Accounts

Run folder: 20260618-1911-e2e
Status: executed

Sanitized account matrix covers admin_test, user_active, user_not_purchased, user_expired, user_low_balance, user_device_revoked, and user_model_denied.
Entitlement state for each account is recorded with identifiers redacted.
Only test accounts were used.
"@
        "02-contract-checks.md" = @"
# 02 Contract Checks

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

Client API contract checks covered bootstrap, usage, devices, and redeem.
Browser handoff start, complete, and poll contract checks matched the frozen contract.
Any contract mismatch section states none observed in this sanitized fixture.
"@
        "03-admin-setup.md" = @"
# 03 Admin Setup

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

Test plan, model policy, default model, usage policy, feature flags, and config version were recorded.
Admin-visible entitlement, device, and managed key summaries were captured with secrets redacted.
"@
        "04-client-api-e2e.md" = @"
# 04 Client API E2E

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

Browser handoff start, browser complete, and desktop poll evidence were captured.
`/api/v1/auth/desktop/start`, `/api/v1/auth/desktop/complete`, and `/api/v1/auth/desktop/poll` were exercised in order.
The authorize_url did not contain poll_token, and the 6 digit verification code was confirmed before browser approval.
Bootstrap status was captured for each test user state.
Device registration and refresh behavior were captured.
"@
        "05-gateway-policy-e2e.md" = @"
# 05 Gateway Policy E2E

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

One successful user_active request completed through the Sub2API gateway.
Rejection evidence covers no entitlement, expired entitlement, insufficient balance, revoked device, and unauthorized model.
Blocked users did not receive unexpected HTTP success.
Structured gateway policy rows include request_id, service_status, ready-for-admin-audit correlation, and GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED, GATEWAY_POLICY_ENTITLEMENT_EXPIRED, GATEWAY_POLICY_BALANCE_INSUFFICIENT, GATEWAY_POLICY_DEVICE_REVOKED, and GATEWAY_POLICY_MODEL_NOT_ALLOWED error codes.
"@
        "06-desktop-manager-e2e.md" = @"
# 06 Desktop Manager E2E

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

Manager login, bootstrap fetch, Codex++ Cloud write or repair, and Codex launch evidence were captured.
Manager login result: pass
Codex++ Cloud provider result: pass
Manual provider preservation result: pass
Codex launch result: pass
Manual providers remained present after the managed provider write.
Missing local Codex and config write failure behavior was recorded where applicable.
"@
        "07-package-install-check.md" = @"
# 07 Package Install Check

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

Windows setup exe install, overwrite install, uninstall, and reinstall evidence were captured.
macOS x64 DMG build, mount, install, launch, overwrite, uninstall, and reinstall evidence were captured.
macOS arm64 DMG build, mount, install, launch, overwrite, uninstall, and reinstall evidence were captured.
Missing Codex first-run assistant evidence was captured.
"@
        "08-compatibility-migration.md" = @"
# 08 Compatibility Migration

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

Old user and manual providers compatibility checks were captured.
Cloud login, refresh, logout, repair, provider sync, and rollback behavior were captured.
Password login was recorded as compatibility-only for Turnstile-enabled production-equivalent flow.
"@
        "09-usage-events-audit.md" = @"
# 09 Usage Events Audit

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

Usage rows and admin audit events were captured for success and rejection paths.
Admin audit correlation captured usage_recorded and gateway_policy_rejected rows.
Structured admin audit rows include GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED, GATEWAY_POLICY_ENTITLEMENT_EXPIRED, GATEWAY_POLICY_BALANCE_INSUFFICIENT, GATEWAY_POLICY_DEVICE_REVOKED, and GATEWAY_POLICY_MODEL_NOT_ALLOWED.
Gateway request_id correlation: pass
Each admin event row matched gateway request_id from 05-gateway-policy-e2e.md.
Each audited row includes request_id, config_version, service_status, redaction_applied, and redacted event evidence before sharing.
Redaction confirmation is attached for logs, screenshots, and exported rows.
"@
        "10-rollback-notes.md" = @"
# 10 Rollback Notes

Run folder: 20260618-1911-e2e
Status: executed
Result: pass

Config rollback evidence captured.
Backend rollback evidence captured.
Desktop rollback evidence captured.
Entitlement correction evidence captured.
Leaked user-side Key response process recorded with values redacted.
Failed provider write recovery evidence captured.
"@
        "11-defects.md" = @"
# 11 Defects

Run folder: 20260618-1911-e2e
Status: executed

P0 defects: none observed in this sanitized fixture.
P1 defects: none observed in this sanitized fixture.
P2 defects: accepted with owner and mitigation.
P3 defects: tracked with owner and release decision effect.
"@
        "12-release-gate-report.md" = @"
# 12 Release Gate Report

Run folder: 20260618-1911-e2e
Status: executed

Final recommendation: go with accepted risks
Level 3 result: pass

## Commands Executed

- Sanitized command list recorded for backend, gateway, admin frontend, desktop, package, compatibility, and audit checks.

## Evidence Links

- Sanitized logs, screenshots, CI artifacts, and platform notes are linked by internal evidence identifiers.

## Remaining Risks

- P2 and P3 risks are summarized with owners and mitigations.

## Rollback Notes

- Rollback notes are linked for config, backend, desktop, entitlement, and provider write recovery.
"@
    }
    foreach ($entry in $files.GetEnumerator()) {
        Write-EvidenceFile $Directory $entry.Key $entry.Value
    }
}

function Write-ValidPackageFixture {
    param([string]$Directory)
    $files = @{
        "00-artifact-metadata.md" = @"
# 00 Artifact Metadata

Result: pass
Version/tag: v1.0.0-test
Artifact names: CodexPlusPlus-setup.exe, CodexPlusPlus-x64.dmg, CodexPlusPlus-arm64.dmg
Artifact hashes: sha256 values are redacted but recorded.
Build provenance and CI artifact links are recorded.

## Expected Artifact Coverage

- macos-arm64-dmg: present
- macos-x64-dmg: present
- windows-x64-setup: present
"@
        "01-windows-x64-install.md" = @"
# 01 Windows x64 Install

Result: pass

Fresh install created Desktop Codex++.lnk and Desktop Codex++ 管理工具.lnk shortcuts, Start Menu entries, and Apps and Features registration.
Overwrite install replaced binaries while preserving user data.
Uninstall and reinstall removed package-owned files and recreated entries.
Silent launcher opened without a visible console.
Manager opened login, install assistant, diagnostics, and advanced configuration.
Missing-Codex first-run assistant was confirmed.
- Manager login result: pass
- Manager install assistant result: pass
- Manager diagnostics result: pass
- Manager advanced configuration result: pass
- Missing-Codex first-run assistant behavior result: pass
"@
        "02-macos-x64-dmg.md" = @"
# 02 macOS x64 DMG

Result: pass

DMG mount succeeded on Intel macOS.
Codex++.app and Codex++ 管理工具.app were copied to /Applications and launched.
Silent launcher used hidden Dock behavior with LSUIElement.
Manager UI opened login, install assistant, diagnostics, and advanced configuration.
Missing-Codex first-run assistant was confirmed.
Gatekeeper quarantine behavior was recorded.
Overwrite, remove/uninstall, and reinstall behavior was recorded.
"@
        "03-macos-arm64-dmg.md" = @"
# 03 macOS arm64 DMG

Result: pass

DMG mount succeeded on Apple Silicon macOS.
Codex++.app and Codex++ 管理工具.app were copied to /Applications and launched.
Silent launcher used hidden Dock behavior with LSUIElement.
Manager UI opened login, install assistant, diagnostics, and advanced configuration.
Missing-Codex first-run assistant was confirmed.
Gatekeeper quarantine behavior was recorded.
Overwrite, remove/uninstall, and reinstall behavior was recorded.
"@
        "04-artifact-inspection.md" = @"
# 04 Artifact Inspection

Result: pass

Package does not embed a shared Key: pass.
Package does not embed user credentials: pass.
Package does not embed fixed price, plan, or model policy: pass.
Installer scripts do not write Codex credentials: pass.
Package install does not overwrite existing manual provider configuration: covered by platform install and compatibility evidence; artifact inspection records package hygiene only.

## Commands Executed

- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1 -ArtifactDir <artifact-dir> -EvidenceDir <package-evidence-dir>`

## Artifact Coverage

- macos-arm64-dmg: present
- macos-x64-dmg: present
- windows-x64-setup: present

## High Confidence Scanner Findings

- none

## Installer Script Scan

- CodexPlusPlus-main\scripts\installer\windows\CodexPlusPlus.nsi: no credential-write risk pattern found
- CodexPlusPlus-main\scripts\installer\macos\package-dmg.sh: no credential-write risk pattern found
"@
        "05-package-gate-report.md" = @"
# 05 Package Gate Report

Package evidence result: pass

## Commands Executed

- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1 -ArtifactDir <artifact-dir> -EvidenceDir <package-evidence-dir>`
- Sanitized package, install, overwrite, uninstall, reinstall, and inspection commands are recorded.

## Evidence Links

- Sanitized logs, screenshots, CI artifacts, and platform notes are linked.

## Remaining Risks

- Residual platform-specific risks have owners and mitigations.

## Release Boundary

- Package evidence is ready for Module J hygiene review but does not override E2E, compatibility, or release go/no-go.
"@
    }
    foreach ($entry in $files.GetEnumerator()) {
        Write-EvidenceFile $Directory $entry.Key $entry.Value
    }
}

function Write-ValidCompatibilityFixture {
    param([string]$Directory)
    $files = @{
        "00-test-context.md" = @"
# 00 Test Context

Run folder: 20260618-1911-compatibility
Result: pass
Desktop version before upgrade: 0.9.0-test
Desktop version after upgrade: 1.0.0-test
Legacy settings source and sanitized snapshot path recorded.
Environment owner and rollback owner recorded.

## Snapshot Inputs

- pre-upgrade: parsed; path=sanitized/pre-upgrade.json; manual=legacy-openai, local-dev; managed=none; legacy relayProfiles/settings parsed=True
- post-upgrade: parsed; path=sanitized/post-upgrade.json; manual=legacy-openai, local-dev; managed=Codex++ Cloud; legacy relayProfiles/settings parsed=False
- logout: parsed; path=sanitized/logout.json; manual=legacy-openai, local-dev; managed=Codex++ Cloud; legacy relayProfiles/settings parsed=False
- rollback: parsed; path=sanitized/rollback.json; manual=legacy-openai, local-dev; managed=none; legacy relayProfiles/settings parsed=False

## Snapshot Hygiene

- All snapshots token-field scan clear: True.
- All snapshots commercial-policy scan clear: True.
- Legacy relayProfiles/settings parsed: True.
- Manual provider content comparison uses nonprinted base URL/API key hashes: True.

## Missing Inputs

- none

## Parse Failures

- none
"@
        "01-pre-upgrade-snapshot.md" = @"
# 01 Pre Upgrade Snapshot

Result: pass

Manual provider names and types before upgrade recorded.
Base URLs redacted.
API keys redacted.
Default provider before upgrade recorded.

## Snapshot Inspection

- Pre-upgrade snapshot parsed: True.
- Legacy relayProfiles/settings parsed: True.
- Manual provider count: 2.
- Managed provider count before upgrade: 0.
- Snapshot contents were not copied into this evidence folder.
"@
        "02-post-upgrade-cloud.md" = @"
# 02 Post Upgrade Cloud

Result: pass

Manual providers preserved after upgrade: True.
Manual provider content unchanged after upgrade: True.
Codex++ Cloud provider written or refreshed without overwriting manual providers: True.
No plan, price, multiplier, entitlement, or usage policy data was written by migration: True.
Managed provider runtime-only config: True.
Advanced provider configuration remains reachable: True.

## Snapshot Inspection

- Manual providers before upgrade: legacy-openai, local-dev.
- Manual providers after upgrade: legacy-openai, local-dev.
- Missing manual providers after upgrade: none.
- Manual providers with changed content after upgrade: none.
- Managed providers after upgrade: Codex++ Cloud.
"@
        "03-cloud-logout-boundary.md" = @"
# 03 Cloud Logout Boundary

Result: pass

Cloud login creates only expected cloud session state.
Cloud logout clears cloud session state.
Runtime cloud login/logout evidence result: pass.
Manual providers remain unchanged after logout: True.
Manual provider content unchanged after logout: True.
Redacted before and after provider snapshots are linked.

## Snapshot Inspection

- Manual providers before upgrade: legacy-openai, local-dev.
- Manual providers after logout: legacy-openai, local-dev.
- Missing manual providers after logout: none.
- Manual providers with changed content after logout: none.
- Logout token-field scan clear: True.
"@
        "04-manual-provider-switch.md" = @"
# 04 Manual Provider Switch

Result: pass

Manual provider can still be selected after upgrade.
Manual provider can still be used after managed cloud refresh.
Runtime manual provider selection result: pass.
Runtime manual provider request result: pass.
Manual provider content unchanged after managed cloud refresh: True.
Default user path still shows managed cloud entry point: True.
"@
        "05-provider-sync.md" = @"
# 05 Provider Sync

Result: pass

Provider sync recognizes legacy profiles.
Provider sync does not corrupt manual provider entries.
Runtime provider sync log review result: pass.
Provider sync log secret scan clear: True.
Provider sync logs were redacted.

## Snapshot Inspection

- Changed content after upgrade: none.
- Changed content after logout: none.
"@
        "06-rollback-rehearsal.md" = @"
# 06 Rollback Rehearsal

Result: pass

Config rollback preserves or recovers manual providers.
Runtime rollback rehearsal result: pass.
Manual provider content unchanged after rollback: True.
Desktop rollback keeps advanced provider settings reachable.
Backend/gateway rollback does not force managed-provider-only assumptions.
Failed provider write recovery result: pass.
Failed provider write recovery from last settings snapshot was recorded.

## Snapshot Inspection

- Manual providers before upgrade: legacy-openai, local-dev.
- Manual providers after rollback: legacy-openai, local-dev.
- Missing manual providers after rollback: none.
- Manual providers with changed content after rollback: none.
"@
        "07-compatibility-gate-report.md" = @"
# 07 Compatibility Gate Report

Compatibility evidence result: pass
Compatibility snapshot subset result: pass
Runtime compatibility result: pass

## Commands Executed

- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1 -PreUpgradeSnapshot <path> -PostUpgradeSnapshot <path> -LogoutSnapshot <path> -RollbackSnapshot <path> -EvidenceDir <compatibility-evidence-dir>`
- Sanitized upgrade, login, logout, provider switch, sync, and rollback commands recorded.

## Evidence Links

- Sanitized snapshots, screenshots, logs, diffs, and external run records are linked.

## Remaining Risks

- Compatibility risks have owners and mitigations.

## Release Boundary

- Compatibility evidence is ready for Module J hygiene review but does not override E2E, package, or release go/no-go.
"@
    }
    foreach ($entry in $files.GetEnumerator()) {
        Write-EvidenceFile $Directory $entry.Key $entry.Value
    }
}

function Write-ValidBusinessReadinessFixture {
    param([string]$Directory)
    $content = @"
# 11 Business Readiness

Run folder: 20260618-1912-business
Status: executed
Business readiness result: pass

## Source Documents

- PRODUCTION-ENVIRONMENT-MATRIX.md: required production values resolved for launch candidate.
- BUSINESS-CONFIG-DECISION-TABLE.md: package, model, quota, payment and cost decisions approved.
- SERVER-SIZING-AND-SCALING-GUIDE.md: selected server shape matches launch stage.
- DEPLOYMENT-AUTOMATION-RUNBOOK.md: deployment, backup, rollback and healthcheck paths defined.
- SECURITY-REVIEW-PLAN.md: no open P0/P1 security items.
- COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md: privacy, terms, refund and provider terms defined.
- OBSERVABILITY-SLO-ALERTING-PLAN.md: dashboards, SLO targets and alert routing defined.
- COST-CONTROL-AND-ABUSE-RUNBOOK.md: spend caps, quota enforcement, abuse signals and emergency shutoff defined.
- SUPPORT-OPERATIONS-RUNBOOK.md: paid-user support, refund compensation and admin recovery procedures defined.

## Gate Matrix

| Gate | Status | Owner | Evidence | Deferred risk | Mitigation | Latest decision date |
| --- | --- | --- | --- | --- | --- | --- |
| production environment values | pass | release owner | production matrix approved | none | monitor launch checklist | 2026-06-18 |
| business config decisions | pass | product owner | config decision table approved | none | config rollback owner assigned | 2026-06-18 |
| server sizing and scaling | pass | platform owner | sizing guide matched launch stage | none | scale-up runbook ready | 2026-06-18 |
| deployment automation backup rollback healthcheck | pass | ops owner | deployment runbook dry-run recorded | none | rollback owner assigned | 2026-06-18 |
| security review P0/P1/P2 | pass | security owner | security review signed off | accepted P2 monitoring | monitor release telemetry | 2026-06-18 |
| compliance privacy legal payment provider terms refund policy | pass | legal owner | legal checklist approved | none | review after launch market change | 2026-06-18 |
| observability SLO dashboards alert routing | pass | observability owner | dashboard and alert routing recorded | none | on-call escalation active | 2026-06-18 |
| cost control abuse spend caps emergency shutoff | pass | finance owner | cost abuse runbook approved | none | emergency shutoff rehearsed | 2026-06-18 |
| support operations paid-user support refund compensation admin recovery | pass | support owner | support runbook approved | none | severity routing active | 2026-06-18 |
| human business or legal decisions | pass | project owner | all launch decisions recorded | none | owner escalation path active | 2026-06-18 |

## Required No-Go Scan

- Open P0/P1 security items: none
- Missing production value: none
- Missing first-launch package/model/quota/payment/cost decision: none
- Privacy policy, terms, refund policy and provider terms: defined
- Dashboard, SLO and alert routing: defined
- Cost cap and emergency stop: defined
- Paid-user support and entitlement correction: defined

## Release Boundary

This file records business readiness approval only. It does not execute E2E, build packages, run compatibility migration, or make the technical release go/no-go decision.
"@
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null
    Set-Content -LiteralPath (Join-Path $Directory "11-business-readiness.md") -Encoding UTF8 -Value $content
}

function Write-BusinessSourceDocsFixture {
    param(
        [string]$Directory,
        [switch]$Unresolved
    )
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null

    if ($Unresolved) {
        Set-Content -LiteralPath (Join-Path $Directory "PRODUCTION-ENVIRONMENT-MATRIX.md") -Encoding UTF8 -Value @"
# Codex++ Production Environment Matrix

## Status

- State: draft

| Field | Value | Status |
| --- | --- | --- |
| Provider | TBD | unknown |
| API domain | TBD | waiting user domain |
"@
        Set-Content -LiteralPath (Join-Path $Directory "BUSINESS-CONFIG-DECISION-TABLE.md") -Encoding UTF8 -Value @"
# Codex++ Business Config Decision Table

## Status

- State: draft

| Item | Decision | Status |
| --- | --- | --- |
| Payment provider | TBD | user decision needed |
| Per-user daily cost cap | TBD | decision needed |
"@
        return
    }

    Set-Content -LiteralPath (Join-Path $Directory "PRODUCTION-ENVIRONMENT-MATRIX.md") -Encoding UTF8 -Value @"
# Codex++ Production Environment Matrix

## Status

- State: approved

| Field | Value | Status |
| --- | --- | --- |
| Provider | managed launch provider | known |
| API domain | api.codex.example.com | approved |
| Admin domain | admin.codex.example.com | approved |
| Payment callback | https://api.codex.example.com/api/payment/callback/provider | approved |
"@
    Set-Content -LiteralPath (Join-Path $Directory "BUSINESS-CONFIG-DECISION-TABLE.md") -Encoding UTF8 -Value @"
# Codex++ Business Config Decision Table

## Status

- State: approved

| Item | Decision | Status |
| --- | --- | --- |
| Starter price | launch-owner-approved value | approved |
| Default model | launch-owner-approved model | approved |
| Payment provider | launch-owner-approved provider | approved |
| Per-user daily cost cap | launch-owner-approved cap | approved |
"@
}

function Write-PackageArtifactFixtureSet {
    param(
        [string]$Directory,
        [ValidateSet("clean", "token", "policy")]
        [string]$Mode = "clean"
    )
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null
    $windowsContent = "fixture windows setup artifact without credentials"
    if ($Mode -eq "token") {
        $windowsContent = "fixture artifact with fake scanner token sk-fixture-malicious-token-000000"
    } elseif ($Mode -eq "policy") {
        $windowsContent = '{ "plan_id": "starter", "quota": 100000, "price_amount": 9900, "usage_policy": "fixed" }'
    }

    $fixtures = @{
        "CodexPlusPlus-1.0.0-test-windows-x64-setup.exe" = $windowsContent
        "CodexPlusPlus-1.0.0-test-macos-x64.dmg" = "fixture macos x64 dmg artifact without credentials"
        "CodexPlusPlus-1.0.0-test-macos-arm64.dmg" = "fixture macos arm64 dmg artifact without credentials"
    }
    foreach ($entry in $fixtures.GetEnumerator()) {
        Set-Content -LiteralPath (Join-Path $Directory $entry.Key) -Encoding ASCII -Value $entry.Value
    }
}

function Write-CompatibilitySnapshotFixtureSet {
    param(
        [string]$Directory,
        [ValidateSet("clean", "token", "camelToken", "policy", "changed")]
        [string]$Mode = "clean"
    )
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null

    $postExtra = ""
    if ($Mode -eq "token") {
        $postExtra = ',"access_token":"fixture-session-token"'
    } elseif ($Mode -eq "camelToken") {
        $postExtra = ',"accessToken":"fixture-session-token"'
    } elseif ($Mode -eq "policy") {
        $postExtra = ',"plan_id":"starter","quota":100000,"price_amount_minor":9900'
    }

    $legacyBaseUrl = "https://legacy.example/v1"
    $legacyApiKey = "[redacted]"
    if ($Mode -eq "changed") {
        $legacyBaseUrl = "https://changed.example/v1"
        $legacyApiKey = "[changed-redacted]"
    }

    Set-Content -LiteralPath (Join-Path $Directory "pre-upgrade.json") -Encoding UTF8 -Value @"
{
  "settings": {
    "activeRelayId": "legacy-openai",
    "relayProfiles": [
      { "id": "legacy-openai", "name": "legacy-openai", "baseUrl": "https://legacy.example/v1", "apiKey": "[redacted]" },
      { "id": "local-dev", "name": "local-dev", "baseUrl": "http://127.0.0.1:11434/v1", "apiKey": "[redacted]" }
    ]
  }
}
"@
    Set-Content -LiteralPath (Join-Path $Directory "post-upgrade.json") -Encoding UTF8 -Value @"
{
  "model_provider": "Codex++ Cloud",
  "model_providers": {
    "legacy-openai": { "name": "legacy-openai", "base_url": "$legacyBaseUrl", "api_key": "$legacyApiKey" },
    "local-dev": { "name": "local-dev", "base_url": "http://127.0.0.1:11434/v1", "api_key": "[redacted]" },
    "Codex++ Cloud": { "name": "Codex++ Cloud", "base_url": "http://127.0.0.1:19091/v1", "managed": true, "default_model": "allowed-model-fixture"$postExtra }
  }
}
"@
    Set-Content -LiteralPath (Join-Path $Directory "logout.json") -Encoding UTF8 -Value @"
{
  "model_provider": "legacy-openai",
  "model_providers": {
    "legacy-openai": { "name": "legacy-openai", "base_url": "https://legacy.example/v1", "api_key": "[redacted]" },
    "local-dev": { "name": "local-dev", "base_url": "http://127.0.0.1:11434/v1", "api_key": "[redacted]" },
    "Codex++ Cloud": { "name": "Codex++ Cloud", "base_url": "http://127.0.0.1:19091/v1", "managed": true }
  }
}
"@
    Set-Content -LiteralPath (Join-Path $Directory "rollback.json") -Encoding UTF8 -Value @"
{
  "model_provider": "legacy-openai",
  "model_providers": {
    "legacy-openai": { "name": "legacy-openai", "base_url": "https://legacy.example/v1", "api_key": "[redacted]" },
    "local-dev": { "name": "local-dev", "base_url": "http://127.0.0.1:11434/v1", "api_key": "[redacted]" }
  }
}
"@
}

function Write-PngDimensionFixture {
    param(
        [string]$Path,
        [int]$Width,
        [int]$Height
    )
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    $bytes = New-Object byte[] 33
    $signature = [byte[]](137, 80, 78, 71, 13, 10, 26, 10)
    [Array]::Copy($signature, 0, $bytes, 0, $signature.Length)
    $bytes[11] = 13
    $chunk = [System.Text.Encoding]::ASCII.GetBytes("IHDR")
    [Array]::Copy($chunk, 0, $bytes, 12, $chunk.Length)
    $bytes[16] = [byte](($Width -shr 24) -band 0xff)
    $bytes[17] = [byte](($Width -shr 16) -band 0xff)
    $bytes[18] = [byte](($Width -shr 8) -band 0xff)
    $bytes[19] = [byte]($Width -band 0xff)
    $bytes[20] = [byte](($Height -shr 24) -band 0xff)
    $bytes[21] = [byte](($Height -shr 16) -band 0xff)
    $bytes[22] = [byte](($Height -shr 8) -band 0xff)
    $bytes[23] = [byte]($Height -band 0xff)
    [System.IO.File]::WriteAllBytes($Path, $bytes)
}

function Write-ValidDocsProductCopyFixture {
    param([string]$Directory)
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $Directory "05-html-visual-evidence") | Out-Null

    Write-EvidenceFile $Directory "00-docs-sync-record.md" @"
# 00 Docs Sync Record

Run folder: 20260618-1911-docs
Report status: final
Status: final

Public docs are final and backend-configured for release review.
Control Plane, Data Plane, Client Runtime and Platform Ops wording is aligned across release docs and HTML.
No public copy promises a fixed built-in price, fixed built-in model or fixed built-in quota.
User guide, admin operations guide, release notes and HTML product spec are linked as final evidence.
"@

    Write-EvidenceFile $Directory "01-user-guide.md" @"
# 01 User Guide

Run folder: 20260618-1911-docs
Status: final

Codex++ Cloud install and login are documented with backend-configured configuration snapshot wording.
Old manual provider entries remain available after the managed provider is written.
Failure states covered in order: not purchased, expired, insufficient balance, device revoked, model unavailable and local configuration failure.
"@

    Write-EvidenceFile $Directory "02-admin-operations-guide.md" @"
# 02 Admin Operations Guide

Run folder: 20260618-1911-docs
Status: final

Admins control prices, plans, models and quota through backend-configured operations.
Operational control covers configuration version, canary rollout, rollback, audit history and reconciliation.
Public documents exclude upstream credentials, true cost structure and internal-only risk rules.
"@

    Write-EvidenceFile $Directory "03-release-notes.md" @"
# 03 Release Notes

Run folder: 20260618-1911-docs
Status: final

This is not a draft.
Final release notes cover user and admin changes, rollback paths, compatibility impact and known risks with owners.
"@

    Write-EvidenceFile $Directory "04-html-sync-evidence.md" @"
# 04 HTML Sync Evidence

Run folder: 20260618-1911-docs
Status: final
Result: pass

static sync passed.
local Chromium visual evidence passed.
in-app browser local-file visual evidence passed.
Desktop 1440x900 and Mobile 390x844 screenshots were reviewed without obvious overlap and without right-side text clipping.
fixed-value residue scan: pass.
"@

    Write-EvidenceFile $Directory "codex-plus-product-spec.html" @"
<!doctype html>
<html>
<head><title>Codex++ Cloud</title></head>
<body data-quota-progress="74">
  <section>Codex++ Cloud is backend-configured by the backend for release review.</section>
  <section>Control Plane</section>
  <section>Data Plane</section>
  <section>Client Runtime</section>
  <section>Platform Ops</section>
  <script>
    const quotaProgress = 74;
    document.body.style.setProperty("--quota-progress", quotaProgress + "%");
  </script>
</body>
</html>
"@

    Write-EvidenceFile $Directory "05-html-visual-evidence\visual-review.md" @"
# 05 Visual Review

Run folder: 20260618-1911-docs
Status: final
Result: pass

Desktop 1440x900 screenshot: 05-html-visual-evidence/product-spec-desktop.png
Mobile 390x844 screenshot: 05-html-visual-evidence/product-spec-mobile.png
Visual evidence passed in the approved browser preview.
"@

    Write-PngDimensionFixture (Join-Path $Directory "05-html-visual-evidence\product-spec-desktop.png") 1440 900
    Write-PngDimensionFixture (Join-Path $Directory "05-html-visual-evidence\product-spec-mobile.png") 390 844

    Write-EvidenceFile $Directory "06-docs-product-copy-gate-report.md" @"
# 06 Docs Product Copy Gate Report

Run folder: 20260618-1911-docs
Status: final
Docs product copy result: pass

## Commands Executed

- Static sync, visual review and fixed-value residue scan commands recorded.

## Evidence Links

- Sanitized screenshots, review notes and release-candidate document snapshots are linked.

## Remaining Risks

- No blocking copy or visual evidence risk remains.

## Release Boundary

- Docs product copy evidence is ready for Module J hygiene review but does not override E2E, package, compatibility, business readiness or release go/no-go.
"@
}

function Get-ModuleJFixtureValue {
    param(
        [string]$Name,
        [string]$Default
    )
    $value = Get-Variable -Name $Name -Scope Script -ValueOnly -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $Default
    }
    return $value
}

function Write-ValidModuleJReport {
    param([string]$Path)
    $e2eDir = Get-ModuleJFixtureValue "ModuleJFixtureE2EDir" "codex-plus-dev-plan/test-runs/20260618-1911-e2e"
    $packageDir = Get-ModuleJFixtureValue "ModuleJFixturePackageDir" "codex-plus-dev-plan/test-runs/20260618-1911-package"
    $compatibilityDir = Get-ModuleJFixtureValue "ModuleJFixtureCompatibilityDir" "codex-plus-dev-plan/test-runs/20260618-1911-compatibility"
    $docsDir = Get-ModuleJFixtureValue "ModuleJFixtureDocsDir" "codex-plus-dev-plan/test-runs/20260618-1911-docs"
    $businessDir = Get-ModuleJFixtureValue "ModuleJFixtureBusinessDir" "codex-plus-dev-plan/test-runs/20260618-1911-business"
    $coverageSummary = Get-ModuleJFixtureValue "ModuleJFixtureCoverageSummary" "codex-plus-dev-plan/test-runs/20260618-1911-release/release-coverage-summary.md"
    $readinessSummary = Get-ModuleJFixtureValue "ModuleJFixtureReadinessSummary" "codex-plus-dev-plan/test-runs/20260618-1911-release/release-readiness-summary.md"
    $content = @"
# Module J Final Integration Report

Report status: final
Worker lane: Module J
Forbidden edits: none

## Modules merged

- module reports: Module A final report, Module B final report, Module C final report, Module D final report, Module E final report, Module F final report, Module G final report, Module H final report, Module I final report recorded.
- merge order: A, B, C, D, E, F, H, G, I, docs and HTML sync, final verification and report.
- out of scope modules: none

## Builds and versions

- backend: test-build-a
- admin frontend: test-build-a
- desktop manager: test-build-a
- contract version: contract-a
- config version: config-a

## Conflicts resolved

- file: none
  modules: none
  rule used: ownership matrix reviewed
  result: no unresolved conflict

## Contract changes from original plan

- drift status: none
- affected surface: none
- change review evidence: none required because frozen Phase 1 contract matched sanitized fixture
- owner: release owner
- impact: no contract drift from the frozen Phase 1 contract in this sanitized fixture

## Verification commands

- command: powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1
- result: passed
- evidence: sanitized aggregate verifier log and release evidence folder links
- skipped or unavailable: none
- reason unavailable: none
- replacement narrower check: none
- owner needed: none

## Release evidence hygiene

- verifier: tools/verify-07-release-evidence.ps1
- E2E evidence folder: $e2eDir
- Package evidence folder: $packageDir
- Compatibility evidence folder: $compatibilityDir
- Docs product copy evidence folder: $docsDir
- Business readiness folder: $businessDir
- Release coverage summary: $coverageSummary
- Release readiness summary: $readinessSummary
- Aggregate evidence result: passed
- Coverage summary status: complete
- Docs product copy verification: passed
- Business readiness verification: passed

## E2E result

- decision: go with accepted risks
- evidence folder: $e2eDir
- level 3 result: pass
- happy path: passed in sanitized fixture
- failure paths: not purchased, expired, low balance, revoked device and model denied paths passed in sanitized fixture
- admin config bootstrap: admin config change reflected in bootstrap in sanitized fixture

## Docs and HTML sync

- Docs and HTML sync passed against implemented scope, including backend-configured commercial copy and rollback notes.

## Remaining risks

- severity: P2
  owner: release owner
  impact: accepted platform-specific installer follow-up risk with bounded support impact
  mitigation: monitor platform-specific installer notes after release
  target date: release plus seven days

## Rollback notes

- config rollback: documented and tested in sanitized evidence
- backend rollback: documented and tested in sanitized evidence
- desktop rollback: documented and tested in sanitized evidence
- manual provider recovery: documented and tested in sanitized evidence

## Final recommendation

Recommendation: go with accepted risks
"@
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $content
}

function Write-ModuleJReportMissingAcceptedImpact {
    param([string]$Path)
    $e2eDir = Get-ModuleJFixtureValue "ModuleJFixtureE2EDir" "codex-plus-dev-plan/test-runs/20260618-1911-e2e"
    $packageDir = Get-ModuleJFixtureValue "ModuleJFixturePackageDir" "codex-plus-dev-plan/test-runs/20260618-1911-package"
    $compatibilityDir = Get-ModuleJFixtureValue "ModuleJFixtureCompatibilityDir" "codex-plus-dev-plan/test-runs/20260618-1911-compatibility"
    $docsDir = Get-ModuleJFixtureValue "ModuleJFixtureDocsDir" "codex-plus-dev-plan/test-runs/20260618-1911-docs"
    $businessDir = Get-ModuleJFixtureValue "ModuleJFixtureBusinessDir" "codex-plus-dev-plan/test-runs/20260618-1911-business"
    $coverageSummary = Get-ModuleJFixtureValue "ModuleJFixtureCoverageSummary" "codex-plus-dev-plan/test-runs/20260618-1911-release/release-coverage-summary.md"
    $readinessSummary = Get-ModuleJFixtureValue "ModuleJFixtureReadinessSummary" "codex-plus-dev-plan/test-runs/20260618-1911-release/release-readiness-summary.md"
    $content = @"
# Module J Final Integration Report

Report status: final
Worker lane: Module J
Forbidden edits: none

## Modules merged

- module reports: Module A final report, Module B final report, Module C final report, Module D final report, Module E final report, Module F final report, Module G final report, Module H final report, Module I final report recorded.
- merge order: A, B, C, D, E, F, H, G, I, docs and HTML sync, final verification and report.
- out of scope modules: none

## Builds and versions

- backend: test-build-a
- admin frontend: test-build-a
- desktop manager: test-build-a
- contract version: contract-a
- config version: config-a

## Conflicts resolved

- file: none
  modules: none
  rule used: ownership matrix reviewed
  result: no unresolved conflict

## Contract changes from original plan

- drift status: none
- affected surface: none
- change review evidence: none required because frozen Phase 1 contract matched sanitized execution evidence
- owner: release owner
- impact: no contract drift from the frozen Phase 1 contract in this sanitized execution evidence

## Verification commands

- command: powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1
- result: passed
- evidence: sanitized aggregate verifier log and release evidence folder links
- skipped or unavailable: none
- reason unavailable: none
- replacement narrower check: none
- owner needed: none

## Release evidence hygiene

- verifier: tools/verify-07-release-evidence.ps1
- E2E evidence folder: $e2eDir
- Package evidence folder: $packageDir
- Compatibility evidence folder: $compatibilityDir
- Docs product copy evidence folder: $docsDir
- Business readiness folder: $businessDir
- Release coverage summary: $coverageSummary
- Release readiness summary: $readinessSummary
- Aggregate evidence result: passed
- Coverage summary status: complete
- Docs product copy verification: passed
- Business readiness verification: passed

## E2E result

- decision: go with accepted risks
- evidence folder: $e2eDir
- level 3 result: pass
- happy path: passed in sanitized execution evidence
- failure paths: not purchased, expired, low balance, revoked device and model denied paths passed in sanitized execution evidence
- admin config bootstrap: admin config change reflected in bootstrap in sanitized execution evidence

## Docs and HTML sync

- Docs and HTML sync passed against implemented scope, including backend-configured commercial copy and rollback notes.

## Remaining risks

- severity: P2
  owner: release owner
  mitigation: monitor platform-specific installer notes after release
  target date: release plus seven days

## Rollback notes

- config rollback: documented and tested in sanitized evidence
- backend rollback: documented and tested in sanitized evidence
- desktop rollback: documented and tested in sanitized evidence
- manual provider recovery: documented and tested in sanitized evidence

## Final recommendation

Recommendation: go with accepted risks
"@
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $content
}

function Write-ModuleJReportMissingConflictResolution {
    param([string]$Path)
    Write-ValidModuleJReport $Path
    $text = Get-Content -Raw -LiteralPath $Path
    $text = $text -replace "(?ms)^## Conflicts resolved\s+- file: none\s+modules: none\s+rule used: ownership matrix reviewed\s+result: no unresolved conflict", @"
## Conflicts resolved

- file: none
  modules: none
"@
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $text
}

function Write-ModuleJReportMissingVerificationAvailability {
    param([string]$Path)
    Write-ValidModuleJReport $Path
    $text = Get-Content -Raw -LiteralPath $Path
    $text = $text -replace "(?m)^\s*-\s*skipped or unavailable:.*\r?\n\s*-\s*reason unavailable:.*\r?\n\s*-\s*replacement narrower check:.*\r?\n\s*-\s*owner needed:.*\r?\n", ""
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $text
}

function Write-ModuleJReportMissingBusinessEvidenceHygiene {
    param([string]$Path)
    Write-ValidModuleJReport $Path
    $text = Get-Content -Raw -LiteralPath $Path
    $text = $text -replace "(?m)^\s*-\s*Business readiness folder:.*\r?\n", ""
    $text = $text -replace "(?m)^\s*-\s*Business readiness verification:.*\r?\n", ""
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $text
}

function Write-ModuleJReportMissingGoPolicySignals {
    param([string]$Path)
    Write-ValidModuleJReport $Path
    $text = Get-Content -Raw -LiteralPath $Path
    $text = $text -replace "(?m)^\s*-\s*failure paths:.*$", "- failure paths: rejection evidence captured without named go-policy cases"
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $text
}

function Write-ModuleJReportWithGoPolicyKeywordsOutsideFailurePaths {
    param([string]$Path)
    Write-ValidModuleJReport $Path
    $text = Get-Content -Raw -LiteralPath $Path
    $text = $text -replace "(?m)^\s*-\s*failure paths:.*$", "- failure paths: rejection evidence captured without named persona outcomes"
    $text = $text -replace "(?m)^\s*-\s*admin config bootstrap:.*$", "- admin config bootstrap: not purchased, expired, revoked device, and model denied keywords appear outside failure paths; admin config bootstrap passed"
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $text
}

function Write-ModuleJReportWithUnapprovedContractDrift {
    param([string]$Path)
    Write-ValidModuleJReport $Path
    $text = Get-Content -Raw -LiteralPath $Path
    $text = $text -replace "(?m)^\s*-\s*drift status:.*$", "- drift status: unapproved"
    $text = $text -replace "(?m)^\s*-\s*change review evidence:.*$", "- change review evidence: unreviewed route-field change"
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $text
}

function Write-ModuleJReportMissingModuleInputs {
    param([string]$Path)
    Write-ValidModuleJReport $Path
    $text = Get-Content -Raw -LiteralPath $Path
    $text = $text -replace "Module I final report recorded", "Module I input missing"
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $text
}

function Write-GoCandidateReadinessSummary {
    param([string]$Path)
    $e2eDir = Get-ModuleJFixtureValue "ModuleJFixtureE2EDir" "codex-plus-dev-plan/test-runs/20260618-1911-e2e"
    $packageDir = Get-ModuleJFixtureValue "ModuleJFixturePackageDir" "codex-plus-dev-plan/test-runs/20260618-1911-package"
    $compatibilityDir = Get-ModuleJFixtureValue "ModuleJFixtureCompatibilityDir" "codex-plus-dev-plan/test-runs/20260618-1911-compatibility"
    $docsDir = Get-ModuleJFixtureValue "ModuleJFixtureDocsDir" "codex-plus-dev-plan/test-runs/20260618-1911-docs"
    $businessDir = Get-ModuleJFixtureValue "ModuleJFixtureBusinessDir" "codex-plus-dev-plan/test-runs/20260618-1911-business"
    $coverageSummary = Get-ModuleJFixtureValue "ModuleJFixtureCoverageSummary" "codex-plus-dev-plan/test-runs/20260618-1911-release/release-coverage-summary.md"
    $content = @"
# 07 Release Readiness Summary

Report status: generated
Generated at: 2026-06-18 19:11:00+08:00
Aggregate evidence result: passed
Coverage summary verification: passed
Coverage status: complete
MVP package scope: cross-platform
Coverage missing count: 0
Coverage nonrelease marker count: 0
Docs product copy verification: passed
Business readiness verification: passed
Recommended Module J posture: go-candidate-requires-module-j-review
Allow go candidate: true

## Evidence Inputs

- E2E evidence folder: $e2eDir
- Package evidence folder: $packageDir
- Compatibility evidence folder: $compatibilityDir
- Docs product copy evidence folder: $docsDir
- Business readiness folder: $businessDir
- MVP package scope: cross-platform
- Release coverage summary: $coverageSummary
- Aggregate verifier log: codex-plus-dev-plan/test-runs/aggregate-release-evidence.log
- Business readiness verifier log: codex-plus-dev-plan/test-runs/business-readiness.log

## Evidence Signals

- E2E final recommendation: go with accepted risks
- E2E Level 3 result: pass
- Package evidence result: pass
- Compatibility evidence result: pass
- Docs product copy result: pass
- Coverage summary verification: passed
- Business readiness result: pass

## Nonrelease Markers

- none

## Release Boundary

This generated summary is not the final Module J report.
"@
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $content
}

function Write-GoCandidateCoverageSummary {
    param([string]$Path)
    $e2eDir = Get-ModuleJFixtureValue "ModuleJFixtureE2EDir" "codex-plus-dev-plan/test-runs/20260618-1911-e2e"
    $packageDir = Get-ModuleJFixtureValue "ModuleJFixturePackageDir" "codex-plus-dev-plan/test-runs/20260618-1911-package"
    $compatibilityDir = Get-ModuleJFixtureValue "ModuleJFixtureCompatibilityDir" "codex-plus-dev-plan/test-runs/20260618-1911-compatibility"
    $docsDir = Get-ModuleJFixtureValue "ModuleJFixtureDocsDir" "codex-plus-dev-plan/test-runs/20260618-1911-docs"
    $content = @"
# 07 Release Coverage Summary

Report status: generated
Generated at: 2026-06-18 19:11:00+08:00
Coverage status: complete
MVP package scope: cross-platform
Missing coverage count: 0
Nonrelease marker count: 0

## Evidence Inputs

- E2E evidence folder: $e2eDir
- Package evidence folder: $packageDir
- Compatibility evidence folder: $compatibilityDir
- Docs product copy evidence folder: $docsDir

## Coverage Matrix

| Lane | Requirement | Status | Evidence |
| --- | --- | --- | --- |
| e2e | production-equivalent happy path and failure paths | covered | $e2eDir |
| package | Windows and macOS package install paths | covered | $packageDir |
| compatibility | provider migration and rollback | covered | $compatibilityDir |
| docs | final docs and HTML product copy evidence | covered | $docsDir |

## Nonrelease Markers

- none

## Release Boundary

This coverage summary is sanitized go-candidate data for verifier self-tests.
"@
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $content
}

function Write-GoCandidateReadinessSummaryWithoutBusiness {
    param([string]$Path)
    $e2eDir = Get-ModuleJFixtureValue "ModuleJFixtureE2EDir" "codex-plus-dev-plan/test-runs/20260618-1911-e2e"
    $packageDir = Get-ModuleJFixtureValue "ModuleJFixturePackageDir" "codex-plus-dev-plan/test-runs/20260618-1911-package"
    $compatibilityDir = Get-ModuleJFixtureValue "ModuleJFixtureCompatibilityDir" "codex-plus-dev-plan/test-runs/20260618-1911-compatibility"
    $docsDir = Get-ModuleJFixtureValue "ModuleJFixtureDocsDir" "codex-plus-dev-plan/test-runs/20260618-1911-docs"
    $coverageSummary = Get-ModuleJFixtureValue "ModuleJFixtureCoverageSummary" "codex-plus-dev-plan/test-runs/20260618-1911-release/release-coverage-summary.md"
    $content = @"
# 07 Release Readiness Summary

Report status: generated
Generated at: 2026-06-18 19:11:00+08:00
Aggregate evidence result: passed
Coverage summary verification: passed
Coverage status: complete
Coverage missing count: 0
Coverage nonrelease marker count: 0
Docs product copy verification: passed
Recommended Module J posture: go-candidate-requires-module-j-review
Allow go candidate: true

## Evidence Inputs

- E2E evidence folder: $e2eDir
- Package evidence folder: $packageDir
- Compatibility evidence folder: $compatibilityDir
- Docs product copy evidence folder: $docsDir
- Release coverage summary: $coverageSummary
- Aggregate verifier log: codex-plus-dev-plan/test-runs/aggregate-release-evidence.log

## Evidence Signals

- E2E final recommendation: go with accepted risks
- E2E Level 3 result: pass
- Package evidence result: pass
- Compatibility evidence result: pass
- Docs product copy result: pass
- Coverage summary verification: passed

## Nonrelease Markers

- none

## Release Boundary

This generated summary is not the final Module J report.
"@
    Set-Content -LiteralPath $Path -Encoding UTF8 -Value $content
}

function Convert-FixtureEvidenceToReleaseCandidate {
    param([string[]]$Directories)
    foreach ($directory in $Directories) {
        Get-ChildItem -LiteralPath $directory -Recurse -File -Include *.md,*.txt,*.json,*.log |
            ForEach-Object {
                $text = Get-Content -Raw -LiteralPath $_.FullName
                $text = $text -replace "(?i)sanitized fixture", "sanitized execution evidence"
                $text = $text -replace "(?i)\bfixture\b", "execution evidence"
                $text = $text -replace "(?i)not release evidence", "release evidence"
                Set-Content -LiteralPath $_.FullName -Encoding UTF8 -Value $text
            }
    }
}

function Write-ValidReleaseHandoffIndex {
    param(
        [string]$ReleaseDir,
        [string]$RunStamp,
        [string]$E2EDir,
        [string]$PackageDir,
        [string]$CompatibilityDir,
        [string]$DocsDir,
        [string]$BusinessDir,
        [string]$CoverageSummaryFile,
        [string]$ReadinessSummaryFile,
        [string]$FinalReportFile
    )
    $content = @"
# 07 Release Evidence Set

Run stamp: $RunStamp
Status: final

This folder coordinates the final 07 release handoff verification package.

## Generated Evidence Folders

- E2E evidence: $E2EDir
- Package evidence: $PackageDir
- Compatibility evidence: $CompatibilityDir
- Docs product copy evidence: $DocsDir
- Business readiness evidence: $BusinessDir
- Release coverage summary: $CoverageSummaryFile
- Release readiness summary: $ReadinessSummaryFile
- Module J final report: $FinalReportFile

## Final Verification

- aggregate verifier: tools/verify-07-release-evidence.ps1
- coverage summary: tools/summarize-07-release-coverage.ps1
- readiness summary: tools/summarize-07-release-readiness.ps1
- report verifier: tools/verify-07-module-j-report.ps1
- business readiness: tools/verify-07-business-readiness.ps1
- handoff verifier: tools/verify-07-release-handoff.ps1

## Final Verification Results

- aggregate verifier result: passed
- docs product copy verification: passed
- business readiness verification: passed
- coverage summary status: complete
- readiness summary posture: go-candidate-requires-module-j-review
- Module J report verification: passed
- Final recommendation: go with accepted risks

## Release Boundary

- This index proves the handoff files are internally consistent.
- Module J still owns the final go/no-go recommendation.
"@
    Set-Content -LiteralPath (Join-Path $ReleaseDir "00-release-evidence-index.md") -Encoding UTF8 -Value $content
}

function Remove-ReleaseHandoffVerificationResults {
    param([string]$ReleaseDir)
    $indexPath = Join-Path $ReleaseDir "00-release-evidence-index.md"
    $text = Get-Content -Raw -LiteralPath $indexPath
    $text = $text -replace "(?ms)^## Final Verification Results\s+.*?(?=^##\s+Release Boundary)", ""
    Set-Content -LiteralPath $indexPath -Encoding UTF8 -Value $text
}

try {
    $businessSourceDocsRoot = Join-Path $OutputRoot "business-source-docs-clean"
    $businessUnresolvedSourceDocsRoot = Join-Path $OutputRoot "business-source-docs-unresolved"
    Write-BusinessSourceDocsFixture $businessSourceDocsRoot
    Write-BusinessSourceDocsFixture $businessUnresolvedSourceDocsRoot -Unresolved

    $envTemplateStamp = "20260618-1909"
    Invoke-Tool "generated-e2e-env-template" "new-07-e2e-env-template.ps1" @(
        "-Root", $Root,
        "-OutputRoot", $OutputRoot,
        "-Timestamp", $envTemplateStamp
    ) @(0)
    foreach ($expected in @(
        "$envTemplateStamp-e2e-env\e2e-env.template.ps1",
        "$envTemplateStamp-e2e-env\e2e-env-checklist.md"
    )) {
        $path = Join-Path $OutputRoot $expected
        Add-Result "generated-e2e-env-template-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    Invoke-Tool "local-admin-compliance-accept-opt-in-fails" "accept-07-local-admin-compliance.ps1" @(
        "-Root", $Root,
        "-AdminBaseUrl", "http://127.0.0.1:8081"
    ) @(1)
    $localComplianceDeniedEvidence = Join-Path $OutputRoot "20260618-1909-e2e"
    Invoke-Tool "local-admin-compliance-evidence-dir-still-requires-opt-in-fails" "accept-07-local-admin-compliance.ps1" @(
        "-Root", $Root,
        "-AdminBaseUrl", "http://127.0.0.1:8081",
        "-EvidenceDir", $localComplianceDeniedEvidence
    ) @(1)
    Add-Result "local-admin-compliance-denied-no-evidence-file" (-not (Test-Path -LiteralPath (Join-Path $localComplianceDeniedEvidence "03-admin-setup.md") -PathType Leaf)) "Denied compliance accept must not write 03-admin-setup.md."
    $localComplianceHelperText = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot "accept-07-local-admin-compliance.ps1")
    Add-Result "local-admin-compliance-helper-has-evidence-dir-param" ($localComplianceHelperText -match '(?m)^\s*\[string\]\$EvidenceDir\s*,?\s*$') "Helper must expose -EvidenceDir."
    Add-Result "local-admin-compliance-helper-has-output-path-param" ($localComplianceHelperText -match '(?m)^\s*\[string\]\$OutputPath\s*,?\s*$') "Helper must expose -OutputPath."
    Add-Result "local-admin-compliance-helper-evidence-run-folder" ($localComplianceHelperText -match 'Run folder: \$evidenceRunFolder') "Sanitized evidence must include the evidence run folder."
    Add-Result "local-admin-compliance-helper-redacts-token" ($localComplianceHelperText -match "Admin token value was intentionally not printed") "Sanitized evidence must state token values are not printed."
    Add-Result "local-admin-compliance-helper-redacts-raw-response" ($localComplianceHelperText -match "Raw API response body was not written to evidence") "Sanitized evidence must not write raw API responses."
    Add-Result "local-admin-compliance-helper-prefers-english-phrase" ($localComplianceHelperText -match 'ack_phrase_en' -and $localComplianceHelperText -match 'Language\s*=\s*"en"') "Helper should prefer the ASCII English acknowledgement phrase for local scripted acceptance."
    Add-Result "local-admin-compliance-helper-posts-selected-language" ($localComplianceHelperText -match 'language\s*=\s*\$language') "Helper must post the language that matches the selected phrase."

    $desktopHarnessStamp = "20260618-1910"
    Invoke-Tool "desktop-compatibility-harness-generates" "new-07-desktop-compatibility-harness.ps1" @(
        "-Root", $Root,
        "-OutputRoot", $OutputRoot,
        "-Timestamp", $desktopHarnessStamp,
        "-Force"
    ) @(0)
    $desktopHarnessRoot = Join-Path $OutputRoot "$desktopHarnessStamp-desktop-harness"
    foreach ($expected in @(
        "isolated-desktop-env.ps1",
        "capture-snapshot.ps1",
        "launch-manager-isolated.ps1",
        "README.md",
        "isolated-userprofile\.codex\config.toml",
        "isolated-userprofile\.codex-session-delete\settings.json"
    )) {
        $path = Join-Path $desktopHarnessRoot $expected
        Add-Result "desktop-harness-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $desktopSnapshotPath = Join-Path $desktopHarnessRoot "snapshots\pre-upgrade.json"
    Invoke-Tool "desktop-provider-snapshot-capture-generates" "capture-07-desktop-provider-snapshot.ps1" @(
        "-Root", $Root,
        "-ProfileRoot", (Join-Path $desktopHarnessRoot "isolated-userprofile"),
        "-Label", "pre-upgrade",
        "-OutputPath", $desktopSnapshotPath,
        "-Force"
    ) @(0)
    Add-Result "desktop-provider-snapshot-file" (Test-Path -LiteralPath $desktopSnapshotPath -PathType Leaf) $desktopSnapshotPath
    if (Test-Path -LiteralPath $desktopSnapshotPath -PathType Leaf) {
        $desktopSnapshotText = Get-Content -Raw -LiteralPath $desktopSnapshotPath
        Add-Result "desktop-provider-snapshot-hash-only" (
            $desktopSnapshotText -match "base_url_hash" -and
            $desktopSnapshotText -match "api_key_hash" -and
            $desktopSnapshotText -notmatch "manual-provider\.invalid" -and
            $desktopSnapshotText -notmatch "redacted-manual-provider-key-for-hash-only"
        ) "Provider snapshot must contain hashes but not raw seeded URL/key values."
    }

    Invoke-Tool "e2e-readiness-missing-env-fails" "verify-07-e2e-readiness.ps1" @(
        "-Root", $Root,
        "-EnvPrefix", "CODEXPLUS_07_SELFTEST_MISSING_"
    ) @(1)

    $readinessManagerPath = Join-Path $OutputRoot "manager-build-fixture"
    New-Item -ItemType Directory -Force -Path $readinessManagerPath | Out-Null
    New-Item -ItemType File -Force -Path (Join-Path $readinessManagerPath "codex-plus-plus-manager.exe") | Out-Null
    $readinessValues = @{
        "CODEXPLUS_07_SELFTEST_BACKEND_BASE_URL" = "http://127.0.0.1:18080"
        "CODEXPLUS_07_SELFTEST_ADMIN_BASE_URL" = "https://staging.codexplus.test/admin"
        "CODEXPLUS_07_SELFTEST_MANAGER_BUILD_PATH" = $readinessManagerPath
        "CODEXPLUS_07_SELFTEST_ADMIN_TOKEN" = "redacted-admin-token-fixture"
        "CODEXPLUS_07_SELFTEST_USER_ACTIVE_TOKEN" = "redacted-active-token-fixture"
        "CODEXPLUS_07_SELFTEST_USER_NOT_PURCHASED_TOKEN" = "redacted-not-purchased-token-fixture"
        "CODEXPLUS_07_SELFTEST_USER_EXPIRED_TOKEN" = "redacted-expired-token-fixture"
        "CODEXPLUS_07_SELFTEST_USER_LOW_BALANCE_TOKEN" = "redacted-low-balance-token-fixture"
        "CODEXPLUS_07_SELFTEST_USER_DEVICE_REVOKED_TOKEN" = "redacted-revoked-token-fixture"
        "CODEXPLUS_07_SELFTEST_USER_MODEL_DENIED_TOKEN" = "redacted-model-denied-token-fixture"
        "CODEXPLUS_07_SELFTEST_USER_ACTIVE_ID" = "1001"
        "CODEXPLUS_07_SELFTEST_USER_NOT_PURCHASED_ID" = "1002"
        "CODEXPLUS_07_SELFTEST_USER_EXPIRED_ID" = "1003"
        "CODEXPLUS_07_SELFTEST_USER_LOW_BALANCE_ID" = "1004"
        "CODEXPLUS_07_SELFTEST_USER_DEVICE_REVOKED_ID" = "1005"
        "CODEXPLUS_07_SELFTEST_USER_MODEL_DENIED_ID" = "1006"
        "CODEXPLUS_07_SELFTEST_TEST_DEVICE_ID" = "device-fixture-001"
        "CODEXPLUS_07_SELFTEST_ALLOWED_TEST_MODEL" = "allowed-model-fixture"
        "CODEXPLUS_07_SELFTEST_DENIED_TEST_MODEL" = "denied-model-fixture"
    }
    $envFilePath = Join-Path $OutputRoot "readiness-env-file-fixture.ps1"
    $envFileLines = New-Object System.Collections.Generic.List[string]
    foreach ($name in $readinessValues.Keys) {
        $envFileName = $name -replace "CODEXPLUS_07_SELFTEST_", "CODEXPLUS_07_ENVFILE_"
        $envFileValue = $readinessValues[$name] -replace "'", "''"
        $envFileLines.Add("`$env:$envFileName = '$envFileValue'")
    }
    Set-Content -LiteralPath $envFilePath -Encoding UTF8 -Value ($envFileLines -join [Environment]::NewLine)
    Invoke-Tool "e2e-readiness-env-file-passes" "verify-07-e2e-readiness.ps1" @(
        "-Root", $Root,
        "-EnvPrefix", "CODEXPLUS_07_ENVFILE_",
        "-EnvFile", $envFilePath
    ) @(0)

    $previousReadinessValues = @{}
    foreach ($name in $readinessValues.Keys) {
        $previousReadinessValues[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
        [Environment]::SetEnvironmentVariable($name, $readinessValues[$name], "Process")
    }
    try {
        Invoke-Tool "e2e-readiness-fixture-passes" "verify-07-e2e-readiness.ps1" @(
            "-Root", $Root,
            "-EnvPrefix", "CODEXPLUS_07_SELFTEST_"
        ) @(0)
    } finally {
        foreach ($name in $previousReadinessValues.Keys) {
            [Environment]::SetEnvironmentVariable($name, $previousReadinessValues[$name], "Process")
        }
    }

    $clientApiMissingEnvPath = Join-Path $OutputRoot "20260618-1919-e2e"
    Invoke-Tool "client-api-runner-missing-env-fails" "..\..\sub2api-main\tools\e2e\codexplus\run-client-api-checks.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $clientApiMissingEnvPath,
        "-EnvPrefix", "CODEXPLUS_07_SELFTEST_MISSING_"
    ) @(1)
    foreach ($expected in @(
        "02-contract-checks.md",
        "04-client-api-e2e.md",
        "09-usage-events-audit.md",
        "11-defects.md"
    )) {
        $path = Join-Path $clientApiMissingEnvPath $expected
        Add-Result "client-api-missing-env-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $clientApiFixturePath = Join-Path $OutputRoot "20260618-1920-e2e"
    Invoke-Tool "client-api-runner-fixture-passes" "..\..\sub2api-main\tools\e2e\codexplus\run-client-api-checks.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $clientApiFixturePath,
        "-FixtureMode"
    ) @(0)
    foreach ($expected in @(
        "02-contract-checks.md",
        "04-client-api-e2e.md",
        "09-usage-events-audit.md",
        "11-defects.md"
    )) {
        $path = Join-Path $clientApiFixturePath $expected
        Add-Result "client-api-fixture-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $browserHandoffDeniedPath = Join-Path $OutputRoot "20260618-1920-browser-handoff-denied"
    Invoke-Tool "browser-handoff-runner-session-start-opt-in-fails" "..\..\sub2api-main\tools\e2e\codexplus\run-browser-handoff-checks.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $browserHandoffDeniedPath
    ) @(1)
    foreach ($expected in @(
        "02-contract-checks.md",
        "04-client-api-e2e.md",
        "11-defects.md"
    )) {
        $path = Join-Path $browserHandoffDeniedPath $expected
        Add-Result "browser-handoff-denied-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $browserHandoffFixturePath = Join-Path $OutputRoot "20260618-1920-browser-handoff-fixture"
    Invoke-Tool "browser-handoff-runner-fixture-passes" "..\..\sub2api-main\tools\e2e\codexplus\run-browser-handoff-checks.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $browserHandoffFixturePath,
        "-FixtureMode"
    ) @(0)
    foreach ($expected in @(
        "02-contract-checks.md",
        "04-client-api-e2e.md",
        "11-defects.md"
    )) {
        $path = Join-Path $browserHandoffFixturePath $expected
        Add-Result "browser-handoff-fixture-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $gatewayDeniedPath = Join-Path $OutputRoot "20260618-1920-gateway-denied"
    Invoke-Tool "gateway-runner-opt-in-fails" "..\..\sub2api-main\tools\e2e\codexplus\run-gateway-policy-checks.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $gatewayDeniedPath
    ) @(1)
    foreach ($expected in @(
        "05-gateway-policy-e2e.md",
        "09-usage-events-audit.md",
        "11-defects.md"
    )) {
        $path = Join-Path $gatewayDeniedPath $expected
        Add-Result "gateway-denied-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $gatewayFixturePath = Join-Path $OutputRoot "20260618-1920-gateway-fixture"
    Invoke-Tool "gateway-runner-fixture-passes" "..\..\sub2api-main\tools\e2e\codexplus\run-gateway-policy-checks.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $gatewayFixturePath,
        "-FixtureMode"
    ) @(0)
    foreach ($expected in @(
        "05-gateway-policy-e2e.md",
        "09-usage-events-audit.md",
        "11-defects.md"
    )) {
        $path = Join-Path $gatewayFixturePath $expected
        Add-Result "gateway-fixture-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $adminAuditDeniedPath = Join-Path $OutputRoot "20260618-1920-admin-audit-denied"
    Invoke-Tool "admin-audit-runner-read-opt-in-fails" "..\..\sub2api-main\tools\e2e\codexplus\run-admin-audit-checks.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $adminAuditDeniedPath
    ) @(1)
    foreach ($expected in @(
        "09-usage-events-audit.md"
    )) {
        $path = Join-Path $adminAuditDeniedPath $expected
        Add-Result "admin-audit-denied-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $adminAuditFixturePath = Join-Path $OutputRoot "20260618-1920-admin-audit-fixture"
    Invoke-Tool "admin-audit-runner-fixture-passes" "..\..\sub2api-main\tools\e2e\codexplus\run-admin-audit-checks.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $adminAuditFixturePath,
        "-FixtureMode"
    ) @(0)
    foreach ($expected in @(
        "09-usage-events-audit.md"
    )) {
        $path = Join-Path $adminAuditFixturePath $expected
        Add-Result "admin-audit-fixture-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }
    $adminAuditFixtureEvidence = Join-Path $adminAuditFixturePath "09-usage-events-audit.md"
    if (Test-Path -LiteralPath $adminAuditFixtureEvidence -PathType Leaf) {
        $adminAuditFixtureText = Get-Content -Raw -LiteralPath $adminAuditFixtureEvidence
        Add-Result "admin-audit-fixture-request-id-correlation" ($adminAuditFixtureText -match "(?is)Gateway request[_ ]?id correlation\s*:\s*pass.*Request ID correlation.*matched") "Admin audit fixture must prove matched gateway request_id correlation."
    }

    Invoke-Tool "local-e2e-runner-fixture-passes" "..\..\sub2api-main\tools\e2e\codexplus\run-local-e2e.ps1" @(
        "-Root", $Root,
        "-OutputRoot", $OutputRoot,
        "-Timestamp", "20260618-1921",
        "-FixtureMode",
        "-RunBrowserHandoff",
        "-RunGatewayPolicy",
        "-RunAdminAudit"
    ) @(0)
    foreach ($expected in @(
        "20260618-1921-e2e\00-environment.md",
        "20260618-1921-e2e\04-client-api-e2e.md",
        "20260618-1921-e2e\05-gateway-policy-e2e.md",
        "20260618-1921-e2e\09-usage-events-audit.md",
        "20260618-1921-e2e\12-release-gate-report.md"
    )) {
        $path = Join-Path $OutputRoot $expected
        Add-Result "local-e2e-fixture-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }
    $localAdminAuditEvidence = Join-Path $OutputRoot "20260618-1921-e2e\09-usage-events-audit.md"
    if (Test-Path -LiteralPath $localAdminAuditEvidence -PathType Leaf) {
        $localAdminAuditText = Get-Content -Raw -LiteralPath $localAdminAuditEvidence
        Add-Result "local-e2e-fixture-request-id-correlation" ($localAdminAuditText -match "(?is)Gateway request[_ ]?id correlation\s*:\s*pass.*fixture-user-active-request.*matched") "Local fixture must correlate admin audit rows back to gateway request IDs."
    }

    $releaseGapOutputPath = Join-Path $OutputRoot "20260618-1921-release-gap-helper.md"
    Invoke-Tool "release-gap-helper-open-gaps-reports" "report-07-release-gaps.ps1" @(
        "-Root", $Root,
        "-OutputRoot", $OutputRoot,
        "-RunStamp", "20260618-1921",
        "-OutputFile", $releaseGapOutputPath
    ) @(0)
    $releaseGapText = Get-Content -Raw -LiteralPath $releaseGapOutputPath
    Add-Result "release-gap-helper-open-status" ($releaseGapText -match "(?im)^\s*Gap status:\s*open\s*$") "Gap helper should report open gaps for missing sibling lanes."
    Add-Result "release-gap-helper-package-missing" ($releaseGapText -match "package.*missing") "Gap helper should list missing package lane evidence."

    $packageArtifactMissingPath = Join-Path $OutputRoot "20260618-1922-package"
    Invoke-Tool "package-artifact-inspection-missing-artifacts-fails" "inspect-07-package-artifacts.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $packageArtifactMissingPath,
        "-ArtifactDir", (Join-Path $OutputRoot "missing-package-artifacts")
    ) @(1)
    foreach ($expected in @(
        "00-artifact-metadata.md",
        "04-artifact-inspection.md"
    )) {
        $path = Join-Path $packageArtifactMissingPath $expected
        Add-Result "package-artifact-missing-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $packageArtifactFixturePath = Join-Path $OutputRoot "20260618-1923-package"
    Invoke-Tool "package-artifact-inspection-fixture-passes" "inspect-07-package-artifacts.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $packageArtifactFixturePath,
        "-FixtureMode"
    ) @(0)
    foreach ($expected in @(
        "00-artifact-metadata.md",
        "04-artifact-inspection.md"
    )) {
        $path = Join-Path $packageArtifactFixturePath $expected
        Add-Result "package-artifact-fixture-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }
    $packageArtifactInspectionText = Get-Content -Raw -LiteralPath (Join-Path $packageArtifactFixturePath "04-artifact-inspection.md")
    Add-Result "package-artifact-fixture-result-pass" ($packageArtifactInspectionText -match "(?im)^\s*Result\s*:\s*pass\s*$") "04-artifact-inspection.md result should pass for sanitized fixture artifacts."

    $packageArtifactTokenDir = Join-Path $OutputRoot "malicious-package-token-artifacts"
    $packageArtifactTokenPath = Join-Path $OutputRoot "20260618-1932-package"
    Write-PackageArtifactFixtureSet $packageArtifactTokenDir -Mode token
    Invoke-Tool "package-artifact-inspection-embedded-fake-sk-token-fails" "inspect-07-package-artifacts.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $packageArtifactTokenPath,
        "-ArtifactDir", $packageArtifactTokenDir
    ) @(1)

    $packageArtifactPolicyDir = Join-Path $OutputRoot "malicious-package-policy-artifacts"
    $packageArtifactPolicyPath = Join-Path $OutputRoot "20260618-1933-package"
    Write-PackageArtifactFixtureSet $packageArtifactPolicyDir -Mode policy
    Invoke-Tool "package-artifact-inspection-fixed-commercial-policy-fails" "inspect-07-package-artifacts.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $packageArtifactPolicyPath,
        "-ArtifactDir", $packageArtifactPolicyDir
    ) @(1)

    $compatibilitySnapshotMissingPath = Join-Path $OutputRoot "20260618-1924-compatibility"
    Invoke-Tool "compatibility-snapshot-inspection-missing-snapshots-fails" "inspect-07-compatibility-snapshots.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $compatibilitySnapshotMissingPath
    ) @(1)
    foreach ($expected in @(
        "00-test-context.md",
        "01-pre-upgrade-snapshot.md",
        "02-post-upgrade-cloud.md",
        "03-cloud-logout-boundary.md",
        "04-manual-provider-switch.md",
        "05-provider-sync.md",
        "06-rollback-rehearsal.md",
        "07-compatibility-gate-report.md"
    )) {
        $path = Join-Path $compatibilitySnapshotMissingPath $expected
        Add-Result "compatibility-snapshot-missing-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    $compatibilitySnapshotFixturePath = Join-Path $OutputRoot "20260618-1925-compatibility"
    Invoke-Tool "compatibility-snapshot-inspection-fixture-passes" "inspect-07-compatibility-snapshots.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $compatibilitySnapshotFixturePath,
        "-FixtureMode"
    ) @(0)
    Invoke-Tool "compatibility-snapshot-only-evidence-verifier-fails" "verify-07-compatibility-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $compatibilitySnapshotFixturePath
    ) @(1)
    $compatibilitySnapshotGateText = Get-Content -Raw -LiteralPath (Join-Path $compatibilitySnapshotFixturePath "07-compatibility-gate-report.md")
    Add-Result "compatibility-snapshot-fixture-subset-pass" ($compatibilitySnapshotGateText -match "(?im)^\s*Compatibility snapshot subset result\s*:\s*pass\s*$") "07-compatibility-gate-report.md snapshot subset result should pass for sanitized fixture snapshots."
    Add-Result "compatibility-snapshot-fixture-final-fail" ($compatibilitySnapshotGateText -match "(?im)^\s*Compatibility evidence result\s*:\s*fail\s*$") "07-compatibility-gate-report.md final result should stay fail until runtime compatibility evidence is added."

    $compatibilitySnapshotTokenDir = Join-Path $OutputRoot "malicious-compat-token-snapshots"
    $compatibilitySnapshotTokenPath = Join-Path $OutputRoot "20260618-1934-compatibility"
    Write-CompatibilitySnapshotFixtureSet $compatibilitySnapshotTokenDir -Mode token
    Invoke-Tool "compatibility-snapshot-inspection-token-fields-fail" "inspect-07-compatibility-snapshots.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $compatibilitySnapshotTokenPath,
        "-PreUpgradeSnapshot", (Join-Path $compatibilitySnapshotTokenDir "pre-upgrade.json"),
        "-PostUpgradeSnapshot", (Join-Path $compatibilitySnapshotTokenDir "post-upgrade.json"),
        "-LogoutSnapshot", (Join-Path $compatibilitySnapshotTokenDir "logout.json"),
        "-RollbackSnapshot", (Join-Path $compatibilitySnapshotTokenDir "rollback.json")
    ) @(1)

    $compatibilitySnapshotCamelTokenDir = Join-Path $OutputRoot "malicious-compat-camel-token-snapshots"
    $compatibilitySnapshotCamelTokenPath = Join-Path $OutputRoot "20260618-1936-compatibility"
    Write-CompatibilitySnapshotFixtureSet $compatibilitySnapshotCamelTokenDir -Mode camelToken
    Invoke-Tool "compatibility-snapshot-inspection-camel-token-fields-fail" "inspect-07-compatibility-snapshots.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $compatibilitySnapshotCamelTokenPath,
        "-PreUpgradeSnapshot", (Join-Path $compatibilitySnapshotCamelTokenDir "pre-upgrade.json"),
        "-PostUpgradeSnapshot", (Join-Path $compatibilitySnapshotCamelTokenDir "post-upgrade.json"),
        "-LogoutSnapshot", (Join-Path $compatibilitySnapshotCamelTokenDir "logout.json"),
        "-RollbackSnapshot", (Join-Path $compatibilitySnapshotCamelTokenDir "rollback.json")
    ) @(1)

    $compatibilitySnapshotPolicyDir = Join-Path $OutputRoot "malicious-compat-policy-snapshots"
    $compatibilitySnapshotPolicyPath = Join-Path $OutputRoot "20260618-1935-compatibility"
    Write-CompatibilitySnapshotFixtureSet $compatibilitySnapshotPolicyDir -Mode policy
    Invoke-Tool "compatibility-snapshot-inspection-commercial-policy-fields-fail" "inspect-07-compatibility-snapshots.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $compatibilitySnapshotPolicyPath,
        "-PreUpgradeSnapshot", (Join-Path $compatibilitySnapshotPolicyDir "pre-upgrade.json"),
        "-PostUpgradeSnapshot", (Join-Path $compatibilitySnapshotPolicyDir "post-upgrade.json"),
        "-LogoutSnapshot", (Join-Path $compatibilitySnapshotPolicyDir "logout.json"),
        "-RollbackSnapshot", (Join-Path $compatibilitySnapshotPolicyDir "rollback.json")
    ) @(1)

    $compatibilitySnapshotChangedDir = Join-Path $OutputRoot "malicious-compat-changed-provider-snapshots"
    $compatibilitySnapshotChangedPath = Join-Path $OutputRoot "20260618-1937-compatibility"
    Write-CompatibilitySnapshotFixtureSet $compatibilitySnapshotChangedDir -Mode changed
    Invoke-Tool "compatibility-snapshot-inspection-manual-provider-content-change-fails" "inspect-07-compatibility-snapshots.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $compatibilitySnapshotChangedPath,
        "-PreUpgradeSnapshot", (Join-Path $compatibilitySnapshotChangedDir "pre-upgrade.json"),
        "-PostUpgradeSnapshot", (Join-Path $compatibilitySnapshotChangedDir "post-upgrade.json"),
        "-LogoutSnapshot", (Join-Path $compatibilitySnapshotChangedDir "logout.json"),
        "-RollbackSnapshot", (Join-Path $compatibilitySnapshotChangedDir "rollback.json")
    ) @(1)

    $generatedStamp = "20260618-1910"
    Invoke-Tool "generated-release-set" "new-07-release-evidence-set.ps1" @("-Root", $Root, "-OutputRoot", $OutputRoot, "-Timestamp", $generatedStamp) @(0)

    foreach ($expected in @(
        "$generatedStamp-e2e\12-release-gate-report.md",
        "$generatedStamp-package\05-package-gate-report.md",
        "$generatedStamp-compatibility\07-compatibility-gate-report.md",
        "$generatedStamp-docs\06-docs-product-copy-gate-report.md",
        "$generatedStamp-business\11-business-readiness.md",
        "$generatedStamp-release\00-release-evidence-index.md",
        "$generatedStamp-release\module-j-final-report-draft.md"
    )) {
        $path = Join-Path $OutputRoot $expected
        Add-Result "generated-file:$expected" (Test-Path -LiteralPath $path -PathType Leaf) $path
    }

    Invoke-Tool "generated-aggregate-fails" "verify-07-release-evidence.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-e2e"),
        "-PackageEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-package"),
        "-CompatibilityEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-compatibility"),
        "-DocsEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-docs"),
        "-LogRoot", (Join-Path $OutputRoot "generated-aggregate-logs")
    ) @(1)
    Invoke-Tool "generated-docs-product-copy-fails" "verify-07-docs-product-copy-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", (Join-Path $OutputRoot "$generatedStamp-docs")
    ) @(1)
    Invoke-Tool "generated-business-readiness-fails" "verify-07-business-readiness.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", (Join-Path $OutputRoot "$generatedStamp-business")
    ) @(1)
    $generatedCoverageSummary = Join-Path $OutputRoot "$generatedStamp-release\release-coverage-summary.md"
    Invoke-Tool "generated-coverage-summary-fails" "summarize-07-release-coverage.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-e2e"),
        "-PackageEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-package"),
        "-CompatibilityEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-compatibility"),
        "-DocsEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-docs"),
        "-OutputFile", $generatedCoverageSummary,
        "-FailOnIncomplete"
    ) @(1)
    Add-Result "generated-coverage-summary-file" (Test-Path -LiteralPath $generatedCoverageSummary -PathType Leaf) $generatedCoverageSummary
    if (Test-Path -LiteralPath $generatedCoverageSummary -PathType Leaf) {
        $generatedCoverageText = Get-Content -Raw -LiteralPath $generatedCoverageSummary
        Add-Result "generated-coverage-summary-incomplete" ($generatedCoverageText -match "(?im)^\s*Coverage status\s*:\s*incomplete\s*$") "Generated scaffold coverage must remain incomplete."
    }
    $generatedReadinessSummary = Join-Path $OutputRoot "$generatedStamp-release\release-readiness-summary.md"
    Invoke-Tool "generated-readiness-summary-fails" "summarize-07-release-readiness.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-e2e"),
        "-PackageEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-package"),
        "-CompatibilityEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-compatibility"),
        "-DocsEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-docs"),
        "-BusinessEvidenceDir", (Join-Path $OutputRoot "$generatedStamp-business"),
        "-CoverageSummaryFile", $generatedCoverageSummary,
        "-OutputFile", $generatedReadinessSummary,
        "-FailOnNoGo"
    ) @(1)
    Add-Result "generated-readiness-summary-file" (Test-Path -LiteralPath $generatedReadinessSummary -PathType Leaf) $generatedReadinessSummary
    if (Test-Path -LiteralPath $generatedReadinessSummary -PathType Leaf) {
        $generatedReadinessText = Get-Content -Raw -LiteralPath $generatedReadinessSummary
        Add-Result "generated-readiness-summary-no-go" ($generatedReadinessText -match "(?im)^\s*Recommended Module J posture\s*:\s*no-go\s*$") "Generated scaffold summary must remain no-go."
        Add-Result "generated-readiness-summary-business-failed" ($generatedReadinessText -match "(?im)^\s*Business readiness verification\s*:\s*failed\s*$") "Generated scaffold summary must record failed business readiness."
    }

    Invoke-Tool "generated-module-j-draft-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", (Join-Path $OutputRoot "$generatedStamp-release\module-j-final-report-draft.md")
    ) @(1)
    Invoke-Tool "generated-release-handoff-fails" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", (Join-Path $OutputRoot "$generatedStamp-release")
    ) @(1)

    $validStamp = "20260618-1911"
    $validE2E = Join-Path $OutputRoot "$validStamp-e2e"
    $validPackage = Join-Path $OutputRoot "$validStamp-package"
    $validCompatibility = Join-Path $OutputRoot "$validStamp-compatibility"
    $validDocs = Join-Path $OutputRoot "$validStamp-docs"
    $validBusiness = Join-Path $OutputRoot "$validStamp-business"
    $validReport = Join-Path $OutputRoot "module-j-final-report.md"
    $goCandidateCoverageSummary = Join-Path $OutputRoot "go-candidate-release-coverage-summary.md"
    $goCandidateReadinessSummary = Join-Path $OutputRoot "go-candidate-release-readiness-summary.md"
    $script:ModuleJFixtureE2EDir = $validE2E
    $script:ModuleJFixturePackageDir = $validPackage
    $script:ModuleJFixtureCompatibilityDir = $validCompatibility
    $script:ModuleJFixtureDocsDir = $validDocs
    $script:ModuleJFixtureBusinessDir = $validBusiness
    $script:ModuleJFixtureCoverageSummary = $goCandidateCoverageSummary
    $script:ModuleJFixtureReadinessSummary = $goCandidateReadinessSummary
    Write-ValidE2EFixture $validE2E
    Write-ValidPackageFixture $validPackage
    Write-ValidCompatibilityFixture $validCompatibility
    Write-ValidDocsProductCopyFixture $validDocs
    Write-ValidBusinessReadinessFixture $validBusiness
    Write-ValidModuleJReport $validReport
    Invoke-Tool "valid-business-readiness-passes" "verify-07-business-readiness.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $validBusiness,
        "-SourceDocsRoot", $businessSourceDocsRoot
    ) @(0)

    $failedE2E = Join-Path $OutputRoot "20260618-1922-e2e"
    Write-ValidE2EFixture $failedE2E
    $failedE2EReport = Join-Path $failedE2E "04-client-api-e2e.md"
    $failedE2EText = Get-Content -Raw -LiteralPath $failedE2EReport
    $failedE2EText = $failedE2EText -replace "(?m)^\s*Result:\s*pass\s*$", "Result: fail"
    Set-Content -LiteralPath $failedE2EReport -Encoding UTF8 -Value $failedE2EText
    Invoke-Tool "e2e-result-fail-fails" "verify-07-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedE2E
    ) @(1)

    $missingLevel3E2E = Join-Path $OutputRoot "20260618-1928-e2e"
    Write-ValidE2EFixture $missingLevel3E2E
    $missingLevel3E2EReport = Join-Path $missingLevel3E2E "12-release-gate-report.md"
    $missingLevel3E2EText = Get-Content -Raw -LiteralPath $missingLevel3E2EReport
    $missingLevel3E2EText = $missingLevel3E2EText -replace "(?m)^\s*Level 3 result:\s*pass\s*\r?\n", ""
    Set-Content -LiteralPath $missingLevel3E2EReport -Encoding UTF8 -Value $missingLevel3E2EText
    Invoke-Tool "e2e-missing-level3-pass-fails" "verify-07-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $missingLevel3E2E
    ) @(1)

    $missingAuditCorrelationE2E = Join-Path $OutputRoot "20260618-1932-e2e"
    Write-ValidE2EFixture $missingAuditCorrelationE2E
    $missingAuditCorrelationPath = Join-Path $missingAuditCorrelationE2E "09-usage-events-audit.md"
    $missingAuditCorrelationText = Get-Content -Raw -LiteralPath $missingAuditCorrelationPath
    $missingAuditCorrelationText = $missingAuditCorrelationText -replace "(?m)^Gateway request_id correlation: pass\s*\r?\n", ""
    Set-Content -LiteralPath $missingAuditCorrelationPath -Encoding UTF8 -Value $missingAuditCorrelationText
    Invoke-Tool "e2e-usage-audit-missing-request-id-correlation-fails" "verify-07-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $missingAuditCorrelationE2E
    ) @(1)

    $staleRunFolderE2E = Join-Path $OutputRoot "20260618-1939-e2e"
    Write-ValidE2EFixture $staleRunFolderE2E
    Invoke-Tool "e2e-run-folder-mismatch-fails" "verify-07-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $staleRunFolderE2E
    ) @(1)

    $failedPackage = Join-Path $OutputRoot "20260618-1923-package"
    Write-ValidPackageFixture $failedPackage
    $failedPackageReport = Join-Path $failedPackage "01-windows-x64-install.md"
    $failedPackageText = Get-Content -Raw -LiteralPath $failedPackageReport
    $failedPackageText = $failedPackageText -replace "(?m)^\s*Result:\s*pass\s*$", "Result: fail"
    Set-Content -LiteralPath $failedPackageReport -Encoding UTF8 -Value $failedPackageText
    Invoke-Tool "package-result-fail-fails" "verify-07-package-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedPackage
    ) @(1)

    $failedPackagePlatformEvidence = Join-Path $OutputRoot "20260618-1933-package"
    Write-ValidPackageFixture $failedPackagePlatformEvidence
    $failedPackagePlatformPath = Join-Path $failedPackagePlatformEvidence "01-windows-x64-install.md"
    $failedPackagePlatformText = Get-Content -Raw -LiteralPath $failedPackagePlatformPath
    $failedPackagePlatformText = $failedPackagePlatformText -replace "(?m)^Manager opened login, install assistant, diagnostics, and advanced configuration\.\s*\r?\n", ""
    $failedPackagePlatformText = $failedPackagePlatformText -replace "(?m)^Missing-Codex first-run assistant was confirmed\.\s*\r?\n", ""
    $failedPackagePlatformText = $failedPackagePlatformText -replace "(?m)^\s*-\s*Manager login result:\s*pass\s*\r?\n", ""
    $failedPackagePlatformText = $failedPackagePlatformText -replace "(?m)^\s*-\s*Manager install assistant result:\s*pass\s*\r?\n", ""
    $failedPackagePlatformText = $failedPackagePlatformText -replace "(?m)^\s*-\s*Manager diagnostics result:\s*pass\s*\r?\n", ""
    $failedPackagePlatformText = $failedPackagePlatformText -replace "(?m)^\s*-\s*Manager advanced configuration result:\s*pass\s*\r?\n", ""
    $failedPackagePlatformText = $failedPackagePlatformText -replace "(?m)^\s*-\s*Missing-Codex first-run assistant behavior result:\s*pass\s*\r?\n", ""
    Set-Content -LiteralPath $failedPackagePlatformPath -Encoding UTF8 -Value $failedPackagePlatformText
    Invoke-Tool "package-platform-manager-missing-codex-evidence-fails" "verify-07-package-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedPackagePlatformEvidence
    ) @(1)

    $failedPackageMetadata = Join-Path $OutputRoot "20260618-1926-package"
    Write-ValidPackageFixture $failedPackageMetadata
    $failedPackageMetadataPath = Join-Path $failedPackageMetadata "00-artifact-metadata.md"
    $failedPackageMetadataText = Get-Content -Raw -LiteralPath $failedPackageMetadataPath
    $failedPackageMetadataText = $failedPackageMetadataText -replace "(?m)^\s*Result:\s*pass\s*$", "Result: fail"
    Set-Content -LiteralPath $failedPackageMetadataPath -Encoding UTF8 -Value $failedPackageMetadataText
    Invoke-Tool "package-metadata-result-fail-fails" "verify-07-package-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedPackageMetadata
    ) @(1)

    $failedPackageCoverage = Join-Path $OutputRoot "20260618-1927-package"
    Write-ValidPackageFixture $failedPackageCoverage
    $failedPackageCoveragePath = Join-Path $failedPackageCoverage "04-artifact-inspection.md"
    $failedPackageCoverageText = Get-Content -Raw -LiteralPath $failedPackageCoveragePath
    $failedPackageCoverageText = $failedPackageCoverageText -replace "(?m)^\s*-\s*windows-x64-setup:\s*present\s*$", "- windows-x64-setup: missing"
    Set-Content -LiteralPath $failedPackageCoveragePath -Encoding UTF8 -Value $failedPackageCoverageText
    Invoke-Tool "package-artifact-coverage-missing-fails" "verify-07-package-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedPackageCoverage
    ) @(1)

    $failedCompatibility = Join-Path $OutputRoot "20260618-1924-compatibility"
    Write-ValidCompatibilityFixture $failedCompatibility
    $failedCompatibilityReport = Join-Path $failedCompatibility "02-post-upgrade-cloud.md"
    $failedCompatibilityText = Get-Content -Raw -LiteralPath $failedCompatibilityReport
    $failedCompatibilityText = $failedCompatibilityText -replace "(?m)^\s*Result:\s*pass\s*$", "Result: fail"
    Set-Content -LiteralPath $failedCompatibilityReport -Encoding UTF8 -Value $failedCompatibilityText
    Invoke-Tool "compatibility-result-fail-fails" "verify-07-compatibility-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedCompatibility
    ) @(1)

    $failedCompatibilityContext = Join-Path $OutputRoot "20260618-1926-compatibility"
    Write-ValidCompatibilityFixture $failedCompatibilityContext
    $failedCompatibilityContextPath = Join-Path $failedCompatibilityContext "00-test-context.md"
    $failedCompatibilityContextText = Get-Content -Raw -LiteralPath $failedCompatibilityContextPath
    $failedCompatibilityContextText = $failedCompatibilityContextText -replace "(?m)^\s*Result:\s*pass\s*$", "Result: fail"
    Set-Content -LiteralPath $failedCompatibilityContextPath -Encoding UTF8 -Value $failedCompatibilityContextText
    Invoke-Tool "compatibility-context-result-fail-fails" "verify-07-compatibility-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedCompatibilityContext
    ) @(1)

    $failedCompatibilityManual = Join-Path $OutputRoot "20260618-1927-compatibility"
    Write-ValidCompatibilityFixture $failedCompatibilityManual
    $failedCompatibilityManualPath = Join-Path $failedCompatibilityManual "02-post-upgrade-cloud.md"
    $failedCompatibilityManualText = Get-Content -Raw -LiteralPath $failedCompatibilityManualPath
    $failedCompatibilityManualText = $failedCompatibilityManualText -replace "(?m)^\s*-\s*Missing manual providers after upgrade:\s*none\.\s*$", "- Missing manual providers after upgrade: legacy-openai."
    Set-Content -LiteralPath $failedCompatibilityManualPath -Encoding UTF8 -Value $failedCompatibilityManualText
    Invoke-Tool "compatibility-missing-manual-provider-fails" "verify-07-compatibility-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedCompatibilityManual
    ) @(1)

    $failedCompatibilityRuntimeBoundary = Join-Path $OutputRoot "20260618-1938-compatibility"
    Write-ValidCompatibilityFixture $failedCompatibilityRuntimeBoundary
    $failedCompatibilityManualSwitchPath = Join-Path $failedCompatibilityRuntimeBoundary "04-manual-provider-switch.md"
    $failedCompatibilityProviderSyncPath = Join-Path $failedCompatibilityRuntimeBoundary "05-provider-sync.md"
    $failedCompatibilityRollbackPath = Join-Path $failedCompatibilityRuntimeBoundary "06-rollback-rehearsal.md"
    $failedCompatibilityManualSwitchText = Get-Content -Raw -LiteralPath $failedCompatibilityManualSwitchPath
    $failedCompatibilityProviderSyncText = Get-Content -Raw -LiteralPath $failedCompatibilityProviderSyncPath
    $failedCompatibilityRollbackText = Get-Content -Raw -LiteralPath $failedCompatibilityRollbackPath
    $failedCompatibilityManualSwitchText = $failedCompatibilityManualSwitchText -replace "(?m)^Default user path still shows managed cloud entry point: True\.\s*\r?\n", ""
    $failedCompatibilityProviderSyncText = $failedCompatibilityProviderSyncText -replace "(?m)^Provider sync log secret scan clear: True\.\s*\r?\n", ""
    $failedCompatibilityRollbackText = $failedCompatibilityRollbackText -replace "(?m)^Failed provider write recovery result: pass\.\s*\r?\n", ""
    Set-Content -LiteralPath $failedCompatibilityManualSwitchPath -Encoding UTF8 -Value $failedCompatibilityManualSwitchText
    Set-Content -LiteralPath $failedCompatibilityProviderSyncPath -Encoding UTF8 -Value $failedCompatibilityProviderSyncText
    Set-Content -LiteralPath $failedCompatibilityRollbackPath -Encoding UTF8 -Value $failedCompatibilityRollbackText
    Invoke-Tool "compatibility-runtime-boundary-evidence-missing-fails" "verify-07-compatibility-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedCompatibilityRuntimeBoundary
    ) @(1)

    $staleRunFolderCompatibility = Join-Path $OutputRoot "20260618-1939-compatibility"
    Write-ValidCompatibilityFixture $staleRunFolderCompatibility
    Invoke-Tool "compatibility-run-folder-mismatch-fails" "verify-07-compatibility-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $staleRunFolderCompatibility
    ) @(1)

    $failedBusiness = Join-Path $OutputRoot "20260618-1925-business"
    Write-ValidBusinessReadinessFixture $failedBusiness
    $failedBusinessReport = Join-Path $failedBusiness "11-business-readiness.md"
    $failedBusinessText = Get-Content -Raw -LiteralPath $failedBusinessReport
    $failedBusinessText = $failedBusinessText -replace "(?m)^\s*Business readiness result:\s*pass\s*$", "Business readiness result: fail"
    Set-Content -LiteralPath $failedBusinessReport -Encoding UTF8 -Value $failedBusinessText
    Invoke-Tool "business-result-fail-fails" "verify-07-business-readiness.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedBusiness,
        "-SourceDocsRoot", $businessSourceDocsRoot
    ) @(1)

    $sourceUnresolvedBusiness = Join-Path $OutputRoot "20260618-1931-business"
    Write-ValidBusinessReadinessFixture $sourceUnresolvedBusiness
    Invoke-Tool "business-source-doc-unresolved-fails" "verify-07-business-readiness.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $sourceUnresolvedBusiness,
        "-SourceDocsRoot", $businessUnresolvedSourceDocsRoot
    ) @(1)

    $failedDocs = Join-Path $OutputRoot "20260618-1929-docs"
    Write-ValidDocsProductCopyFixture $failedDocs
    $failedDocsReport = Join-Path $failedDocs "06-docs-product-copy-gate-report.md"
    $failedDocsText = Get-Content -Raw -LiteralPath $failedDocsReport
    $failedDocsText = $failedDocsText -replace "(?m)^\s*Docs product copy result:\s*pass\s*$", "Docs product copy result: fail"
    Set-Content -LiteralPath $failedDocsReport -Encoding UTF8 -Value $failedDocsText
    Invoke-Tool "docs-product-copy-result-fail-fails" "verify-07-docs-product-copy-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $failedDocs
    ) @(1)

    $draftDocs = Join-Path $OutputRoot "20260618-1930-docs"
    Write-ValidDocsProductCopyFixture $draftDocs
    $draftDocsSync = Join-Path $draftDocs "00-docs-sync-record.md"
    $draftDocsText = Get-Content -Raw -LiteralPath $draftDocsSync
    $draftDocsText = $draftDocsText -replace "(?m)^\s*Report status:\s*final\s*$", "Report status: draft"
    Set-Content -LiteralPath $draftDocsSync -Encoding UTF8 -Value $draftDocsText
    Invoke-Tool "docs-product-copy-draft-status-fails" "verify-07-docs-product-copy-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $draftDocs
    ) @(1)

    Invoke-Tool "valid-e2e-passes" "verify-07-evidence.ps1" @("-Root", $Root, "-EvidenceDir", $validE2E) @(0)
    Invoke-Tool "valid-package-passes" "verify-07-package-evidence.ps1" @("-Root", $Root, "-EvidenceDir", $validPackage) @(0)
    Invoke-Tool "valid-compatibility-passes" "verify-07-compatibility-evidence.ps1" @("-Root", $Root, "-EvidenceDir", $validCompatibility) @(0)
    Invoke-Tool "valid-docs-product-copy-passes" "verify-07-docs-product-copy-evidence.ps1" @("-Root", $Root, "-EvidenceDir", $validDocs) @(0)
    Invoke-Tool "valid-aggregate-passes" "verify-07-release-evidence.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", $validE2E,
        "-PackageEvidenceDir", $validPackage,
        "-CompatibilityEvidenceDir", $validCompatibility,
        "-DocsEvidenceDir", $validDocs,
        "-LogRoot", (Join-Path $OutputRoot "valid-aggregate-logs")
    ) @(0)
    $validCoverageSummary = Join-Path $OutputRoot "valid-release-coverage-summary.md"
    Invoke-Tool "valid-fixture-coverage-summary-incomplete" "summarize-07-release-coverage.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", $validE2E,
        "-PackageEvidenceDir", $validPackage,
        "-CompatibilityEvidenceDir", $validCompatibility,
        "-DocsEvidenceDir", $validDocs,
        "-OutputFile", $validCoverageSummary,
        "-FailOnIncomplete"
    ) @(1)
    $validReadinessSummary = Join-Path $OutputRoot "valid-release-readiness-summary.md"
    Invoke-Tool "valid-fixture-readiness-summary-fails" "summarize-07-release-readiness.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", $validE2E,
        "-PackageEvidenceDir", $validPackage,
        "-CompatibilityEvidenceDir", $validCompatibility,
        "-DocsEvidenceDir", $validDocs,
        "-BusinessEvidenceDir", $validBusiness,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-CoverageSummaryFile", $validCoverageSummary,
        "-OutputFile", $validReadinessSummary,
        "-AllowGoCandidate",
        "-FailOnNoGo"
    ) @(1)
    Add-Result "valid-fixture-readiness-summary-file" (Test-Path -LiteralPath $validReadinessSummary -PathType Leaf) $validReadinessSummary
    if (Test-Path -LiteralPath $validReadinessSummary -PathType Leaf) {
        $validReadinessText = Get-Content -Raw -LiteralPath $validReadinessSummary
        Add-Result "valid-fixture-readiness-summary-no-go" ($validReadinessText -match "(?im)^\s*Recommended Module J posture\s*:\s*no-go\s*$") "Sanitized fixture summary must remain no-go."
        Add-Result "valid-fixture-readiness-summary-fixture-marker" ($validReadinessText -match "(?i)marker:\s*fixture") "Sanitized fixture summary must record fixture marker."
        Add-Result "valid-fixture-readiness-summary-business-passed" ($validReadinessText -match "(?im)^\s*Business readiness verification\s*:\s*passed\s*$") "Sanitized fixture summary must still verify business readiness."
    }
    Write-GoCandidateCoverageSummary $goCandidateCoverageSummary
    Write-GoCandidateReadinessSummary $goCandidateReadinessSummary
    Invoke-Tool "module-j-report-without-coverage-summary-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    Invoke-Tool "module-j-report-with-no-go-readiness-summary-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $validReadinessSummary
    ) @(1)

    $missingBusinessReadinessSummary = Join-Path $OutputRoot "go-candidate-missing-business-readiness-summary.md"
    Write-GoCandidateReadinessSummaryWithoutBusiness $missingBusinessReadinessSummary
    Invoke-Tool "module-j-report-with-missing-business-readiness-summary-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $missingBusinessReadinessSummary
    ) @(1)
    Invoke-Tool "module-j-report-without-readiness-summary-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary
    ) @(1)

    $missingCoverageReadinessSummary = Join-Path $OutputRoot "go-candidate-missing-coverage-readiness-summary.md"
    Write-GoCandidateReadinessSummary $missingCoverageReadinessSummary
    $missingCoverageReadinessText = Get-Content -Raw -LiteralPath $missingCoverageReadinessSummary
    $missingCoverageReadinessText = $missingCoverageReadinessText -replace "(?m)^\s*Coverage summary verification:.*\r?\n", ""
    $missingCoverageReadinessText = $missingCoverageReadinessText -replace "(?m)^\s*Coverage status:.*\r?\n", ""
    $missingCoverageReadinessText = $missingCoverageReadinessText -replace "(?m)^\s*Coverage missing count:.*\r?\n", ""
    $missingCoverageReadinessText = $missingCoverageReadinessText -replace "(?m)^\s*Coverage nonrelease marker count:.*\r?\n", ""
    $missingCoverageReadinessText = $missingCoverageReadinessText -replace "(?m)^\s*-\s*Coverage summary verification:.*\r?\n", ""
    Set-Content -LiteralPath $missingCoverageReadinessSummary -Encoding UTF8 -Value $missingCoverageReadinessText
    Invoke-Tool "module-j-report-with-missing-readiness-coverage-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $missingCoverageReadinessSummary
    ) @(1)

    $notAllowedReadinessSummary = Join-Path $OutputRoot "go-candidate-readiness-not-allowed-summary.md"
    Write-GoCandidateReadinessSummary $notAllowedReadinessSummary
    $notAllowedReadinessText = Get-Content -Raw -LiteralPath $notAllowedReadinessSummary
    $notAllowedReadinessText = $notAllowedReadinessText -replace "(?m)^\s*Allow go candidate:\s*true\s*$", "Allow go candidate: false"
    Set-Content -LiteralPath $notAllowedReadinessSummary -Encoding UTF8 -Value $notAllowedReadinessText
    Invoke-Tool "module-j-report-with-readiness-not-allowed-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $notAllowedReadinessSummary
    ) @(1)

    $nongeneratedReadinessSummary = Join-Path $OutputRoot "go-candidate-nongenerated-readiness-summary.md"
    Write-GoCandidateReadinessSummary $nongeneratedReadinessSummary
    $nongeneratedReadinessText = Get-Content -Raw -LiteralPath $nongeneratedReadinessSummary
    $nongeneratedReadinessText = $nongeneratedReadinessText -replace "(?m)^\s*Report status:\s*generated\s*$", "Report status: manual"
    Set-Content -LiteralPath $nongeneratedReadinessSummary -Encoding UTF8 -Value $nongeneratedReadinessText
    Invoke-Tool "module-j-report-with-nongenerated-readiness-summary-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $nongeneratedReadinessSummary
    ) @(1)

    $markerReadinessSummary = Join-Path $OutputRoot "go-candidate-readiness-marker-summary.md"
    Write-GoCandidateReadinessSummary $markerReadinessSummary
    $markerReadinessText = Get-Content -Raw -LiteralPath $markerReadinessSummary
    $markerReadinessText = $markerReadinessText -replace "(?m)^\s*-\s*none\s*$", "- source: e2e; marker: residual-nonrelease-marker; detail: injected self-test marker"
    Set-Content -LiteralPath $markerReadinessSummary -Encoding UTF8 -Value $markerReadinessText
    Invoke-Tool "module-j-report-with-readiness-marker-despite-go-posture-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $markerReadinessSummary
    ) @(1)

    $failedLevel3ReadinessSummary = Join-Path $OutputRoot "go-candidate-readiness-level3-failed-summary.md"
    Write-GoCandidateReadinessSummary $failedLevel3ReadinessSummary
    $failedLevel3ReadinessText = Get-Content -Raw -LiteralPath $failedLevel3ReadinessSummary
    $failedLevel3ReadinessText = $failedLevel3ReadinessText -replace "(?m)^\s*-\s*E2E Level 3 result:\s*pass\s*$", "- E2E Level 3 result: fail"
    Set-Content -LiteralPath $failedLevel3ReadinessSummary -Encoding UTF8 -Value $failedLevel3ReadinessText
    Invoke-Tool "module-j-report-with-readiness-level3-failed-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $failedLevel3ReadinessSummary
    ) @(1)

    $mismatchedReadinessCoveragePath = Join-Path $OutputRoot "go-candidate-mismatched-coverage-path-readiness-summary.md"
    Write-GoCandidateReadinessSummary $mismatchedReadinessCoveragePath
    $mismatchedReadinessCoverageText = Get-Content -Raw -LiteralPath $mismatchedReadinessCoveragePath
    $mismatchedReadinessCoverageText = $mismatchedReadinessCoverageText -replace "(?m)^\s*-\s*Release coverage summary:.*$", "- Release coverage summary: codex-plus-dev-plan/test-runs/wrong-release-coverage-summary.md"
    Set-Content -LiteralPath $mismatchedReadinessCoveragePath -Encoding UTF8 -Value $mismatchedReadinessCoverageText
    Invoke-Tool "module-j-report-with-mismatched-readiness-coverage-path-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $mismatchedReadinessCoveragePath
    ) @(1)

    $mismatchedSummaryPathsReport = Join-Path $OutputRoot "module-j-mismatched-summary-paths.md"
    Write-ValidModuleJReport $mismatchedSummaryPathsReport
    $mismatchedSummaryPathsText = Get-Content -Raw -LiteralPath $mismatchedSummaryPathsReport
    $mismatchedSummaryPathsText = $mismatchedSummaryPathsText -replace "(?m)^\s*-\s*Release coverage summary:.*$", "- Release coverage summary: codex-plus-dev-plan/test-runs/wrong-release-coverage-summary.md"
    $mismatchedSummaryPathsText = $mismatchedSummaryPathsText -replace "(?m)^\s*-\s*Release readiness summary:.*$", "- Release readiness summary: codex-plus-dev-plan/test-runs/wrong-release-readiness-summary.md"
    Set-Content -LiteralPath $mismatchedSummaryPathsReport -Encoding UTF8 -Value $mismatchedSummaryPathsText
    Invoke-Tool "module-j-report-with-mismatched-summary-paths-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $mismatchedSummaryPathsReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)

    $mismatchedEvidenceInputsReport = Join-Path $OutputRoot "module-j-mismatched-evidence-inputs.md"
    Write-ValidModuleJReport $mismatchedEvidenceInputsReport
    $mismatchedEvidenceInputsText = Get-Content -Raw -LiteralPath $mismatchedEvidenceInputsReport
    $mismatchedEvidenceInputsText = $mismatchedEvidenceInputsText -replace "(?m)^\s*-\s*E2E evidence folder:.*$", "- E2E evidence folder: codex-plus-dev-plan/test-runs/wrong-e2e"
    Set-Content -LiteralPath $mismatchedEvidenceInputsReport -Encoding UTF8 -Value $mismatchedEvidenceInputsText
    Invoke-Tool "module-j-report-with-mismatched-evidence-inputs-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $mismatchedEvidenceInputsReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)

    foreach ($moduleJEvidenceInputMismatch in @(
        @{ Name = "package"; Field = "Package evidence folder"; Wrong = "codex-plus-dev-plan/test-runs/wrong-package" },
        @{ Name = "compatibility"; Field = "Compatibility evidence folder"; Wrong = "codex-plus-dev-plan/test-runs/wrong-compatibility" },
        @{ Name = "docs"; Field = "Docs product copy evidence folder"; Wrong = "codex-plus-dev-plan/test-runs/wrong-docs" },
        @{ Name = "business"; Field = "Business readiness folder"; Wrong = "codex-plus-dev-plan/test-runs/wrong-business" }
    )) {
        $mismatchedEvidenceInputsReport = Join-Path $OutputRoot "module-j-mismatched-$($moduleJEvidenceInputMismatch.Name)-evidence-inputs.md"
        Write-ValidModuleJReport $mismatchedEvidenceInputsReport
        $mismatchedEvidenceInputsText = Get-Content -Raw -LiteralPath $mismatchedEvidenceInputsReport
        $mismatchedEvidenceInputsText = $mismatchedEvidenceInputsText -replace ("(?m)^\s*-\s*" + [regex]::Escape($moduleJEvidenceInputMismatch.Field) + ":.*$"), "- $($moduleJEvidenceInputMismatch.Field): $($moduleJEvidenceInputMismatch.Wrong)"
        Set-Content -LiteralPath $mismatchedEvidenceInputsReport -Encoding UTF8 -Value $mismatchedEvidenceInputsText
        Invoke-Tool "module-j-report-with-mismatched-$($moduleJEvidenceInputMismatch.Name)-evidence-inputs-fails" "verify-07-module-j-report.ps1" @(
            "-Root", $Root,
            "-ReportFile", $mismatchedEvidenceInputsReport,
            "-CoverageSummaryFile", $goCandidateCoverageSummary,
            "-ReadinessSummaryFile", $goCandidateReadinessSummary
        ) @(1)
    }

    $missingLevel3Report = Join-Path $OutputRoot "module-j-missing-level3.md"
    Write-ValidModuleJReport $missingLevel3Report
    $missingLevel3ReportText = Get-Content -Raw -LiteralPath $missingLevel3Report
    $missingLevel3ReportText = $missingLevel3ReportText -replace "(?m)^\s*-\s*level 3 result:\s*pass\s*\r?\n", ""
    Set-Content -LiteralPath $missingLevel3Report -Encoding UTF8 -Value $missingLevel3ReportText
    Invoke-Tool "module-j-report-without-level3-pass-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $missingLevel3Report,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)

    $missingBusinessEvidenceHygieneReport = Join-Path $OutputRoot "module-j-missing-business-evidence-hygiene.md"
    Write-ModuleJReportMissingBusinessEvidenceHygiene $missingBusinessEvidenceHygieneReport
    Invoke-Tool "module-j-report-without-business-evidence-hygiene-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $missingBusinessEvidenceHygieneReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    $missingModuleInputsReport = Join-Path $OutputRoot "module-j-missing-module-inputs.md"
    Write-ModuleJReportMissingModuleInputs $missingModuleInputsReport
    Invoke-Tool "module-j-report-without-module-inputs-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $missingModuleInputsReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    $unapprovedContractDriftReport = Join-Path $OutputRoot "module-j-unapproved-contract-drift.md"
    Write-ModuleJReportWithUnapprovedContractDrift $unapprovedContractDriftReport
    Invoke-Tool "module-j-report-with-unapproved-contract-drift-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $unapprovedContractDriftReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    $missingGoPolicySignalsReport = Join-Path $OutputRoot "module-j-missing-go-policy-signals.md"
    Write-ModuleJReportMissingGoPolicySignals $missingGoPolicySignalsReport
    Invoke-Tool "module-j-report-without-go-policy-signals-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $missingGoPolicySignalsReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    $misplacedGoPolicySignalsReport = Join-Path $OutputRoot "module-j-go-policy-keywords-outside-failure-paths.md"
    Write-ModuleJReportWithGoPolicyKeywordsOutsideFailurePaths $misplacedGoPolicySignalsReport
    Invoke-Tool "module-j-report-with-go-policy-keywords-outside-failure-paths-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $misplacedGoPolicySignalsReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    $missingVerificationAvailabilityReport = Join-Path $OutputRoot "module-j-missing-verification-availability.md"
    Write-ModuleJReportMissingVerificationAvailability $missingVerificationAvailabilityReport
    Invoke-Tool "module-j-report-without-verification-availability-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $missingVerificationAvailabilityReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    $missingConflictResolutionReport = Join-Path $OutputRoot "module-j-missing-conflict-resolution.md"
    Write-ModuleJReportMissingConflictResolution $missingConflictResolutionReport
    Invoke-Tool "module-j-report-without-conflict-resolution-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $missingConflictResolutionReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    $missingAcceptedImpactReport = Join-Path $OutputRoot "module-j-missing-accepted-impact.md"
    Write-ModuleJReportMissingAcceptedImpact $missingAcceptedImpactReport
    Invoke-Tool "module-j-report-without-accepted-impact-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $missingAcceptedImpactReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    $moduleJTemplatePath = Join-Path $Root "codex-plus-dev-plan\07-integration-release\reports\module-j-final-report-template.md"
    $moduleJTemplateText = Get-Content -Raw -LiteralPath $moduleJTemplatePath
    Add-Result "module-j-template-has-no-pending-field-labels" (-not ($moduleJTemplateText -match "(?im)^-\s*.*pending.*:")) "Module J template field labels must not include scanner-blocked pending wording."

    $camelTokenReport = Join-Path $OutputRoot "module-j-camelcase-token-field.md"
    Write-ValidModuleJReport $camelTokenReport
    Add-Content -LiteralPath $camelTokenReport -Encoding UTF8 -Value "`napiKey: abcdefghijklmnopqrstuvwxyz123456`n"
    Invoke-Tool "module-j-report-with-camelcase-token-field-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $camelTokenReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)

    $envSecretReport = Join-Path $OutputRoot "module-j-env-secret-field.md"
    Write-ValidModuleJReport $envSecretReport
    Add-Content -LiteralPath $envSecretReport -Encoding UTF8 -Value "`nJWT_SECRET=abcdefghijklmnopqrstuvwxyz123456`n"
    Invoke-Tool "module-j-report-with-env-secret-field-fails" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $envSecretReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(1)
    Invoke-Tool "module-j-report-with-go-candidate-readiness-summary-passes" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(0)
    Invoke-Tool "valid-module-j-report-passes" "verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $validReport,
        "-CoverageSummaryFile", $goCandidateCoverageSummary,
        "-ReadinessSummaryFile", $goCandidateReadinessSummary
    ) @(0)

    if ($SkipHandoff) {
        Add-Result "handoff-suite-skipped" $true "Skipped release handoff self-tests because -SkipHandoff was supplied."
    } else {
    $handoffStamp = "20260618-1912"
    $handoffE2E = Join-Path $OutputRoot "$handoffStamp-e2e"
    $handoffPackage = Join-Path $OutputRoot "$handoffStamp-package"
    $handoffCompatibility = Join-Path $OutputRoot "$handoffStamp-compatibility"
    $handoffDocs = Join-Path $OutputRoot "$handoffStamp-docs"
    $handoffBusiness = Join-Path $OutputRoot "$handoffStamp-business"
    $handoffRelease = Join-Path $OutputRoot "$handoffStamp-release"
    New-Item -ItemType Directory -Force -Path $handoffRelease | Out-Null
    Write-ValidE2EFixture $handoffE2E
    Write-ValidPackageFixture $handoffPackage
    Write-ValidCompatibilityFixture $handoffCompatibility
    Write-ValidDocsProductCopyFixture $handoffDocs
    Write-ValidBusinessReadinessFixture $handoffBusiness
    Update-RunFolderDeclarations $handoffE2E "$handoffStamp-e2e"
    Update-RunFolderDeclarations $handoffCompatibility "$handoffStamp-compatibility"
    Update-RunFolderDeclarations $handoffDocs "$handoffStamp-docs"
    Convert-FixtureEvidenceToReleaseCandidate @($handoffE2E, $handoffPackage, $handoffCompatibility)
    Invoke-Tool "valid-handoff-business-readiness-passes" "verify-07-business-readiness.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $handoffBusiness,
        "-SourceDocsRoot", $businessSourceDocsRoot
    ) @(0)
    Invoke-Tool "valid-handoff-docs-product-copy-passes" "verify-07-docs-product-copy-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $handoffDocs
    ) @(0)

    $handoffCoverage = Join-Path $handoffRelease "release-coverage-summary.md"
    Invoke-Tool "valid-handoff-coverage-summary-passes" "summarize-07-release-coverage.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", $handoffE2E,
        "-PackageEvidenceDir", $handoffPackage,
        "-CompatibilityEvidenceDir", $handoffCompatibility,
        "-DocsEvidenceDir", $handoffDocs,
        "-OutputFile", $handoffCoverage,
        "-FailOnIncomplete"
    ) @(0)
    $handoffWindowsOnlyStamp = "20260618-1913"
    $handoffWindowsOnlyPackage = Join-Path $OutputRoot "$handoffWindowsOnlyStamp-package"
    Write-ValidPackageFixture $handoffWindowsOnlyPackage
    Convert-FixtureEvidenceToReleaseCandidate @($handoffWindowsOnlyPackage)
    foreach ($packageFileName in @("00-artifact-metadata.md", "04-artifact-inspection.md")) {
        $packageFilePath = Join-Path $handoffWindowsOnlyPackage $packageFileName
        $packageFileText = Get-Content -Raw -LiteralPath $packageFilePath
        $packageFileText = $packageFileText -replace "(?im)^\s*-\s*macos-x64-dmg\s*:\s*present\s*$", "- macos-x64-dmg: missing"
        $packageFileText = $packageFileText -replace "(?im)^\s*-\s*macos-arm64-dmg\s*:\s*present\s*$", "- macos-arm64-dmg: missing"
        Set-Content -LiteralPath $packageFilePath -Encoding UTF8 -Value $packageFileText
    }
    foreach ($packageFileName in @("02-macos-x64-dmg.md", "03-macos-arm64-dmg.md")) {
        $packageFilePath = Join-Path $handoffWindowsOnlyPackage $packageFileName
        $packageFileText = Get-Content -Raw -LiteralPath $packageFilePath
        $packageFileText = $packageFileText -replace "(?im)^\s*Result\s*:\s*pass\s*$", "Result: fail"
        Set-Content -LiteralPath $packageFilePath -Encoding UTF8 -Value $packageFileText
    }
    Set-Content -LiteralPath (Join-Path $handoffWindowsOnlyPackage "06-mvp-scope-decision.md") -Encoding UTF8 -Value @"
# 06 MVP Scope Decision

Run folder: $handoffWindowsOnlyStamp-package
Status: owner-approved scope change
Result: pass

MVP package scope: Windows x64 only.

Owner decision: user stated that MVP is Windows-only.

## Deferred Post-MVP

- macOS x64 DMG artifact and install evidence are deferred post-MVP.
- macOS arm64 DMG artifact and install evidence are deferred post-MVP.

## Release Boundary

- This Windows-only MVP scope decision does not waive Windows evidence requirements.
- This Windows-only MVP scope decision does not waive E2E, compatibility, docs, business readiness, Module J, or release go/no-go gates.
- The full cross-platform package verifier without the Windows-only switch must still fail until macOS package evidence exists.
"@
    Invoke-Tool "valid-windows-only-package-evidence-passes" "verify-07-package-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $handoffWindowsOnlyPackage,
        "-WindowsOnlyMvp"
    ) @(0)
    $handoffWindowsOnlyCoverage = Join-Path $handoffRelease "release-coverage-summary-windows-only.md"
    Invoke-Tool "valid-windows-only-coverage-summary-passes" "summarize-07-release-coverage.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", $handoffE2E,
        "-PackageEvidenceDir", $handoffWindowsOnlyPackage,
        "-CompatibilityEvidenceDir", $handoffCompatibility,
        "-DocsEvidenceDir", $handoffDocs,
        "-OutputFile", $handoffWindowsOnlyCoverage,
        "-WindowsOnlyMvp",
        "-FailOnIncomplete"
    ) @(0)
    $handoffWindowsOnlyCoverageText = Get-Content -Raw -LiteralPath $handoffWindowsOnlyCoverage
    Add-Result "windows-only-coverage-records-scope" ($handoffWindowsOnlyCoverageText -match "(?im)^\s*MVP package scope\s*:\s*windows-only\s*$") "Windows-only coverage summary must record the MVP package scope."
    Add-Result "windows-only-coverage-not-cross-platform" (-not ($handoffWindowsOnlyCoverageText -match "(?im)^\s*MVP package scope\s*:\s*cross-platform\s*$")) "Windows-only coverage summary must not use the default cross-platform scope."
    Add-Result "windows-only-coverage-has-zero-missing" ($handoffWindowsOnlyCoverageText -match "(?im)^\s*Missing coverage count\s*:\s*0\s*$") "Windows-only coverage summary must not count deferred macOS package evidence as missing."
    Add-Result "windows-only-coverage-defers-macos-x64" ($handoffWindowsOnlyCoverageText -match "(?im)^\|\s*package\s*\|\s*macOS x64 DMG mount/gatekeeper/reinstall\s*\|\s*deferred-post-mvp\s*\|") "Windows-only coverage summary must defer macOS x64 package evidence."
    Add-Result "windows-only-coverage-defers-macos-arm64" ($handoffWindowsOnlyCoverageText -match "(?im)^\|\s*package\s*\|\s*macOS arm64 DMG mount/gatekeeper/reinstall\s*\|\s*deferred-post-mvp\s*\|") "Windows-only coverage summary must defer macOS arm64 package evidence."
    $mismatchedCoverageForReadiness = Join-Path $OutputRoot "handoff-mismatched-coverage-input-summary.md"
    Copy-Item -LiteralPath $handoffCoverage -Destination $mismatchedCoverageForReadiness -Force
    $mismatchedCoverageForReadinessText = Get-Content -Raw -LiteralPath $mismatchedCoverageForReadiness
    $mismatchedCoverageForReadinessText = $mismatchedCoverageForReadinessText -replace "(?m)^\s*-\s*E2E evidence folder:.*$", "- E2E evidence folder: codex-plus-dev-plan/test-runs/wrong-e2e"
    Set-Content -LiteralPath $mismatchedCoverageForReadiness -Encoding UTF8 -Value $mismatchedCoverageForReadinessText
    $mismatchedCoverageReadiness = Join-Path $OutputRoot "handoff-mismatched-coverage-readiness-summary.md"
    Invoke-Tool "readiness-summary-with-mismatched-coverage-input-fails" "summarize-07-release-readiness.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", $handoffE2E,
        "-PackageEvidenceDir", $handoffPackage,
        "-CompatibilityEvidenceDir", $handoffCompatibility,
        "-DocsEvidenceDir", $handoffDocs,
        "-BusinessEvidenceDir", $handoffBusiness,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-CoverageSummaryFile", $mismatchedCoverageForReadiness,
        "-OutputFile", $mismatchedCoverageReadiness,
        "-AllowGoCandidate",
        "-FailOnNoGo"
    ) @(1)
    if (Test-Path -LiteralPath $mismatchedCoverageReadiness -PathType Leaf) {
        $mismatchedCoverageReadinessText = Get-Content -Raw -LiteralPath $mismatchedCoverageReadiness
        Add-Result "readiness-summary-mismatched-coverage-input-marker" ($mismatchedCoverageReadinessText -match "coverage-e2e-input-mismatch") "Readiness summary must record mismatched coverage summary inputs."
        Add-Result "readiness-summary-mismatched-coverage-verification-failed" ($mismatchedCoverageReadinessText -match "(?im)^\s*Coverage summary verification\s*:\s*failed\s*$") "Readiness summary coverage verification must fail when coverage inputs do not match."
    }
    foreach ($coverageInputMismatch in @(
        @{ Name = "package"; Field = "Package evidence folder"; Wrong = "codex-plus-dev-plan/test-runs/wrong-package" },
        @{ Name = "compatibility"; Field = "Compatibility evidence folder"; Wrong = "codex-plus-dev-plan/test-runs/wrong-compatibility" },
        @{ Name = "docs"; Field = "Docs product copy evidence folder"; Wrong = "codex-plus-dev-plan/test-runs/wrong-docs" }
    )) {
        $mismatchedCoverageForReadiness = Join-Path $OutputRoot "handoff-mismatched-coverage-$($coverageInputMismatch.Name)-input-summary.md"
        Copy-Item -LiteralPath $handoffCoverage -Destination $mismatchedCoverageForReadiness -Force
        $mismatchedCoverageForReadinessText = Get-Content -Raw -LiteralPath $mismatchedCoverageForReadiness
        $mismatchedCoverageForReadinessText = $mismatchedCoverageForReadinessText -replace ("(?m)^\s*-\s*" + [regex]::Escape($coverageInputMismatch.Field) + ":.*$"), "- $($coverageInputMismatch.Field): $($coverageInputMismatch.Wrong)"
        Set-Content -LiteralPath $mismatchedCoverageForReadiness -Encoding UTF8 -Value $mismatchedCoverageForReadinessText
        $mismatchedCoverageReadiness = Join-Path $OutputRoot "handoff-mismatched-coverage-$($coverageInputMismatch.Name)-readiness-summary.md"
        Invoke-Tool "readiness-summary-with-mismatched-coverage-$($coverageInputMismatch.Name)-input-fails" "summarize-07-release-readiness.ps1" @(
            "-Root", $Root,
            "-E2EEvidenceDir", $handoffE2E,
            "-PackageEvidenceDir", $handoffPackage,
            "-CompatibilityEvidenceDir", $handoffCompatibility,
            "-DocsEvidenceDir", $handoffDocs,
            "-BusinessEvidenceDir", $handoffBusiness,
            "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
            "-CoverageSummaryFile", $mismatchedCoverageForReadiness,
            "-OutputFile", $mismatchedCoverageReadiness,
            "-AllowGoCandidate",
            "-FailOnNoGo"
        ) @(1)
    }
    $handoffSummary = Join-Path $handoffRelease "release-readiness-summary.md"
    Invoke-Tool "valid-handoff-readiness-summary-passes" "summarize-07-release-readiness.ps1" @(
        "-Root", $Root,
        "-E2EEvidenceDir", $handoffE2E,
        "-PackageEvidenceDir", $handoffPackage,
        "-CompatibilityEvidenceDir", $handoffCompatibility,
        "-DocsEvidenceDir", $handoffDocs,
        "-BusinessEvidenceDir", $handoffBusiness,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-CoverageSummaryFile", $handoffCoverage,
        "-OutputFile", $handoffSummary,
        "-AllowGoCandidate",
        "-FailOnNoGo"
    ) @(0)
    $handoffReport = Join-Path $handoffRelease "module-j-final-report.md"
    $script:ModuleJFixtureE2EDir = $handoffE2E
    $script:ModuleJFixturePackageDir = $handoffPackage
    $script:ModuleJFixtureCompatibilityDir = $handoffCompatibility
    $script:ModuleJFixtureDocsDir = $handoffDocs
    $script:ModuleJFixtureBusinessDir = $handoffBusiness
    $script:ModuleJFixtureCoverageSummary = $handoffCoverage
    $script:ModuleJFixtureReadinessSummary = $handoffSummary
    Write-ValidModuleJReport $handoffReport
    Write-ValidReleaseHandoffIndex $handoffRelease $handoffStamp $handoffE2E $handoffPackage $handoffCompatibility $handoffDocs $handoffBusiness $handoffCoverage $handoffSummary $handoffReport

    $handoffIndexPath = Join-Path $handoffRelease "00-release-evidence-index.md"
    $originalHandoffIndexText = Get-Content -Raw -LiteralPath $handoffIndexPath
    $mismatchedStampIndexText = $originalHandoffIndexText -replace "(?m)^\s*Run stamp:\s*$handoffStamp\s*$", "Run stamp: 20260618-1913"
    Set-Content -LiteralPath $handoffIndexPath -Encoding UTF8 -Value $mismatchedStampIndexText
    Invoke-Tool "release-handoff-with-index-run-stamp-mismatch-fails" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", $handoffRelease,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-AllowGoCandidate"
    ) @(1)
    Set-Content -LiteralPath $handoffIndexPath -Encoding UTF8 -Value $originalHandoffIndexText

    $misleadingIndexPathText = $originalHandoffIndexText -replace "(?m)^\s*-\s*E2E evidence:.*$", "- E2E evidence: codex-plus-dev-plan/test-runs/wrong-e2e"
    $misleadingIndexPathText = $misleadingIndexPathText + [Environment]::NewLine + "Correct E2E evidence elsewhere: $handoffE2E" + [Environment]::NewLine
    Set-Content -LiteralPath $handoffIndexPath -Encoding UTF8 -Value $misleadingIndexPathText
    Invoke-Tool "release-handoff-with-misleading-index-path-fields-fails" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", $handoffRelease,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-AllowGoCandidate"
    ) @(1)
    Set-Content -LiteralPath $handoffIndexPath -Encoding UTF8 -Value $originalHandoffIndexText

    $originalHandoffSummaryText = Get-Content -Raw -LiteralPath $handoffSummary
    $mismatchedAllowSummaryText = $originalHandoffSummaryText -replace "(?m)^\s*Allow go candidate:\s*true\s*$", "Allow go candidate: false"
    Set-Content -LiteralPath $handoffSummary -Encoding UTF8 -Value $mismatchedAllowSummaryText
    Invoke-Tool "release-handoff-with-mismatched-readiness-allow-fails" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", $handoffRelease,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-AllowGoCandidate"
    ) @(1)
    Set-Content -LiteralPath $handoffSummary -Encoding UTF8 -Value $originalHandoffSummaryText

    $mismatchedDocsResultSummaryText = $originalHandoffSummaryText -replace "(?m)^\s*-\s*Docs product copy result:\s*pass\s*$", "- Docs product copy result: fail"
    Set-Content -LiteralPath $handoffSummary -Encoding UTF8 -Value $mismatchedDocsResultSummaryText
    Invoke-Tool "release-handoff-with-mismatched-readiness-docs-result-fails" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", $handoffRelease,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-AllowGoCandidate"
    ) @(1)
    Set-Content -LiteralPath $handoffSummary -Encoding UTF8 -Value $originalHandoffSummaryText

    $mismatchedBusinessResultSummaryText = $originalHandoffSummaryText -replace "(?m)^\s*-\s*Business readiness result:\s*pass\s*$", "- Business readiness result: fail"
    Set-Content -LiteralPath $handoffSummary -Encoding UTF8 -Value $mismatchedBusinessResultSummaryText
    Invoke-Tool "release-handoff-with-mismatched-readiness-business-result-fails" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", $handoffRelease,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-AllowGoCandidate"
    ) @(1)
    Set-Content -LiteralPath $handoffSummary -Encoding UTF8 -Value $originalHandoffSummaryText

    foreach ($handoffIndexResultMismatch in @(
        @{ Name = "docs-result"; Pattern = "(?m)^\s*-\s*docs product copy verification:\s*passed\s*$"; Replacement = "- docs product copy verification: failed" },
        @{ Name = "business-result"; Pattern = "(?m)^\s*-\s*business readiness verification:\s*passed\s*$"; Replacement = "- business readiness verification: failed" },
        @{ Name = "final-recommendation"; Pattern = "(?m)^\s*-\s*Final recommendation:\s*go with accepted risks\s*$"; Replacement = "- Final recommendation: no-go" }
    )) {
        $mismatchedIndexResultText = $originalHandoffIndexText -replace $handoffIndexResultMismatch.Pattern, $handoffIndexResultMismatch.Replacement
        Set-Content -LiteralPath $handoffIndexPath -Encoding UTF8 -Value $mismatchedIndexResultText
        Invoke-Tool "release-handoff-with-mismatched-$($handoffIndexResultMismatch.Name)-fails" "verify-07-release-handoff.ps1" @(
            "-Root", $Root,
            "-ReleaseDir", $handoffRelease,
            "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
            "-AllowGoCandidate"
        ) @(1)
    }
    Set-Content -LiteralPath $handoffIndexPath -Encoding UTF8 -Value $originalHandoffIndexText

    $handoffMismatchedCoverageRelease = Join-Path $OutputRoot "$handoffStamp-mismatched-coverage-release"
    New-Item -ItemType Directory -Force -Path $handoffMismatchedCoverageRelease | Out-Null
    $mismatchedHandoffCoverage = Join-Path $handoffMismatchedCoverageRelease "release-coverage-summary.md"
    $mismatchedHandoffSummary = Join-Path $handoffMismatchedCoverageRelease "release-readiness-summary.md"
    $mismatchedHandoffReport = Join-Path $handoffMismatchedCoverageRelease "module-j-final-report.md"
    Copy-Item -LiteralPath $handoffCoverage -Destination $mismatchedHandoffCoverage -Force
    $mismatchedHandoffCoverageText = Get-Content -Raw -LiteralPath $mismatchedHandoffCoverage
    $mismatchedHandoffCoverageText = $mismatchedHandoffCoverageText -replace "(?m)^\s*-\s*E2E evidence folder:.*$", "- E2E evidence folder: codex-plus-dev-plan/test-runs/wrong-e2e"
    Set-Content -LiteralPath $mismatchedHandoffCoverage -Encoding UTF8 -Value $mismatchedHandoffCoverageText
    Copy-Item -LiteralPath $handoffSummary -Destination $mismatchedHandoffSummary -Force
    $mismatchedHandoffSummaryText = Get-Content -Raw -LiteralPath $mismatchedHandoffSummary
    $mismatchedHandoffSummaryText = $mismatchedHandoffSummaryText -replace "(?m)^\s*-\s*Release coverage summary:.*$", "- Release coverage summary: $mismatchedHandoffCoverage"
    Set-Content -LiteralPath $mismatchedHandoffSummary -Encoding UTF8 -Value $mismatchedHandoffSummaryText
    $script:ModuleJFixtureCoverageSummary = $mismatchedHandoffCoverage
    $script:ModuleJFixtureReadinessSummary = $mismatchedHandoffSummary
    Write-ValidModuleJReport $mismatchedHandoffReport
    Write-ValidReleaseHandoffIndex $handoffMismatchedCoverageRelease $handoffStamp $handoffE2E $handoffPackage $handoffCompatibility $handoffDocs $handoffBusiness $mismatchedHandoffCoverage $mismatchedHandoffSummary $mismatchedHandoffReport
    Invoke-Tool "release-handoff-with-mismatched-coverage-summary-fails" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", $handoffMismatchedCoverageRelease,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-AllowGoCandidate"
    ) @(1)

    $handoffMissingResultsRelease = Join-Path $OutputRoot "$handoffStamp-missing-results-release"
    New-Item -ItemType Directory -Force -Path $handoffMissingResultsRelease | Out-Null
    $missingResultsCoverage = Join-Path $handoffMissingResultsRelease "release-coverage-summary.md"
    $missingResultsSummary = Join-Path $handoffMissingResultsRelease "release-readiness-summary.md"
    $missingResultsReport = Join-Path $handoffMissingResultsRelease "module-j-final-report.md"
    Copy-Item -LiteralPath $handoffCoverage -Destination $missingResultsCoverage -Force
    Copy-Item -LiteralPath $handoffSummary -Destination $missingResultsSummary -Force
    $script:ModuleJFixtureCoverageSummary = $missingResultsCoverage
    $script:ModuleJFixtureReadinessSummary = $missingResultsSummary
    Write-ValidModuleJReport $missingResultsReport
    Write-ValidReleaseHandoffIndex $handoffMissingResultsRelease $handoffStamp $handoffE2E $handoffPackage $handoffCompatibility $handoffDocs $handoffBusiness $missingResultsCoverage $missingResultsSummary $missingResultsReport
    Remove-ReleaseHandoffVerificationResults $handoffMissingResultsRelease
    Invoke-Tool "release-handoff-without-final-verification-results-fails" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", $handoffMissingResultsRelease,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-AllowGoCandidate"
    ) @(1)

    Invoke-Tool "valid-release-handoff-passes" "verify-07-release-handoff.ps1" @(
        "-Root", $Root,
        "-ReleaseDir", $handoffRelease,
        "-BusinessSourceDocsRoot", $businessSourceDocsRoot,
        "-AllowGoCandidate"
    ) @(0)
    }
} finally {
    if (-not $KeepArtifacts -and (Test-Path -LiteralPath $OutputRoot)) {
        Remove-Item -LiteralPath $OutputRoot -Recurse -Force
    }
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 evidence tooling self-test failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "07 evidence tooling self-test passed."
exit 0
