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

if ($Timestamp -match "^\d{8}-\d{4}-release$") {
    $runStamp = $Timestamp -replace "-release$", ""
} elseif ($Timestamp -match "^\d{8}-\d{4}$") {
    $runStamp = $Timestamp
} else {
    throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-release."
}

$releaseRunName = "$runStamp-release"
$releaseRunPath = Join-Path $OutputRoot $releaseRunName
if ((Test-Path -LiteralPath $releaseRunPath) -and -not $Force) {
    throw "Release evidence set already exists: $releaseRunPath. Use -Force only when intentionally regenerating placeholders."
}

New-Item -ItemType Directory -Force -Path $OutputRoot | Out-Null

function Invoke-ScaffoldGenerator {
    param([string]$ScriptName)
    $scriptPath = Join-Path $PSScriptRoot $ScriptName
    if (-not (Test-Path -LiteralPath $scriptPath -PathType Leaf)) {
        throw "Missing scaffold generator: $scriptPath"
    }

    $args = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", $scriptPath,
        "-Root", $Root,
        "-OutputRoot", $OutputRoot,
        "-Timestamp", $runStamp
    )
    if ($Force) {
        $args += "-Force"
    }

    & powershell @args
    if ($LASTEXITCODE -ne 0) {
        throw "$ScriptName failed with exit code $LASTEXITCODE."
    }
}

Invoke-ScaffoldGenerator "new-07-evidence-run.ps1"
Invoke-ScaffoldGenerator "new-07-package-evidence.ps1"
Invoke-ScaffoldGenerator "new-07-compatibility-evidence.ps1"
Invoke-ScaffoldGenerator "new-07-docs-product-copy-evidence.ps1"
Invoke-ScaffoldGenerator "new-07-business-readiness-evidence.ps1"

New-Item -ItemType Directory -Force -Path $releaseRunPath | Out-Null

$templatePath = Join-Path $Root "codex-plus-dev-plan\07-integration-release\reports\module-j-final-report-template.md"
$draftReportPath = Join-Path $releaseRunPath "module-j-final-report-draft.md"
Copy-Item -LiteralPath $templatePath -Destination $draftReportPath -Force

$e2ePath = Join-Path $OutputRoot "$runStamp-e2e"
$packagePath = Join-Path $OutputRoot "$runStamp-package"
$compatibilityPath = Join-Path $OutputRoot "$runStamp-compatibility"
$docsPath = Join-Path $OutputRoot "$runStamp-docs"
$businessPath = Join-Path $OutputRoot "$runStamp-business"

$index = @"
# 07 Release Evidence Set

Run stamp: $runStamp
Status: scaffold only

This folder coordinates the 07 release evidence set. It does not contain executed evidence, and it does not change the release recommendation.

## Generated Evidence Folders

- E2E evidence: $e2ePath
- Package evidence: $packagePath
- Compatibility evidence: $compatibilityPath
- Docs product copy evidence: $docsPath
- Business readiness evidence: $businessPath
- Module J report draft: $draftReportPath
- Release coverage summary: $releaseRunPath\release-coverage-summary.md
- Release readiness summary: $releaseRunPath\release-readiness-summary.md
- Module J final report: $releaseRunPath\module-j-final-report.md

## Final Verification Results

- aggregate verifier result: pending
- docs product copy verification: pending
- business readiness verification: pending
- coverage summary status: pending
- readiness summary posture: pending
- Module J report verification: pending
- Final recommendation: pending

## Required Fill Order

1. Replace every placeholder in the E2E, package, compatibility, docs product copy and business readiness folders with sanitized runtime/platform/docs/business evidence.
2. Run each individual verifier:
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-evidence.ps1 -EvidenceDir $e2ePath`
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-package-evidence.ps1 -EvidenceDir $packagePath`
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1 -EvidenceDir $compatibilityPath`
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-docs-product-copy-evidence.ps1 -EvidenceDir $docsPath`
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-business-readiness.ps1 -EvidenceDir $businessPath`
3. Run the aggregate release evidence verifier:
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1 -E2EEvidenceDir $e2ePath -PackageEvidenceDir $packagePath -CompatibilityEvidenceDir $compatibilityPath -DocsEvidenceDir $docsPath`
4. Generate the release coverage matrix:
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1 -E2EEvidenceDir $e2ePath -PackageEvidenceDir $packagePath -CompatibilityEvidenceDir $compatibilityPath -DocsEvidenceDir $docsPath -OutputFile $releaseRunPath\release-coverage-summary.md -FailOnIncomplete`
5. Generate the conservative Module J readiness summary:
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1 -E2EEvidenceDir $e2ePath -PackageEvidenceDir $packagePath -CompatibilityEvidenceDir $compatibilityPath -DocsEvidenceDir $docsPath -BusinessEvidenceDir $businessPath -CoverageSummaryFile $releaseRunPath\release-coverage-summary.md -OutputFile $releaseRunPath\release-readiness-summary.md -FailOnNoGo`
6. Fill the Module J report draft from the verified evidence, coverage matrix and readiness summary, then save the final version as `$releaseRunPath\module-j-final-report.md`.
7. Change this index to `Status: final`, replace scaffold wording with final handoff wording, fill `Final Verification Results`, and run:
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-module-j-report.ps1 -ReportFile $releaseRunPath\module-j-final-report.md -CoverageSummaryFile $releaseRunPath\release-coverage-summary.md -ReadinessSummaryFile $releaseRunPath\release-readiness-summary.md`
   - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-handoff.ps1 -ReleaseDir $releaseRunPath`
8. Copy the verified final Module J report to `codex-plus-dev-plan/07-integration-release/reports/module-j-final-report.md` for the release handoff.

## Release Boundary

- The generated folders intentionally fail their verifiers until real sanitized evidence replaces every placeholder and pending marker.
- A passing aggregate evidence verifier proves evidence hygiene only.
- The readiness summary is conservative and must remain no-go when it sees fixture, scaffold, subset, pending, missing external evidence, or failed/missing business readiness markers.
- The Module J report still owns the final go/no-go recommendation.
"@

Set-Content -LiteralPath (Join-Path $releaseRunPath "00-release-evidence-index.md") -Encoding UTF8 -Value $index

Write-Host "Created 07 release evidence set: $releaseRunPath"
Write-Host "Generated:"
Write-Host "- $e2ePath"
Write-Host "- $packagePath"
Write-Host "- $compatibilityPath"
Write-Host "- $docsPath"
Write-Host "- $businessPath"
Write-Host "- $draftReportPath"
