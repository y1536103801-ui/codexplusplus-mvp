param(
    [string]$Root,
    [string]$E2EEvidenceDir,
    [string]$PackageEvidenceDir,
    [string]$CompatibilityEvidenceDir,
    [string]$DocsEvidenceDir,
    [string]$BusinessEvidenceDir,
    [string]$BusinessSourceDocsRoot,
    [string]$CoverageSummaryFile,
    [string]$OutputFile,
    [string]$LogRoot,
    [switch]$WindowsOnlyMvp,
    [switch]$AllowGoCandidate,
    [switch]$FailOnNoGo
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($OutputFile)) {
    $OutputFile = Join-Path $Root "codex-plus-dev-plan\07-integration-release\reports\release-readiness-summary.md"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputFile)) {
    $OutputFile = Join-Path $Root $OutputFile
}

if ([string]::IsNullOrWhiteSpace($LogRoot)) {
    $LogRoot = Join-Path $env:TEMP ("codexplus-07-readiness-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
} elseif (-not [System.IO.Path]::IsPathRooted($LogRoot)) {
    $LogRoot = Join-Path $Root $LogRoot
}

if (-not [string]::IsNullOrWhiteSpace($CoverageSummaryFile) -and -not [System.IO.Path]::IsPathRooted($CoverageSummaryFile)) {
    $CoverageSummaryFile = Join-Path $Root $CoverageSummaryFile
}

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputFile) | Out-Null
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

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

function ConvertTo-ComparablePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    return ($Path.Trim().Trim('"', "'") -replace "/", "\").TrimEnd('\')
}

function Resolve-ReferencePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    $trimmed = $Path.Trim().Trim('"', "'")
    if ([System.IO.Path]::IsPathRooted($trimmed)) {
        return [System.IO.Path]::GetFullPath($trimmed)
    }
    return [System.IO.Path]::GetFullPath((Join-Path $Root $trimmed))
}

function Test-PathReference {
    param(
        [string]$Actual,
        [string]$Expected
    )
    if ([string]::IsNullOrWhiteSpace($Actual) -or [string]::IsNullOrWhiteSpace($Expected)) {
        return $false
    }

    $actualComparable = ConvertTo-ComparablePath $Actual
    $expectedResolved = Resolve-ReferencePath $Expected
    $expectedFull = ConvertTo-ComparablePath $expectedResolved
    $expectedRelative = ConvertTo-ComparablePath (Get-RelativePath $expectedResolved)

    if ($actualComparable.Equals($expectedFull, [System.StringComparison]::OrdinalIgnoreCase) -or
        $actualComparable.Equals($expectedRelative, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $true
    }

    try {
        $actualFull = Resolve-ReferencePath $actualComparable
        return (ConvertTo-ComparablePath $actualFull).Equals($expectedFull, [System.StringComparison]::OrdinalIgnoreCase)
    } catch {
        return $false
    }
}

function ConvertTo-SafeEvidenceText {
    param([string]$Text)
    if ($null -eq $Text) {
        return ""
    }
    $safe = $Text
    $safe = $safe -replace "(?i)\bsk-proj-[A-Za-z0-9_-]{8,}\b", "[redacted-openai-project-key]"
    $safe = $safe -replace "(?i)\bsk-ant-[A-Za-z0-9_-]{8,}\b", "[redacted-anthropic-key]"
    $safe = $safe -replace "(?i)\bsk-[A-Za-z0-9_-]{8,}\b", "[redacted-api-key]"
    $safe = $safe -replace "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b", "[redacted-jwt]"
    $safe = $safe -replace "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}", "Authorization: [redacted]"
    return $safe
}

function Read-TextIfPresent {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return ""
    }
    return Get-Content -Raw -LiteralPath $Path
}

function Get-RegexValue {
    param(
        [string]$Text,
        [string]$Pattern,
        [string]$Fallback = "not recorded"
    )
    $match = [regex]::Match($Text, $Pattern, [System.Text.RegularExpressions.RegexOptions]::IgnoreCase -bor [System.Text.RegularExpressions.RegexOptions]::Multiline)
    if ($match.Success) {
        return ($match.Groups[1].Value.Trim())
    }
    return $Fallback
}

