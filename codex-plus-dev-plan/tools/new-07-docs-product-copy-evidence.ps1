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

if ($Timestamp -match "^\d{8}-\d{4}-docs$") {
    $runName = $Timestamp
} elseif ($Timestamp -match "^\d{8}-\d{4}$") {
    $runName = "$Timestamp-docs"
} else {
    throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-docs."
}

$runPath = Join-Path $OutputRoot $runName
if ((Test-Path -LiteralPath $runPath) -and -not $Force) {
    throw "Docs/product-copy evidence run already exists: $runPath. Use -Force only when intentionally regenerating placeholders."
}

New-Item -ItemType Directory -Force -Path $runPath | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $runPath "05-html-visual-evidence") | Out-Null

function Write-DocsEvidenceFile {
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

Write-DocsEvidenceFile "00-docs-sync-record.md" "00 Docs Sync Record" @"
Report status: draft

## Required Evidence

- TODO: Public docs reviewed against backend-configured commercial copy.
- TODO: Control Plane, Data Plane, Client Runtime and Platform Ops wording aligned.
- TODO: No public copy promises a fixed built-in price, fixed built-in model or fixed built-in quota.
- TODO: User guide, admin operations guide, release notes and HTML product spec are linked.
- TODO: E2E-dependent release claims are either backed by verified evidence or omitted.
"@

Write-DocsEvidenceFile "01-user-guide.md" "01 User Guide" @"
## Required Evidence

- TODO: Codex++ Cloud install and login flow.
- TODO: Backend-configured plan/model/quota snapshot wording.
- TODO: Old manual provider preservation wording.
- TODO: Not purchased, expired, insufficient balance, device revoked, model unavailable and local configuration failure states.
"@

Write-DocsEvidenceFile "02-admin-operations-guide.md" "02 Admin Operations Guide" @"
## Required Evidence

- TODO: Prices, plans, models and quota are described as backend-configured.
- TODO: Configuration version, canary, rollback, audit and reconciliation flows.
- TODO: No public secret, upstream credential or true cost structure disclosure.
"@

Write-DocsEvidenceFile "03-release-notes.md" "03 Release Notes" @"
Status: draft

## Required Evidence

- TODO: Final release scope.
- TODO: User and admin changes.
- TODO: Rollback paths.
- TODO: Compatibility impact.
- TODO: Known risks and owner decisions.
"@

Write-DocsEvidenceFile "04-html-sync-evidence.md" "04 HTML Sync Evidence" @"
Result: pending

## Required Evidence

- TODO: Static sync passed.
- TODO: Local Chromium visual evidence passed.
- TODO: In-app browser local-file visual evidence passed, or an approved remote/public preview pass is linked.
- TODO: Desktop 1440x900 and mobile 390x844 screenshots reviewed without obvious overlap or right-side clipping.
- TODO: Fixed-value residue scan result.
"@

Set-Content -LiteralPath (Join-Path $runPath "codex-plus-product-spec.html") -Encoding UTF8 -Value @"
<!-- TODO: Copy the release-candidate codex-plus-product-spec.html snapshot here. -->
"@

Set-Content -LiteralPath (Join-Path $runPath "05-html-visual-evidence\visual-review.md") -Encoding UTF8 -Value @"
# 05 Visual Review

Run folder: $runName
Status: TODO

- TODO: Desktop 1440x900 screenshot: 05-html-visual-evidence/product-spec-desktop.png
- TODO: Mobile 390x844 screenshot: 05-html-visual-evidence/product-spec-mobile.png
- TODO: In-app browser or approved browser preview result: pending
"@

Set-Content -LiteralPath (Join-Path $runPath "05-html-visual-evidence\product-spec-desktop.png") -Encoding Byte -Value @()
Set-Content -LiteralPath (Join-Path $runPath "05-html-visual-evidence\product-spec-mobile.png") -Encoding Byte -Value @()

Write-DocsEvidenceFile "06-docs-product-copy-gate-report.md" "06 Docs Product Copy Gate Report" @"
Docs product copy result: fail

## Commands Executed

- TODO: List static sync, visual review and residue scan commands.

## Evidence Links

- TODO: Link sanitized screenshots, review notes and release-candidate document snapshots.

## Remaining Risks

- TODO: Summarize copy, documentation and visual evidence risks.

## Release Boundary

- TODO: State whether docs/product-copy evidence is ready for Module J. This does not override E2E, package, compatibility, business readiness or release go/no-go.
"@

Write-Host "Created 07 docs/product-copy evidence scaffold: $runPath"
Write-Host "Fill all TODO items with sanitized final documentation evidence, then run:"
Write-Host "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-docs-product-copy-evidence.ps1 -EvidenceDir $runPath"
