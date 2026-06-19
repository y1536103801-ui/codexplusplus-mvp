param(
    [string]$Root,
    [string]$E2EEvidenceDir,
    [string]$PackageEvidenceDir,
    [string]$CompatibilityEvidenceDir,
    [string]$DocsEvidenceDir,
    [string]$OutputFile,
    [switch]$WindowsOnlyMvp,
    [switch]$FailOnIncomplete
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($OutputFile)) {
    throw "OutputFile is required. Example: -OutputFile codex-plus-dev-plan/test-runs/20260618-1911-release/release-coverage-summary.md"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputFile)) {
    $OutputFile = Join-Path $Root $OutputFile
}

function Resolve-EvidencePath {
    param([string]$EvidenceDir)
    if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
        return ""
    }
    if ([System.IO.Path]::IsPathRooted($EvidenceDir)) {
        return [System.IO.Path]::GetFullPath($EvidenceDir)
    }
    return [System.IO.Path]::GetFullPath((Join-Path $Root $EvidenceDir))
}

function Get-RelativePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    $base = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath($Path)
    if ($full.StartsWith($base, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $full.Substring($base.Length).TrimStart('\', '/')
    }
    return $full
}

function Read-TextIfPresent {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return ""
    }
    return Get-Content -Raw -LiteralPath $Path
}

function ConvertTo-SafeText {
    param([string]$Text)
    if ($null -eq $Text) {
        return ""
    }
    $safe = $Text
    $safe = $safe -replace "(?is)\bsk-proj-[A-Za-z0-9_-]{8,}\b", "[redacted-openai-project-key]"
    $safe = $safe -replace "(?is)\bsk-ant-[A-Za-z0-9_-]{8,}\b", "[redacted-anthropic-key]"
    $safe = $safe -replace "(?is)\bsk-[A-Za-z0-9_-]{8,}\b", "[redacted-api-key]"
    $safe = $safe -replace "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b", "[redacted-jwt]"
    $safe = $safe -replace "(?is)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}", "Authorization: [redacted]"
    return $safe
}

function Add-Requirement {
    param(
        [System.Collections.Generic.List[object]]$Requirements,
        [string]$Lane,
        [string]$Requirement,
        [string]$File,
        [string]$Pattern
    )

    $basePath = switch ($Lane) {
        "e2e" { $script:e2ePath }
        "package" { $script:packagePath }
        "compatibility" { $script:compatibilityPath }
        "docs" { $script:docsPath }
        default { "" }
    }
    $path = if ([string]::IsNullOrWhiteSpace($basePath)) { "" } else { Join-Path $basePath $File }
    $text = Read-TextIfPresent $path
    $covered = (-not [string]::IsNullOrWhiteSpace($text)) -and ($text -match $Pattern)

    $Requirements.Add([pscustomobject]@{
        Lane = $Lane
        Requirement = $Requirement
        Status = if ($covered) { "covered" } else { "missing" }
        Evidence = if ([string]::IsNullOrWhiteSpace($path)) { "" } else { Get-RelativePath $path }
        Pattern = $Pattern
    })
}

function Add-DeferredRequirement {
    param(
        [System.Collections.Generic.List[object]]$Requirements,
        [string]$Lane,
        [string]$Requirement,
        [string]$Evidence,
        [string]$Reason
    )

    $Requirements.Add([pscustomobject]@{
        Lane = $Lane
        Requirement = $Requirement
        Status = "deferred-post-mvp"
        Evidence = $Evidence
        Pattern = $Reason
    })
}

function Add-Marker {
    param(
        [System.Collections.Generic.List[object]]$Markers,
        [string]$Source,
        [string]$Marker,
        [string]$Detail
    )
    $Markers.Add([pscustomobject]@{
        Source = $Source
        Marker = $Marker
        Detail = ConvertTo-SafeText $Detail
    })
}