function Get-StrictRegexValue {
    param(
        [string]$Text,
        [string]$Pattern
    )
    $match = [regex]::Match($Text, $Pattern, [System.Text.RegularExpressions.RegexOptions]::IgnoreCase -bor [System.Text.RegularExpressions.RegexOptions]::Multiline)
    if ($match.Success) {
        return ($match.Groups[1].Value.Trim())
    }
    return ""
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
        Detail = ConvertTo-SafeEvidenceText $Detail
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
        @{ Marker = "fixture"; Pattern = "(?i)\bfixture\b"; Detail = "Evidence mentions fixture data." },
        @{ Marker = "scaffold-only"; Pattern = "(?i)scaffold only|placeholder workspace"; Detail = "Evidence mentions scaffold-only state." },
        @{ Marker = "subset-only"; Pattern = "(?i)subset only|selected API/gateway subsets|client API subset"; Detail = "Evidence mentions partial subset execution." },
        @{ Marker = "not-release-evidence"; Pattern = "(?i)not release evidence|not full release evidence|not counted as release evidence"; Detail = "Evidence disclaims release readiness." },
        @{ Marker = "runtime-evidence-required"; Pattern = "(?i)runtime evidence remains required|real .* evidence remains required|platform install.*required|desktop launch.*required"; Detail = "Evidence says external/runtime proof is still required." },
        @{ Marker = "no-go-or-pending"; Pattern = "(?i)release recommendation no-go|evidence pending|compatibility evidence pending|package evidence pending|E2E evidence pending"; Detail = "Evidence records no-go or pending release evidence." }
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

$e2ePath = Resolve-EvidencePath $E2EEvidenceDir
$packagePath = Resolve-EvidencePath $PackageEvidenceDir
$compatibilityPath = Resolve-EvidencePath $CompatibilityEvidenceDir
$docsPath = Resolve-EvidencePath $DocsEvidenceDir
$businessPath = Resolve-EvidencePath $BusinessEvidenceDir

$aggregateLog = Join-Path $LogRoot "aggregate-release-evidence.log"
$aggregateScript = Join-Path $Root "codex-plus-dev-plan\tools\verify-07-release-evidence.ps1"
$aggregateArgs = @(
    "-NoProfile",
    "-ExecutionPolicy", "Bypass",
    "-File", $aggregateScript,
    "-Root", $Root,
    "-E2EEvidenceDir", $e2ePath,
    "-PackageEvidenceDir", $packagePath,
    "-CompatibilityEvidenceDir", $compatibilityPath,
    "-DocsEvidenceDir", $docsPath,
    "-LogRoot", (Join-Path $LogRoot "aggregate-verifier-logs")
)
if ($WindowsOnlyMvp) {
    $aggregateArgs += "-WindowsOnlyMvp"
}

& powershell @aggregateArgs *> $aggregateLog
$aggregateExit = $LASTEXITCODE
$aggregatePassed = ($aggregateExit -eq 0)

$businessLog = Join-Path $LogRoot "business-readiness.log"
$businessPassed = $false
$businessExit = -1
if (-not [string]::IsNullOrWhiteSpace($businessPath) -and (Test-Path -LiteralPath $businessPath -PathType Container)) {
    $businessScript = Join-Path $Root "codex-plus-dev-plan\tools\verify-07-business-readiness.ps1"
    $businessArgs = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", $businessScript,
        "-Root", $Root,
        "-EvidenceDir", $businessPath
    )
    if (-not [string]::IsNullOrWhiteSpace($BusinessSourceDocsRoot)) {
        $businessArgs += @("-SourceDocsRoot", $BusinessSourceDocsRoot)
    }
    & powershell @businessArgs *> $businessLog
    $businessExit = $LASTEXITCODE
    $businessPassed = ($businessExit -eq 0)
} else {
    Set-Content -LiteralPath $businessLog -Encoding UTF8 -Value "Business readiness evidence directory missing or not supplied."
}

