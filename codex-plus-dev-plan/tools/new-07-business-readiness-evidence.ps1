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

if ($Timestamp -match "^\d{8}-\d{4}-business$") {
    $runStamp = $Timestamp -replace "-business$", ""
} elseif ($Timestamp -match "^\d{8}-\d{4}$") {
    $runStamp = $Timestamp
} else {
    throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-business."
}

$businessRunPath = Join-Path $OutputRoot "$runStamp-business"
if ((Test-Path -LiteralPath $businessRunPath) -and -not $Force) {
    throw "Business readiness evidence already exists: $businessRunPath. Use -Force only when intentionally regenerating placeholders."
}

New-Item -ItemType Directory -Force -Path $businessRunPath | Out-Null

$businessReadiness = @"
# 11 Business Readiness

Run folder: $runStamp-business
Status: scaffold only
Business readiness result: pending

This scaffold captures the Phase 9 business readiness gate from CODEX-AUTONOMOUS-TEST-RUNBOOK.md. It is not release evidence until every TODO and pending marker is replaced with sanitized owner-approved decisions.

## Source Documents

- PRODUCTION-ENVIRONMENT-MATRIX.md: TODO
- BUSINESS-CONFIG-DECISION-TABLE.md: TODO
- SERVER-SIZING-AND-SCALING-GUIDE.md: TODO
- DEPLOYMENT-AUTOMATION-RUNBOOK.md: TODO
- SECURITY-REVIEW-PLAN.md: TODO
- COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md: TODO
- OBSERVABILITY-SLO-ALERTING-PLAN.md: TODO
- COST-CONTROL-AND-ABUSE-RUNBOOK.md: TODO
- SUPPORT-OPERATIONS-RUNBOOK.md: TODO

## Gate Matrix

| Gate | Status | Owner | Evidence | Deferred risk | Mitigation | Latest decision date |
| --- | --- | --- | --- | --- | --- | --- |
| production environment values | pending | TODO | TODO | TODO | TODO | TODO |
| business config decisions | pending | TODO | TODO | TODO | TODO | TODO |
| server sizing and scaling | pending | TODO | TODO | TODO | TODO | TODO |
| deployment automation backup rollback healthcheck | pending | TODO | TODO | TODO | TODO | TODO |
| security review P0/P1/P2 | pending | TODO | TODO | TODO | TODO | TODO |
| compliance privacy legal payment provider terms refund policy | pending | TODO | TODO | TODO | TODO | TODO |
| observability SLO dashboards alert routing | pending | TODO | TODO | TODO | TODO | TODO |
| cost control abuse spend caps emergency shutoff | pending | TODO | TODO | TODO | TODO | TODO |
| support operations paid-user support refund compensation admin recovery | pending | TODO | TODO | TODO | TODO | TODO |
| human business or legal decisions | pending | TODO | TODO | TODO | TODO | TODO |

## Required No-Go Scan

- Open P0/P1 security items: pending
- Missing production value: pending
- Missing first-launch package/model/quota/payment/cost decision: pending
- Privacy policy, terms, refund policy and provider terms: pending
- Dashboard, SLO and alert routing: pending
- Cost cap and emergency stop: pending
- Paid-user support and entitlement correction: pending

## Release Boundary

This file proves business readiness only after it is fully completed and passes tools/verify-07-business-readiness.ps1. It does not execute E2E, build packages, run compatibility migration, or make the technical release go/no-go decision.
"@

Set-Content -LiteralPath (Join-Path $businessRunPath "11-business-readiness.md") -Encoding UTF8 -Value $businessReadiness

Write-Host "Created 07 business readiness evidence scaffold: $businessRunPath"
Write-Host "- $(Join-Path $businessRunPath "11-business-readiness.md")"