function Scan-NonReleaseMarkers {
    param(
        [System.Collections.Generic.List[object]]$Markers,
        [string]$Name,
        [string]$Directory
    )

    if ([string]::IsNullOrWhiteSpace($Directory) -or -not (Test-Path -LiteralPath $Directory -PathType Container)) {
        Add-Marker $Markers $Name "missing-evidence-dir" "Evidence directory is missing or was not supplied."
        return
    }

    $textExtensions = @(".md", ".txt", ".json", ".toml", ".yaml", ".yml", ".log")
    $files = Get-ChildItem -LiteralPath $Directory -Recurse -File |
        Where-Object { $textExtensions -contains $_.Extension.ToLowerInvariant() }

    $rules = @(
        @{ Marker = "fixture"; Pattern = "(?is)\bfixture\b"; Detail = "Evidence mentions fixture data." },
        @{ Marker = "scaffold-only"; Pattern = "(?is)scaffold only|placeholder workspace"; Detail = "Evidence mentions scaffold-only state." },
        @{ Marker = "unfinished-placeholder"; Pattern = "(?is)\b(TODO|TBD|PLACEHOLDER|NOT_EXECUTED|FILL_ME|pending)\b"; Detail = "Evidence contains placeholder or pending wording." },
        @{ Marker = "not-release-evidence"; Pattern = "(?is)not release evidence|not full release evidence|not counted as release evidence"; Detail = "Evidence disclaims release readiness." }
    )

    foreach ($file in $files) {
        $text = Get-Content -Raw -LiteralPath $file.FullName
        foreach ($rule in $rules) {
            if ($text -match $rule.Pattern) {
                Add-Marker $Markers $Name $rule.Marker ("{0}: {1}" -f (Get-RelativePath $file.FullName), $rule.Detail)
            }
        }
    }
}

$script:e2ePath = Resolve-EvidencePath $E2EEvidenceDir
$script:packagePath = Resolve-EvidencePath $PackageEvidenceDir
$script:compatibilityPath = Resolve-EvidencePath $CompatibilityEvidenceDir
$script:docsPath = Resolve-EvidencePath $DocsEvidenceDir

$requirements = New-Object System.Collections.Generic.List[object]
$markers = New-Object System.Collections.Generic.List[object]

Add-Requirement $requirements "e2e" "test-account matrix covers all user states" "01-test-accounts.md" "(?is)admin_test.*user_active.*user_not_purchased.*user_expired.*user_low_balance.*user_device_revoked.*user_model_denied"
Add-Requirement $requirements "e2e" "browser handoff login contract and poll path" "04-client-api-e2e.md" "(?is)browser handoff.*desktop poll"
Add-Requirement $requirements "e2e" "bootstrap and device refresh evidence" "04-client-api-e2e.md" "(?is)bootstrap.*device"
Add-Requirement $requirements "e2e" "gateway active-user success" "05-gateway-policy-e2e.md" "(?is)user_active|active-user|active user"
Add-Requirement $requirements "e2e" "gateway no-entitlement rejection" "05-gateway-policy-e2e.md" "(?is)no entitlement|not_purchased|not purchased"
Add-Requirement $requirements "e2e" "gateway expired rejection" "05-gateway-policy-e2e.md" "(?is)expired"
Add-Requirement $requirements "e2e" "gateway insufficient-balance rejection" "05-gateway-policy-e2e.md" "(?is)insufficient|low balance"
Add-Requirement $requirements "e2e" "gateway revoked-device rejection" "05-gateway-policy-e2e.md" "(?is)revoked"
Add-Requirement $requirements "e2e" "gateway unauthorized-model rejection" "05-gateway-policy-e2e.md" "(?is)unauthorized model|model_denied|model denied"
Add-Requirement $requirements "e2e" "desktop Manager login/bootstrap/provider-write/Codex launch" "06-desktop-manager-e2e.md" "(?is)Manager login.*bootstrap.*Codex\+\+ Cloud.*Codex launch"
Add-Requirement $requirements "e2e" "manual providers survive managed provider write" "06-desktop-manager-e2e.md" "(?is)Manual providers?.*present|manual providers?.*remained"
Add-Requirement $requirements "e2e" "usage and rejection audit evidence" "09-usage-events-audit.md" "(?is)Usage.*admin audit.*success.*rejection.*gateway_policy_rejected.*GATEWAY_POLICY_.*request[_ ]?id.*(Gateway request[_ ]?id correlation\s*:\s*pass|Request ID correlation.*matched|matched gateway request[_ ]?id).*config[_ ]?version.*redaction_applied"
Add-Requirement $requirements "e2e" "rollback covers config/backend/desktop/entitlement/provider write" "10-rollback-notes.md" "(?is)Config rollback.*Backend rollback.*Desktop rollback.*Entitlement.*provider write"