$docsLog = Join-Path $LogRoot "docs-product-copy.log"
$docsPassed = $false
$docsExit = -1
if (-not [string]::IsNullOrWhiteSpace($docsPath) -and (Test-Path -LiteralPath $docsPath -PathType Container)) {
    $docsScript = Join-Path $Root "codex-plus-dev-plan\tools\verify-07-docs-product-copy-evidence.ps1"
    $docsArgs = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", $docsScript,
        "-Root", $Root,
        "-EvidenceDir", $docsPath
    )
    & powershell @docsArgs *> $docsLog
    $docsExit = $LASTEXITCODE
    $docsPassed = ($docsExit -eq 0)
} else {
    Set-Content -LiteralPath $docsLog -Encoding UTF8 -Value "Docs product copy evidence directory missing or not supplied."
}

$e2eReport = Read-TextIfPresent (Join-Path $e2ePath "12-release-gate-report.md")
$packageReport = Read-TextIfPresent (Join-Path $packagePath "05-package-gate-report.md")
$compatibilityReport = Read-TextIfPresent (Join-Path $compatibilityPath "07-compatibility-gate-report.md")
$docsReport = Read-TextIfPresent (Join-Path $docsPath "06-docs-product-copy-gate-report.md")
$businessReport = Read-TextIfPresent (Join-Path $businessPath "11-business-readiness.md")
$coverageText = Read-TextIfPresent $CoverageSummaryFile

$e2eRecommendation = Get-RegexValue $e2eReport "^\s*Final recommendation\s*:\s*(.+?)\s*$"
$packageResult = Get-RegexValue $packageReport "^\s*Package evidence result\s*:\s*(.+?)\s*$"
$compatibilityResult = Get-RegexValue $compatibilityReport "^\s*Compatibility evidence result\s*:\s*(.+?)\s*$"
$docsResult = Get-RegexValue $docsReport "^\s*Docs product copy result\s*:\s*(.+?)\s*$"
$businessResult = Get-RegexValue $businessReport "^\s*Business readiness result\s*:\s*(.+?)\s*$"
$e2eLevel3Result = Get-RegexValue $e2eReport "^\s*Level 3 result\s*:\s*(.+?)\s*$"
$coverageStatus = Get-StrictRegexValue $coverageText "^\s*Coverage status\s*:\s*(complete|incomplete)\s*$"
$coverageMvpPackageScope = Get-StrictRegexValue $coverageText "^\s*MVP package scope\s*:\s*(windows-only|cross-platform)\s*$"
$coverageMissingCountText = Get-StrictRegexValue $coverageText "^\s*Missing coverage count\s*:\s*(\d+)\s*$"
$coverageMarkerCountText = Get-StrictRegexValue $coverageText "^\s*Nonrelease marker count\s*:\s*(\d+)\s*$"
$coverageE2EInput = Get-StrictRegexValue $coverageText "^\s*-\s*E2E evidence folder\s*:\s*(.+?)\s*$"
$coveragePackageInput = Get-StrictRegexValue $coverageText "^\s*-\s*Package evidence folder\s*:\s*(.+?)\s*$"
$coverageCompatibilityInput = Get-StrictRegexValue $coverageText "^\s*-\s*Compatibility evidence folder\s*:\s*(.+?)\s*$"
$coverageDocsInput = Get-StrictRegexValue $coverageText "^\s*-\s*Docs product copy evidence folder\s*:\s*(.+?)\s*$"
$coverageMissingCount = if ([string]::IsNullOrWhiteSpace($coverageMissingCountText)) { -1 } else { [int]$coverageMissingCountText }
$coverageMarkerCount = if ([string]::IsNullOrWhiteSpace($coverageMarkerCountText)) { -1 } else { [int]$coverageMarkerCountText }
$coverageE2EInputMatches = Test-PathReference $coverageE2EInput $e2ePath
$coveragePackageInputMatches = Test-PathReference $coveragePackageInput $packagePath
$coverageCompatibilityInputMatches = Test-PathReference $coverageCompatibilityInput $compatibilityPath
$coverageDocsInputMatches = Test-PathReference $coverageDocsInput $docsPath
$expectedMvpPackageScope = if ($WindowsOnlyMvp) { "windows-only" } else { "cross-platform" }
$coverageMvpPackageScopeMatches = ($coverageMvpPackageScope -eq $expectedMvpPackageScope)
$coveragePassed = (
    -not [string]::IsNullOrWhiteSpace($CoverageSummaryFile) -and
    (Test-Path -LiteralPath $CoverageSummaryFile -PathType Leaf) -and
    $coverageStatus -eq "complete" -and
    $coverageMvpPackageScopeMatches -and
    $coverageMissingCount -eq 0 -and
    $coverageMarkerCount -eq 0 -and
    $coverageE2EInputMatches -and
    $coveragePackageInputMatches -and
    $coverageCompatibilityInputMatches -and
    $coverageDocsInputMatches
)
$coverageVerification = if ($coveragePassed) { "passed" } else { "failed" }

