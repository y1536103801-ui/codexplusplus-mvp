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

if ($Timestamp -match "^\d{8}-\d{4}-package$") {
    $runName = $Timestamp
} elseif ($Timestamp -match "^\d{8}-\d{4}$") {
    $runName = "$Timestamp-package"
} else {
    throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-package."
}

$runPath = Join-Path $OutputRoot $runName
if ((Test-Path -LiteralPath $runPath) -and -not $Force) {
    throw "Package evidence run already exists: $runPath. Use -Force only when intentionally regenerating placeholders."
}

New-Item -ItemType Directory -Force -Path $runPath | Out-Null

function Write-PackageEvidenceFile {
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

Write-PackageEvidenceFile "00-artifact-metadata.md" "00 Artifact Metadata" @"
## Required Evidence

- TODO: Version/tag.
- TODO: Build source branch/commit.
- TODO: CI run URL or packaging host record.
- TODO: Artifact names:
  - Windows x64 setup.
  - macOS x64 DMG.
  - macOS arm64 DMG.
- TODO: Artifact hashes for all three artifacts.
"@

Write-PackageEvidenceFile "01-windows-x64-install.md" "01 Windows x64 Install Evidence" @"
Result: pending

## Required Evidence

- TODO: Test machine or VM identity.
- TODO: Windows version and architecture.
- TODO: Installer file and hash.
- TODO: Fresh install evidence.
- TODO: Desktop shortcuts for Codex++ and Codex++ Manager.
- TODO: Start Menu folder with launcher, Manager, and uninstall entries.
- TODO: Apps and Features uninstall metadata.
- TODO: Silent launcher starts Codex without Manager UI by default.
- TODO: Manager opens login, install assistant, diagnostics, and advanced configuration.
- TODO: Missing-Codex first-run assistant behavior.
- TODO: Overwrite install evidence.
- TODO: Uninstall and reinstall evidence.
"@

Write-PackageEvidenceFile "02-macos-x64-dmg.md" "02 macOS x64 DMG Evidence" @"
Result: pending

## Required Evidence

- TODO: Test machine or runner identity.
- TODO: macOS version and x64 architecture.
- TODO: DMG file and hash.
- TODO: DMG mount evidence.
- TODO: Codex++.app and Codex++ 管理工具.app presence.
- TODO: Copy/open from /Applications.
- TODO: Silent app launches hidden Dock path.
- TODO: Manager app opens UI.
- TODO: Missing-Codex first-run assistant behavior.
- TODO: Overwrite install, uninstall by removing apps, and reinstall evidence.
- TODO: Gatekeeper/quarantine behavior.
"@

Write-PackageEvidenceFile "03-macos-arm64-dmg.md" "03 macOS arm64 DMG Evidence" @"
Result: pending

## Required Evidence

- TODO: Test machine or runner identity.
- TODO: macOS version and arm64 architecture.
- TODO: DMG file and hash.
- TODO: DMG mount evidence.
- TODO: Codex++.app and Codex++ 管理工具.app presence.
- TODO: Copy/open from /Applications.
- TODO: Silent app launches hidden Dock path.
- TODO: Manager app opens UI.
- TODO: Missing-Codex first-run assistant behavior.
- TODO: Overwrite install, uninstall by removing apps, and reinstall evidence.
- TODO: Gatekeeper/quarantine behavior.
"@

Write-PackageEvidenceFile "04-artifact-inspection.md" "04 Artifact Inspection" @"
Result: pending

## Required Evidence

- TODO: Run `tools/inspect-07-package-artifacts.ps1` against the generated artifact directory when Windows setup and macOS x64/arm64 DMG files are available.
- TODO: Package does not embed a shared Key.
- TODO: Package does not embed user credentials.
- TODO: Package does not embed fixed price, plan, or model policy.
- TODO: Installer scripts do not write Codex credentials.
- TODO: Package install does not overwrite existing manual provider configuration.
- TODO: Inspection command list and sanitized output links.
"@

Write-PackageEvidenceFile "05-package-gate-report.md" "05 Package Gate Report" @"
Package evidence result: fail

## Commands Executed

- TODO: List packaging, artifact inspection, install, overwrite, uninstall, and reinstall commands.

## Evidence Links

- TODO: Link sanitized logs, screenshots, CI artifacts, and platform notes.

## Remaining Risks

- TODO: Summarize platform-specific risks and owners.

## Release Boundary

- TODO: State whether package evidence is ready for Module J. This does not override E2E, compatibility, or release go/no-go.
"@

Write-Host "Created 07 package evidence scaffold: $runPath"
Write-Host "Fill all TODO items with sanitized platform evidence, then run:"
Write-Host "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-package-evidence.ps1 -EvidenceDir $runPath"