Add-Requirement $requirements "package" "Windows fresh/overwrite/uninstall-reinstall" "01-windows-x64-install.md" "(?is)Fresh install.*Overwrite install.*Uninstall and reinstall"
if ($WindowsOnlyMvp) {
    Add-DeferredRequirement $requirements "package" "macOS x64 DMG mount/gatekeeper/reinstall" "post-MVP by Windows-only MVP scope" "Windows-only MVP scope defers macOS x64 package evidence."
    Add-DeferredRequirement $requirements "package" "macOS arm64 DMG mount/gatekeeper/reinstall" "post-MVP by Windows-only MVP scope" "Windows-only MVP scope defers macOS arm64 package evidence."
} else {
    Add-Requirement $requirements "package" "macOS x64 DMG mount/gatekeeper/reinstall" "02-macos-x64-dmg.md" "(?is)DMG mount.*Gatekeeper.*reinstall"
    Add-Requirement $requirements "package" "macOS arm64 DMG mount/gatekeeper/reinstall" "03-macos-arm64-dmg.md" "(?is)DMG mount.*Gatekeeper.*reinstall"
}
Add-Requirement $requirements "package" "package artifact inspection excludes shared credentials and fixed policy" "04-artifact-inspection.md" "(?is)does not embed a shared Key.*does not embed user credentials.*does not embed fixed price, plan, or model policy"
Add-Requirement $requirements "e2e" "missing Codex first-run assistant evidence" "07-package-install-check.md" "(?is)Missing Codex.*first-run|first-run.*Missing Codex"

Add-Requirement $requirements "compatibility" "pre-upgrade manual providers and redacted keys" "01-pre-upgrade-snapshot.md" "(?is)Manual provider.*API keys? redacted"
Add-Requirement $requirements "compatibility" "post-upgrade manual providers preserved and managed cloud present" "02-post-upgrade-cloud.md" "(?is)Manual providers preserved.*Codex\+\+ Cloud"
Add-Requirement $requirements "compatibility" "migration does not write local commercial policy" "02-post-upgrade-cloud.md" "(?is)No plan, price, multiplier, entitlement, or usage policy"
Add-Requirement $requirements "compatibility" "cloud logout leaves manual providers unchanged" "03-cloud-logout-boundary.md" "(?is)Cloud logout.*Manual providers remain unchanged"
Add-Requirement $requirements "compatibility" "manual provider switch remains usable" "04-manual-provider-switch.md" "(?is)Manual provider can still be selected.*Manual provider can still be used"
Add-Requirement $requirements "compatibility" "provider sync recognizes legacy profiles without corruption" "05-provider-sync.md" "(?is)Provider sync recognizes legacy profiles.*does not corrupt manual provider entries"
Add-Requirement $requirements "compatibility" "rollback rehearses config/desktop/backend-gateway recovery" "06-rollback-rehearsal.md" "(?is)Config rollback.*Desktop rollback.*Backend/gateway rollback"