$markers = New-Object System.Collections.Generic.List[object]
Scan-NonReleaseMarkers $markers "e2e" $e2ePath
Scan-NonReleaseMarkers $markers "package" $packagePath
Scan-NonReleaseMarkers $markers "compatibility" $compatibilityPath
Scan-NonReleaseMarkers $markers "docs" $docsPath
Scan-NonReleaseMarkers $markers "business" $businessPath

if ([string]::IsNullOrWhiteSpace($CoverageSummaryFile) -or -not (Test-Path -LiteralPath $CoverageSummaryFile -PathType Leaf)) {
    Add-Marker $markers "coverage" "coverage-summary-missing" "Release coverage summary was missing or not supplied."
} else {
    if ($coverageStatus -ne "complete") {
        Add-Marker $markers "coverage" "coverage-incomplete" "Coverage summary status is not complete."
    }
    if (-not $coverageMvpPackageScopeMatches) {
        Add-Marker $markers "coverage" "coverage-mvp-package-scope-mismatch" "Coverage summary MVP package scope does not match readiness input."
    }
    if ($coverageMissingCount -ne 0) {
        Add-Marker $markers "coverage" "coverage-missing-requirements" "Coverage summary missing coverage count is not zero."
    }
    if ($coverageMarkerCount -ne 0) {
        Add-Marker $markers "coverage" "coverage-nonrelease-markers" "Coverage summary nonrelease marker count is not zero."
    }
    if (-not $coverageE2EInputMatches) {
        Add-Marker $markers "coverage" "coverage-e2e-input-mismatch" "Coverage summary E2E evidence folder does not match readiness input."
    }
    if (-not $coveragePackageInputMatches) {
        Add-Marker $markers "coverage" "coverage-package-input-mismatch" "Coverage summary package evidence folder does not match readiness input."
    }
    if (-not $coverageCompatibilityInputMatches) {
        Add-Marker $markers "coverage" "coverage-compatibility-input-mismatch" "Coverage summary compatibility evidence folder does not match readiness input."
    }
    if (-not $coverageDocsInputMatches) {
        Add-Marker $markers "coverage" "coverage-docs-input-mismatch" "Coverage summary docs product copy evidence folder does not match readiness input."
    }
}
if (-not $aggregatePassed) {
    Add-Marker $markers "aggregate" "aggregate-verifier-failed" "verify-07-release-evidence.ps1 failed; see aggregate log."
}
if (-not $businessPassed) {
    Add-Marker $markers "business" "business-readiness-verifier-failed" "verify-07-business-readiness.ps1 failed or business readiness evidence was missing; see business readiness log."
}
if (-not $docsPassed) {
    Add-Marker $markers "docs" "docs-product-copy-verifier-failed" "verify-07-docs-product-copy-evidence.ps1 failed or docs product copy evidence was missing; see docs product copy log."
}
if ($e2eLevel3Result -notmatch "^(?i:pass)$") {
    Add-Marker $markers "e2e" "level3-pass-missing" "E2E release gate report did not record Level 3 result: pass."
}
if (-not $AllowGoCandidate) {
    Add-Marker $markers "module-j-boundary" "go-candidate-not-authorized" "Use -AllowGoCandidate only for a Module J candidate review after real external evidence exists."
}

$noGo = ($markers.Count -gt 0)
$posture = if ($noGo) { "no-go" } else { "go-candidate-requires-module-j-review" }
$aggregateText = if ($aggregatePassed) { "passed" } else { "failed" }
$businessText = if ($businessPassed) { "passed" } else { "failed" }
$docsText = if ($docsPassed) { "passed" } else { "failed" }
$allowText = if ($AllowGoCandidate) { "true" } else { "false" }
$generatedAt = Get-Date -Format "yyyy-MM-dd HH:mm:ssK"

