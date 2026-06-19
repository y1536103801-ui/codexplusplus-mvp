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

if ($Timestamp -match "^\d{8}-\d{4}-compatibility$") {
    $runName = $Timestamp
} elseif ($Timestamp -match "^\d{8}-\d{4}$") {
    $runName = "$Timestamp-compatibility"
} else {
    throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-compatibility."
}

$runPath = Join-Path $OutputRoot $runName
if ((Test-Path -LiteralPath $runPath) -and -not $Force) {
    throw "Compatibility evidence run already exists: $runPath. Use -Force only when intentionally regenerating placeholders."
}

New-Item -ItemType Directory -Force -Path $runPath | Out-Null

function Write-CompatibilityEvidenceFile {
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

Write-CompatibilityEvidenceFile "00-test-context.md" "00 Test Context" @"
## Required Evidence

- TODO: Tester, date, operating system, runtime build or commit.
- TODO: Codex++ desktop version before upgrade.
- TODO: Codex++ desktop version after upgrade.
- TODO: Legacy settings source and sanitized snapshot path.
- TODO: Test environment owner and rollback owner.
- TODO: All snapshots token-field scan clear: True.
- TODO: All snapshots commercial-policy scan clear: True.
- TODO: Legacy relayProfiles/settings parsed: True.
- TODO: Manual provider content comparison uses nonprinted base URL/API key hashes: True.
"@

Write-CompatibilityEvidenceFile "01-pre-upgrade-snapshot.md" "01 Pre-Upgrade Snapshot" @"
Result: pending

## Required Evidence

- TODO: Run `tools/inspect-07-compatibility-snapshots.ps1` with pre-upgrade, post-upgrade, logout, and rollback provider snapshots when sanitized snapshots are available.
- TODO: Manual provider names and types before upgrade.
- TODO: Base URLs redacted.
- TODO: API keys redacted.
- TODO: Default provider before upgrade.
- TODO: Settings snapshot path, screenshot path, and redacted log path.
- TODO: Legacy relayProfiles/settings parsed: True.
- TODO: Manual provider count: at least one.
"@

Write-CompatibilityEvidenceFile "02-post-upgrade-cloud.md" "02 Post-Upgrade Managed Cloud" @"
Result: pending

## Required Evidence

- TODO: Manual providers preserved after upgrade.
- TODO: Manual provider content unchanged after upgrade: True.
- TODO: Codex++ Cloud provider written or refreshed without overwriting manual providers.
- TODO: Advanced provider configuration remains reachable.
- TODO: No plan, price, multiplier, entitlement, or usage policy data was written by migration.
- TODO: Managed provider stores only required runtime configuration.
- TODO: Missing manual providers after upgrade: none.
- TODO: Manual providers with changed content after upgrade: none.
"@

Write-CompatibilityEvidenceFile "03-cloud-logout-boundary.md" "03 Cloud Logout Boundary" @"
Result: pending

## Required Evidence

- TODO: Cloud login creates only expected cloud/session state.
- TODO: Cloud logout clears cloud session state.
- TODO: Runtime cloud login/logout evidence result: pass.
- TODO: Manual providers remain unchanged after logout.
- TODO: Manual provider content unchanged after logout: True.
- TODO: Redacted before/after provider snapshots.
- TODO: Missing manual providers after logout: none.
- TODO: Manual providers with changed content after logout: none.
- TODO: Logout token-field scan clear: True.
"@

Write-CompatibilityEvidenceFile "04-manual-provider-switch.md" "04 Manual Provider Switch" @"
Result: pending

## Required Evidence

- TODO: Manual provider can still be selected after upgrade.
- TODO: Manual provider can still be used after managed cloud refresh.
- TODO: Runtime manual provider selection result: pass.
- TODO: Runtime manual provider request result: pass.
- TODO: Manual provider content unchanged after managed cloud refresh: True.
- TODO: Default user path still shows managed cloud entry point.
- TODO: Advanced users can reach provider configuration.
"@

Write-CompatibilityEvidenceFile "05-provider-sync.md" "05 Provider Sync" @"
Result: pending

## Required Evidence

- TODO: Provider sync recognizes legacy profiles.
- TODO: Provider sync does not corrupt manual provider entries.
- TODO: Runtime provider sync log review result: pass.
- TODO: Provider sync does not log full API keys, JWTs, Authorization headers, upstream credentials, or .env secrets.
- TODO: Redacted sync logs and snapshot diff.
- TODO: Changed content after upgrade: none.
- TODO: Changed content after logout: none.
"@

Write-CompatibilityEvidenceFile "06-rollback-rehearsal.md" "06 Rollback Rehearsal" @"
Result: pending

## Required Evidence

- TODO: Config rollback preserves or recovers manual providers.
- TODO: Runtime rollback rehearsal result: pass.
- TODO: Manual provider content unchanged after rollback: True.
- TODO: Desktop rollback keeps advanced provider settings reachable.
- TODO: Backend/gateway rollback does not force managed-provider-only assumptions.
- TODO: Failed provider write recovery from last settings snapshot.
- TODO: User-side key exposure response, if applicable, is redacted and owned.
- TODO: Missing manual providers after rollback: none.
- TODO: Manual providers with changed content after rollback: none.
"@

Write-CompatibilityEvidenceFile "07-compatibility-gate-report.md" "07 Compatibility Gate Report" @"
Compatibility evidence result: fail
Compatibility snapshot subset result: pending
Runtime compatibility result: pending

## Commands Executed

- TODO: List upgrade, login, logout, provider switch, sync, and rollback commands.

## Evidence Links

- TODO: Link sanitized snapshots, screenshots, logs, diffs, or external run records.

## Remaining Risks

- TODO: Summarize compatibility risks and owners.

## Release Boundary

- TODO: State whether compatibility evidence is ready for Module J. This does not override E2E, package, or release go/no-go.
"@

Write-Host "Created 07 compatibility evidence scaffold: $runPath"
Write-Host "Fill all TODO items with sanitized compatibility evidence, then run:"
Write-Host "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1 -EvidenceDir $runPath"