Add-Requirement $requirements "docs" "docs sync is final and backend-configured" "00-docs-sync-record.md" "(?is)Report status:\s*final.*backend-configured.*Control Plane.*Data Plane.*Client Runtime.*Platform Ops"
Add-Requirement $requirements "docs" "user guide covers cloud flow and failure states" "01-user-guide.md" "(?is)Codex\+\+ Cloud.*not purchased.*expired.*insufficient balance.*device revoked.*model unavailable.*local configuration failure"
Add-Requirement $requirements "docs" "admin guide covers config rollback audit reconciliation" "02-admin-operations-guide.md" "(?is)configuration version.*canary.*rollback.*audit.*reconciliation"
Add-Requirement $requirements "docs" "release notes are final with rollback compatibility and risks" "03-release-notes.md" "(?is)Status:\s*final.*rollback.*compatibility.*known risks"
Add-Requirement $requirements "docs" "HTML sync has browser visual pass and residue scan" "04-html-sync-evidence.md" "(?is)Result:\s*pass.*static sync passed.*local Chromium visual evidence passed.*(in-app browser.*visual evidence.*passed|approved browser preview.*passed).*residue"
Add-Requirement $requirements "docs" "docs product copy gate report passes" "06-docs-product-copy-gate-report.md" "(?im)^\s*Docs product copy result\s*:\s*pass\s*$"

Scan-NonReleaseMarkers $markers "e2e" $e2ePath
Scan-NonReleaseMarkers $markers "package" $packagePath
Scan-NonReleaseMarkers $markers "compatibility" $compatibilityPath
Scan-NonReleaseMarkers $markers "docs" $docsPath

$missing = @($requirements | Where-Object { $_.Status -eq "missing" })
$complete = ($missing.Count -eq 0 -and $markers.Count -eq 0)
$coverageStatus = if ($complete) { "complete" } else { "incomplete" }
$generatedAt = Get-Date -Format "yyyy-MM-dd HH:mm:ssK"

$coverageLines = $requirements | ForEach-Object {
    "| $($_.Lane) | $($_.Requirement) | $($_.Status) | $($_.Evidence) |"
}
$markerLines = if ($markers.Count -gt 0) {
    $markers | ForEach-Object {
        "- source: $($_.Source); marker: $($_.Marker); detail: $($_.Detail)"
    }
} else {
    @("- none")
}

$summary = @(
    "# 07 Release Coverage Summary",
    "",
    "Report status: generated",
    "Generated at: $generatedAt",
    "Coverage status: $coverageStatus",
    "MVP package scope: $(if ($WindowsOnlyMvp) { "windows-only" } else { "cross-platform" })",
    "Missing coverage count: $($missing.Count)",
    "Nonrelease marker count: $($markers.Count)",
    "",
    "## Evidence Inputs",
    "",
    "- E2E evidence folder: $(ConvertTo-SafeText (Get-RelativePath $e2ePath))",
    "- Package evidence folder: $(ConvertTo-SafeText (Get-RelativePath $packagePath))",
    "- Compatibility evidence folder: $(ConvertTo-SafeText (Get-RelativePath $compatibilityPath))",
    "- Docs product copy evidence folder: $(ConvertTo-SafeText (Get-RelativePath $docsPath))",
    "",
    "## Coverage Matrix",
    "",
    "| Lane | Requirement | Status | Evidence |",
    "| --- | --- | --- | --- |",
    ($coverageLines -join [Environment]::NewLine),
    "",
    "## Nonrelease Markers",
    "",
    ($markerLines -join [Environment]::NewLine),
    "",
    "## Release Boundary",
    "",
    "This coverage summary maps evidence files to the required 07 release scenarios. It does not execute E2E, build packages, run compatibility migration, or make the release go/no-go decision."
) -join [Environment]::NewLine

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputFile) | Out-Null
Set-Content -LiteralPath $OutputFile -Encoding UTF8 -Value $summary

Write-Host "07 release coverage summary: $OutputFile"
Write-Host "Coverage status: $coverageStatus"
Write-Host "Missing coverage count: $($missing.Count)"
Write-Host "Nonrelease markers: $($markers.Count)"

if ($FailOnIncomplete -and -not $complete) {
    exit 1
}

exit 0