$markerLines = if ($markers.Count -gt 0) {
    ($markers | ForEach-Object {
        "- source: $($_.Source); marker: $($_.Marker); detail: $($_.Detail)"
    }) -join [Environment]::NewLine
} else {
    "- none"
}

$summary = @(
    "# 07 Release Readiness Summary",
    "",
    "Report status: generated",
    "Generated at: $generatedAt",
    "Aggregate evidence result: $aggregateText",
    "Coverage summary verification: $coverageVerification",
    "Coverage status: $(if ([string]::IsNullOrWhiteSpace($coverageStatus)) { "not recorded" } else { $coverageStatus })",
    "MVP package scope: $(if ([string]::IsNullOrWhiteSpace($coverageMvpPackageScope)) { "not recorded" } else { $coverageMvpPackageScope })",
    "Coverage missing count: $(if ($coverageMissingCount -lt 0) { "not recorded" } else { $coverageMissingCount })",
    "Coverage nonrelease marker count: $(if ($coverageMarkerCount -lt 0) { "not recorded" } else { $coverageMarkerCount })",
    "Docs product copy verification: $docsText",
    "Business readiness verification: $businessText",
    "Recommended Module J posture: $posture",
    "Allow go candidate: $allowText",
    "",
    "## Evidence Inputs",
    "",
    "- E2E evidence folder: $(ConvertTo-SafeEvidenceText (Get-RelativePath $e2ePath))",
    "- Package evidence folder: $(ConvertTo-SafeEvidenceText (Get-RelativePath $packagePath))",
    "- Compatibility evidence folder: $(ConvertTo-SafeEvidenceText (Get-RelativePath $compatibilityPath))",
    "- Docs product copy evidence folder: $(ConvertTo-SafeEvidenceText (Get-RelativePath $docsPath))",
    "- Business readiness folder: $(ConvertTo-SafeEvidenceText (Get-RelativePath $businessPath))",
    "- MVP package scope: $expectedMvpPackageScope",
    "- Release coverage summary: $(ConvertTo-SafeEvidenceText (Get-RelativePath $CoverageSummaryFile))",
    "- Aggregate verifier log: $(ConvertTo-SafeEvidenceText (Get-RelativePath $aggregateLog))",
    "- Docs product copy verifier log: $(ConvertTo-SafeEvidenceText (Get-RelativePath $docsLog))",
    "- Business readiness verifier log: $(ConvertTo-SafeEvidenceText (Get-RelativePath $businessLog))",
    "",
    "## Evidence Signals",
    "",
    "- E2E final recommendation: $(ConvertTo-SafeEvidenceText $e2eRecommendation)",
    "- E2E Level 3 result: $(ConvertTo-SafeEvidenceText $e2eLevel3Result)",
    "- Package evidence result: $(ConvertTo-SafeEvidenceText $packageResult)",
    "- Compatibility evidence result: $(ConvertTo-SafeEvidenceText $compatibilityResult)",
    "- Docs product copy result: $(ConvertTo-SafeEvidenceText $docsResult)",
    "- Coverage summary verification: $coverageVerification",
    "- Business readiness result: $(ConvertTo-SafeEvidenceText $businessResult)",
    "",
    "## Nonrelease Markers",
    "",
    $markerLines,
    "",
    "## Release Boundary",
    "",
    "This generated summary is not the final Module J report. It only combines verifier output and obvious nonrelease markers so Module J cannot accidentally treat fixture, scaffold, subset, pending, or missing external evidence as a release go decision.",
    "The final Module J report must still pass tools/verify-07-module-j-report.ps1 with generated coverage and readiness summaries, and must be based on real production-equivalent E2E, package, compatibility and business readiness evidence."
) -join [Environment]::NewLine

Set-Content -LiteralPath $OutputFile -Encoding UTF8 -Value $summary

Write-Host "07 release readiness summary: $OutputFile"
Write-Host "Aggregate evidence result: $aggregateText"
Write-Host "Coverage summary verification: $coverageVerification"
Write-Host "Docs product copy verification: $docsText"
Write-Host "Business readiness verification: $businessText"
Write-Host "Recommended Module J posture: $posture"
Write-Host "Nonrelease markers: $($markers.Count)"

if ($FailOnNoGo -and $noGo) {
    exit 1
}

exit 0
