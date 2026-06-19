param(
    [string]$Root,
    [string]$ReleaseDir,
    [string]$LogRoot,
    [string]$BusinessSourceDocsRoot,
    [switch]$WindowsOnlyMvp,
    [switch]$AllowGoCandidate
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($ReleaseDir)) {
    throw "ReleaseDir is required. Example: -ReleaseDir codex-plus-dev-plan/test-runs/20260618-1911-release"
}

if ([System.IO.Path]::IsPathRooted($ReleaseDir)) {
    $ReleasePath = [System.IO.Path]::GetFullPath($ReleaseDir)
} else {
    $ReleasePath = [System.IO.Path]::GetFullPath((Join-Path $Root $ReleaseDir))
}

if ([string]::IsNullOrWhiteSpace($LogRoot)) {
    $LogRoot = Join-Path $env:TEMP ("codexplus-07-release-handoff-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
} elseif (-not [System.IO.Path]::IsPathRooted($LogRoot)) {
    $LogRoot = Join-Path $Root $LogRoot
}
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

$results = New-Object System.Collections.Generic.List[object]

function Add-Check {
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

function Get-FieldValue {
    param(
        [string]$Text,
        [string]$Field
    )
    if ([string]::IsNullOrWhiteSpace($Text)) {
        return ""
    }
    $match = [regex]::Match($Text, ("(?im)^\s*(?:-\s*)?" + [regex]::Escape($Field) + "\s*:\s*(?<value>\S.*?)\s*$"))
    if ($match.Success) {
        return $match.Groups["value"].Value.Trim()
    }
    return ""
}

function Get-MarkdownSectionText {
    param(
        [string]$Text,
        [string]$Title
    )
    if ([string]::IsNullOrWhiteSpace($Text)) {
        return ""
    }
    $escapedTitle = [regex]::Escape($Title)
    $match = [regex]::Match($Text, "(?ims)^##\s+$escapedTitle\s*\r?\n(?<body>.*?)(?=^##\s+|\z)")
    if ($match.Success) {
        return $match.Groups["body"].Value
    }
    return ""
}

function Get-NonreleaseMarkerCount {
    param([string]$Text)
    $section = Get-MarkdownSectionText $Text "Nonrelease Markers"
    if ([string]::IsNullOrWhiteSpace($section)) {
        return -1
    }
    $markerLines = @(
        $section -split "\r?\n" |
            ForEach-Object { $_.Trim() } |
            Where-Object { $_ -match "^\-\s+" -and $_ -notmatch "^\-\s*none\s*$" }
    )
    return $markerLines.Count
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
        [string]$Fallback = ""
    )
    $match = [regex]::Match($Text, $Pattern, [System.Text.RegularExpressions.RegexOptions]::IgnoreCase -bor [System.Text.RegularExpressions.RegexOptions]::Multiline)
    if ($match.Success) {
        return $match.Groups[1].Value.Trim()
    }
    return $Fallback
}

function Invoke-Tool {
    param(
        [string]$Name,
        [string]$ScriptRelativePath,
        [string[]]$Arguments,
        [string]$LogName
    )

    $scriptPath = Join-Path $Root $ScriptRelativePath
    $scriptExists = Test-Path -LiteralPath $scriptPath -PathType Leaf
    Add-Check "${Name}:script-exists" $scriptExists $ScriptRelativePath
    if (-not $scriptExists) {
        return $false
    }

    $logPath = Join-Path $LogRoot $LogName
    & powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath @Arguments *> $logPath
    $exitCode = $LASTEXITCODE
    Add-Check "${Name}:passed" ($exitCode -eq 0) "exit=$exitCode; log=$(Get-RelativePath $logPath)"
    return ($exitCode -eq 0)
}

Add-Check "release-dir-exists" (Test-Path -LiteralPath $ReleasePath -PathType Container) (Get-RelativePath $ReleasePath)

$indexPath = Join-Path $ReleasePath "00-release-evidence-index.md"
$storedCoveragePath = Join-Path $ReleasePath "release-coverage-summary.md"
$storedSummaryPath = Join-Path $ReleasePath "release-readiness-summary.md"
$finalReportPath = Join-Path $ReleasePath "module-j-final-report.md"
$draftReportPath = Join-Path $ReleasePath "module-j-final-report-draft.md"

Add-Check "handoff-index:exists" (Test-Path -LiteralPath $indexPath -PathType Leaf) (Get-RelativePath $indexPath)
Add-Check "coverage-summary:exists" (Test-Path -LiteralPath $storedCoveragePath -PathType Leaf) (Get-RelativePath $storedCoveragePath)
Add-Check "readiness-summary:exists" (Test-Path -LiteralPath $storedSummaryPath -PathType Leaf) (Get-RelativePath $storedSummaryPath)
Add-Check "module-j-final-report:exists" (Test-Path -LiteralPath $finalReportPath -PathType Leaf) (Get-RelativePath $finalReportPath)
Add-Check "module-j-final-report:not-draft-only" (Test-Path -LiteralPath $finalReportPath -PathType Leaf) "Final handoff must use module-j-final-report.md, not only $(Get-RelativePath $draftReportPath)."

$indexText = Read-TextIfPresent $indexPath
$releaseLeaf = Split-Path -Leaf $ReleasePath
$releaseRunStamp = Get-RegexValue $releaseLeaf "^(\d{8}-\d{4})-release$"
$indexRunStamp = Get-RegexValue $indexText "^\s*Run stamp\s*:\s*(\d{8}-\d{4})\s*$"
$runStamp = if (-not [string]::IsNullOrWhiteSpace($indexRunStamp)) { $indexRunStamp } else { $releaseRunStamp }

Add-Check "release-dir:run-stamp" (-not [string]::IsNullOrWhiteSpace($releaseRunStamp)) "Release directory must be named YYYYMMDD-HHMM-release; got $releaseLeaf."
Add-Check "handoff-index:run-stamp" (-not [string]::IsNullOrWhiteSpace($indexRunStamp)) "Run stamp: $indexRunStamp"
if (-not [string]::IsNullOrWhiteSpace($indexRunStamp) -and -not [string]::IsNullOrWhiteSpace($releaseRunStamp)) {
    Add-Check "handoff-index:run-stamp-release-dir-consistency" ($indexRunStamp -eq $releaseRunStamp) "index=$indexRunStamp; release-dir=$releaseRunStamp"
}
Add-Check "handoff-index:final-status" ($indexText -match "(?im)^\s*Status\s*:\s*final\s*$") "Release handoff index must be marked final."
Add-Check "handoff-index:not-scaffold-only" ($indexText -notmatch "(?i)scaffold only|placeholder workspace|Result:\s*pending|TODO|FILL_ME") "Release handoff index must not retain scaffold placeholders."

$indexAggregateResult = Get-RegexValue $indexText "^\s*-\s*aggregate verifier result\s*:\s*(passed|failed)\s*$"
$indexDocsResult = Get-RegexValue $indexText "^\s*-\s*docs product copy verification\s*:\s*(passed|failed)\s*$"
$indexBusinessResult = Get-RegexValue $indexText "^\s*-\s*business readiness verification\s*:\s*(passed|failed)\s*$"
$indexCoverageStatus = Get-RegexValue $indexText "^\s*-\s*coverage summary status\s*:\s*(complete|incomplete)\s*$"
$indexReadinessPosture = Get-RegexValue $indexText "^\s*-\s*readiness summary posture\s*:\s*(no-go|go-candidate-requires-module-j-review)\s*$"
$indexReportResult = Get-RegexValue $indexText "^\s*-\s*Module J report verification\s*:\s*(passed|failed)\s*$"
$indexFinalRecommendation = Get-RegexValue $indexText "^\s*-\s*Final recommendation\s*:\s*(go|go with accepted risks|no-go)\s*$"

Add-Check "handoff-index:aggregate-result-field" (-not [string]::IsNullOrWhiteSpace($indexAggregateResult)) "Final index must record aggregate verifier result."
Add-Check "handoff-index:docs-result-field" (-not [string]::IsNullOrWhiteSpace($indexDocsResult)) "Final index must record docs product copy verification."
Add-Check "handoff-index:business-result-field" (-not [string]::IsNullOrWhiteSpace($indexBusinessResult)) "Final index must record business readiness verification."
Add-Check "handoff-index:coverage-status-field" (-not [string]::IsNullOrWhiteSpace($indexCoverageStatus)) "Final index must record coverage summary status."
Add-Check "handoff-index:readiness-posture-field" (-not [string]::IsNullOrWhiteSpace($indexReadinessPosture)) "Final index must record readiness summary posture."
Add-Check "handoff-index:module-j-report-result-field" (-not [string]::IsNullOrWhiteSpace($indexReportResult)) "Final index must record Module J report verification result."
Add-Check "handoff-index:final-recommendation-field" (-not [string]::IsNullOrWhiteSpace($indexFinalRecommendation)) "Final index must record Module J final recommendation."
if (-not [string]::IsNullOrWhiteSpace($indexAggregateResult)) {
    Add-Check "handoff-index:aggregate-passed" ($indexAggregateResult -eq "passed") "index=$indexAggregateResult"
}
if (-not [string]::IsNullOrWhiteSpace($indexDocsResult)) {
    Add-Check "handoff-index:docs-passed" ($indexDocsResult -eq "passed") "index=$indexDocsResult"
}
if (-not [string]::IsNullOrWhiteSpace($indexBusinessResult)) {
    Add-Check "handoff-index:business-passed" ($indexBusinessResult -eq "passed") "index=$indexBusinessResult"
}
if (-not [string]::IsNullOrWhiteSpace($indexCoverageStatus)) {
    Add-Check "handoff-index:coverage-complete" ($indexCoverageStatus -eq "complete") "index=$indexCoverageStatus"
}
if (-not [string]::IsNullOrWhiteSpace($indexReportResult)) {
    Add-Check "handoff-index:module-j-report-passed" ($indexReportResult -eq "passed") "index=$indexReportResult"
}

if (-not [string]::IsNullOrWhiteSpace($runStamp)) {
    $outputRoot = Split-Path -Parent $ReleasePath
    $e2ePath = Join-Path $outputRoot "$runStamp-e2e"
    $packagePath = Join-Path $outputRoot "$runStamp-package"
    $compatibilityPath = Join-Path $outputRoot "$runStamp-compatibility"
    $docsPath = Join-Path $outputRoot "$runStamp-docs"
    $businessPath = Join-Path $outputRoot "$runStamp-business"

    Add-Check "e2e:evidence-dir-exists" (Test-Path -LiteralPath $e2ePath -PathType Container) (Get-RelativePath $e2ePath)
    Add-Check "package:evidence-dir-exists" (Test-Path -LiteralPath $packagePath -PathType Container) (Get-RelativePath $packagePath)
    Add-Check "compatibility:evidence-dir-exists" (Test-Path -LiteralPath $compatibilityPath -PathType Container) (Get-RelativePath $compatibilityPath)
    Add-Check "docs:evidence-dir-exists" (Test-Path -LiteralPath $docsPath -PathType Container) (Get-RelativePath $docsPath)
    Add-Check "business:evidence-dir-exists" (Test-Path -LiteralPath $businessPath -PathType Container) (Get-RelativePath $businessPath)

    foreach ($entry in @(
        @{ Name = "handoff-index:e2e-path"; Path = $e2ePath; Label = "E2E evidence" },
        @{ Name = "handoff-index:package-path"; Path = $packagePath; Label = "Package evidence" },
        @{ Name = "handoff-index:compatibility-path"; Path = $compatibilityPath; Label = "Compatibility evidence" },
        @{ Name = "handoff-index:docs-path"; Path = $docsPath; Label = "Docs product copy evidence" },
        @{ Name = "handoff-index:business-path"; Path = $businessPath; Label = "Business readiness evidence" },
        @{ Name = "handoff-index:coverage-summary-path"; Path = $storedCoveragePath; Label = "Release coverage summary" },
        @{ Name = "handoff-index:readiness-summary-path"; Path = $storedSummaryPath; Label = "Release readiness summary" },
        @{ Name = "handoff-index:final-report-path"; Path = $finalReportPath; Label = "Module J final report" }
    )) {
        $indexPathValue = Get-FieldValue $indexText $entry.Label
        Add-Check "$($entry.Name)-field" (-not [string]::IsNullOrWhiteSpace($indexPathValue)) "$($entry.Label) must be recorded as a field in the handoff index."
        Add-Check $entry.Name (Test-PathReference $indexPathValue $entry.Path) "$($entry.Label): stored=$indexPathValue; expected=$(Get-RelativePath $entry.Path)"
    }

    $aggregateArgs = @(
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
    Invoke-Tool "aggregate-release-evidence" "codex-plus-dev-plan\tools\verify-07-release-evidence.ps1" $aggregateArgs "aggregate-release-evidence.log" | Out-Null

    Invoke-Tool "docs-product-copy" "codex-plus-dev-plan\tools\verify-07-docs-product-copy-evidence.ps1" @(
        "-Root", $Root,
        "-EvidenceDir", $docsPath
    ) "docs-product-copy.log" | Out-Null

    $businessArgs = @(
        "-Root", $Root,
        "-EvidenceDir", $businessPath
    )
    if (-not [string]::IsNullOrWhiteSpace($BusinessSourceDocsRoot)) {
        $businessArgs += @("-SourceDocsRoot", $BusinessSourceDocsRoot)
    }
    Invoke-Tool "business-readiness" "codex-plus-dev-plan\tools\verify-07-business-readiness.ps1" $businessArgs "business-readiness.log" | Out-Null

    $regeneratedCoveragePath = Join-Path $LogRoot "regenerated-release-coverage-summary.md"
    $coverageArgs = @(
        "-Root", $Root,
        "-E2EEvidenceDir", $e2ePath,
        "-PackageEvidenceDir", $packagePath,
        "-CompatibilityEvidenceDir", $compatibilityPath,
        "-DocsEvidenceDir", $docsPath,
        "-OutputFile", $regeneratedCoveragePath,
        "-FailOnIncomplete"
    )
    if ($WindowsOnlyMvp) {
        $coverageArgs += "-WindowsOnlyMvp"
    }
    Invoke-Tool "regenerate-coverage-summary" "codex-plus-dev-plan\tools\summarize-07-release-coverage.ps1" $coverageArgs "regenerate-coverage-summary.log" | Out-Null

    $storedCoverageText = Read-TextIfPresent $storedCoveragePath
    $regeneratedCoverageText = Read-TextIfPresent $regeneratedCoveragePath
    $storedCoverageStatus = Get-RegexValue $storedCoverageText "^\s*Coverage status\s*:\s*(complete|incomplete)\s*$"
    $regeneratedCoverageStatus = Get-RegexValue $regeneratedCoverageText "^\s*Coverage status\s*:\s*(complete|incomplete)\s*$"
    $storedCoverageMvpPackageScope = Get-RegexValue $storedCoverageText "^\s*MVP package scope\s*:\s*(windows-only|cross-platform)\s*$"
    $regeneratedCoverageMvpPackageScope = Get-RegexValue $regeneratedCoverageText "^\s*MVP package scope\s*:\s*(windows-only|cross-platform)\s*$"
    $storedCoverageMissingCount = Get-RegexValue $storedCoverageText "^\s*Missing coverage count\s*:\s*(\d+)\s*$"
    $regeneratedCoverageMissingCount = Get-RegexValue $regeneratedCoverageText "^\s*Missing coverage count\s*:\s*(\d+)\s*$"
    $storedCoverageMarkerCount = Get-RegexValue $storedCoverageText "^\s*Nonrelease marker count\s*:\s*(\d+)\s*$"
    $regeneratedCoverageMarkerCount = Get-RegexValue $regeneratedCoverageText "^\s*Nonrelease marker count\s*:\s*(\d+)\s*$"
    $storedCoverageE2EInput = Get-FieldValue $storedCoverageText "E2E evidence folder"
    $storedCoveragePackageInput = Get-FieldValue $storedCoverageText "Package evidence folder"
    $storedCoverageCompatibilityInput = Get-FieldValue $storedCoverageText "Compatibility evidence folder"
    $storedCoverageDocsInput = Get-FieldValue $storedCoverageText "Docs product copy evidence folder"
    $regeneratedCoverageE2EInput = Get-FieldValue $regeneratedCoverageText "E2E evidence folder"
    $regeneratedCoveragePackageInput = Get-FieldValue $regeneratedCoverageText "Package evidence folder"
    $regeneratedCoverageCompatibilityInput = Get-FieldValue $regeneratedCoverageText "Compatibility evidence folder"
    $regeneratedCoverageDocsInput = Get-FieldValue $regeneratedCoverageText "Docs product copy evidence folder"
    Add-Check "coverage-summary:stored-status" (-not [string]::IsNullOrWhiteSpace($storedCoverageStatus)) "stored=$storedCoverageStatus"
    Add-Check "coverage-summary:regenerated-status" (-not [string]::IsNullOrWhiteSpace($regeneratedCoverageStatus)) "regenerated=$regeneratedCoverageStatus"
    Add-Check "coverage-summary:stored-mvp-package-scope" (-not [string]::IsNullOrWhiteSpace($storedCoverageMvpPackageScope)) "stored=$storedCoverageMvpPackageScope"
    Add-Check "coverage-summary:regenerated-mvp-package-scope" (-not [string]::IsNullOrWhiteSpace($regeneratedCoverageMvpPackageScope)) "regenerated=$regeneratedCoverageMvpPackageScope"
    Add-Check "coverage-summary:stored-missing-count" (-not [string]::IsNullOrWhiteSpace($storedCoverageMissingCount)) "stored=$storedCoverageMissingCount"
    Add-Check "coverage-summary:regenerated-missing-count" (-not [string]::IsNullOrWhiteSpace($regeneratedCoverageMissingCount)) "regenerated=$regeneratedCoverageMissingCount"
    Add-Check "coverage-summary:stored-marker-count" (-not [string]::IsNullOrWhiteSpace($storedCoverageMarkerCount)) "stored=$storedCoverageMarkerCount"
    Add-Check "coverage-summary:regenerated-marker-count" (-not [string]::IsNullOrWhiteSpace($regeneratedCoverageMarkerCount)) "regenerated=$regeneratedCoverageMarkerCount"
    Add-Check "coverage-summary:e2e-input" (Test-PathReference $storedCoverageE2EInput $e2ePath) "stored=$storedCoverageE2EInput; expected=$(Get-RelativePath $e2ePath)"
    Add-Check "coverage-summary:package-input" (Test-PathReference $storedCoveragePackageInput $packagePath) "stored=$storedCoveragePackageInput; expected=$(Get-RelativePath $packagePath)"
    Add-Check "coverage-summary:compatibility-input" (Test-PathReference $storedCoverageCompatibilityInput $compatibilityPath) "stored=$storedCoverageCompatibilityInput; expected=$(Get-RelativePath $compatibilityPath)"
    Add-Check "coverage-summary:docs-input" (Test-PathReference $storedCoverageDocsInput $docsPath) "stored=$storedCoverageDocsInput; expected=$(Get-RelativePath $docsPath)"
    Add-Check "coverage-summary:e2e-input-consistency" (Test-PathReference $storedCoverageE2EInput $regeneratedCoverageE2EInput) "stored=$storedCoverageE2EInput; regenerated=$regeneratedCoverageE2EInput"
    Add-Check "coverage-summary:package-input-consistency" (Test-PathReference $storedCoveragePackageInput $regeneratedCoveragePackageInput) "stored=$storedCoveragePackageInput; regenerated=$regeneratedCoveragePackageInput"
    Add-Check "coverage-summary:compatibility-input-consistency" (Test-PathReference $storedCoverageCompatibilityInput $regeneratedCoverageCompatibilityInput) "stored=$storedCoverageCompatibilityInput; regenerated=$regeneratedCoverageCompatibilityInput"
    Add-Check "coverage-summary:docs-input-consistency" (Test-PathReference $storedCoverageDocsInput $regeneratedCoverageDocsInput) "stored=$storedCoverageDocsInput; regenerated=$regeneratedCoverageDocsInput"
    if (-not [string]::IsNullOrWhiteSpace($storedCoverageStatus)) {
        Add-Check "coverage-summary:complete" ($storedCoverageStatus -eq "complete") "stored=$storedCoverageStatus"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedCoverageStatus) -and -not [string]::IsNullOrWhiteSpace($regeneratedCoverageStatus)) {
        Add-Check "coverage-summary:status-consistency" ($storedCoverageStatus -eq $regeneratedCoverageStatus) "stored=$storedCoverageStatus; regenerated=$regeneratedCoverageStatus"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedCoverageMvpPackageScope) -and -not [string]::IsNullOrWhiteSpace($regeneratedCoverageMvpPackageScope)) {
        $expectedCoverageMvpPackageScope = if ($WindowsOnlyMvp) { "windows-only" } else { "cross-platform" }
        Add-Check "coverage-summary:mvp-package-scope-consistency" ($storedCoverageMvpPackageScope -eq $regeneratedCoverageMvpPackageScope) "stored=$storedCoverageMvpPackageScope; regenerated=$regeneratedCoverageMvpPackageScope"
        Add-Check "coverage-summary:mvp-package-scope-switch-consistency" ($storedCoverageMvpPackageScope -eq $expectedCoverageMvpPackageScope) "switch=$expectedCoverageMvpPackageScope; stored=$storedCoverageMvpPackageScope"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedCoverageMissingCount) -and -not [string]::IsNullOrWhiteSpace($regeneratedCoverageMissingCount)) {
        Add-Check "coverage-summary:missing-count-consistency" ($storedCoverageMissingCount -eq $regeneratedCoverageMissingCount) "stored=$storedCoverageMissingCount; regenerated=$regeneratedCoverageMissingCount"
        Add-Check "coverage-summary:no-missing-coverage" ($storedCoverageMissingCount -eq "0") "stored=$storedCoverageMissingCount"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedCoverageMarkerCount) -and -not [string]::IsNullOrWhiteSpace($regeneratedCoverageMarkerCount)) {
        Add-Check "coverage-summary:marker-count-consistency" ($storedCoverageMarkerCount -eq $regeneratedCoverageMarkerCount) "stored=$storedCoverageMarkerCount; regenerated=$regeneratedCoverageMarkerCount"
        Add-Check "coverage-summary:no-nonrelease-markers" ($storedCoverageMarkerCount -eq "0") "stored=$storedCoverageMarkerCount"
    }
    if (-not [string]::IsNullOrWhiteSpace($indexCoverageStatus) -and -not [string]::IsNullOrWhiteSpace($storedCoverageStatus)) {
        Add-Check "handoff-index:coverage-status-consistency" ($indexCoverageStatus -eq $storedCoverageStatus) "index=$indexCoverageStatus; stored=$storedCoverageStatus"
    }

    $regeneratedSummaryPath = Join-Path $LogRoot "regenerated-release-readiness-summary.md"
    $summaryArgs = @(
        "-Root", $Root,
        "-E2EEvidenceDir", $e2ePath,
        "-PackageEvidenceDir", $packagePath,
        "-CompatibilityEvidenceDir", $compatibilityPath,
        "-DocsEvidenceDir", $docsPath,
        "-BusinessEvidenceDir", $businessPath,
        "-CoverageSummaryFile", $storedCoveragePath,
        "-OutputFile", $regeneratedSummaryPath,
        "-LogRoot", (Join-Path $LogRoot "readiness-summary-logs")
    )
    if (-not [string]::IsNullOrWhiteSpace($BusinessSourceDocsRoot)) {
        $summaryArgs += @("-BusinessSourceDocsRoot", $BusinessSourceDocsRoot)
    }
    if ($WindowsOnlyMvp) {
        $summaryArgs += "-WindowsOnlyMvp"
    }
    if ($AllowGoCandidate) {
        $summaryArgs += "-AllowGoCandidate"
    }
    Invoke-Tool "regenerate-readiness-summary" "codex-plus-dev-plan\tools\summarize-07-release-readiness.ps1" $summaryArgs "regenerate-readiness-summary.log" | Out-Null

    $storedSummaryText = Read-TextIfPresent $storedSummaryPath
    $regeneratedSummaryText = Read-TextIfPresent $regeneratedSummaryPath
    $storedSummaryStatus = Get-RegexValue $storedSummaryText "^\s*Report status\s*:\s*(generated)\s*$"
    $regeneratedSummaryStatus = Get-RegexValue $regeneratedSummaryText "^\s*Report status\s*:\s*(generated)\s*$"
    $storedAggregate = Get-RegexValue $storedSummaryText "^\s*Aggregate evidence result\s*:\s*(passed|failed)\s*$"
    $regeneratedAggregate = Get-RegexValue $regeneratedSummaryText "^\s*Aggregate evidence result\s*:\s*(passed|failed)\s*$"
    $storedCoverageVerification = Get-RegexValue $storedSummaryText "^\s*Coverage summary verification\s*:\s*(passed|failed)\s*$"
    $regeneratedCoverageVerification = Get-RegexValue $regeneratedSummaryText "^\s*Coverage summary verification\s*:\s*(passed|failed)\s*$"
    $storedReadinessCoverageStatus = Get-RegexValue $storedSummaryText "^\s*Coverage status\s*:\s*(complete|incomplete|not recorded)\s*$"
    $regeneratedReadinessCoverageStatus = Get-RegexValue $regeneratedSummaryText "^\s*Coverage status\s*:\s*(complete|incomplete|not recorded)\s*$"
    $storedReadinessMvpPackageScope = Get-RegexValue $storedSummaryText "^\s*MVP package scope\s*:\s*(windows-only|cross-platform|not recorded)\s*$"
    $regeneratedReadinessMvpPackageScope = Get-RegexValue $regeneratedSummaryText "^\s*MVP package scope\s*:\s*(windows-only|cross-platform|not recorded)\s*$"
    $storedReadinessCoverageMissingCount = Get-RegexValue $storedSummaryText "^\s*Coverage missing count\s*:\s*(\d+|not recorded)\s*$"
    $regeneratedReadinessCoverageMissingCount = Get-RegexValue $regeneratedSummaryText "^\s*Coverage missing count\s*:\s*(\d+|not recorded)\s*$"
    $storedReadinessCoverageMarkerCount = Get-RegexValue $storedSummaryText "^\s*Coverage nonrelease marker count\s*:\s*(\d+|not recorded)\s*$"
    $regeneratedReadinessCoverageMarkerCount = Get-RegexValue $regeneratedSummaryText "^\s*Coverage nonrelease marker count\s*:\s*(\d+|not recorded)\s*$"
    $storedDocs = Get-RegexValue $storedSummaryText "^\s*Docs product copy verification\s*:\s*(passed|failed)\s*$"
    $regeneratedDocs = Get-RegexValue $regeneratedSummaryText "^\s*Docs product copy verification\s*:\s*(passed|failed)\s*$"
    $storedBusiness = Get-RegexValue $storedSummaryText "^\s*Business readiness verification\s*:\s*(passed|failed)\s*$"
    $regeneratedBusiness = Get-RegexValue $regeneratedSummaryText "^\s*Business readiness verification\s*:\s*(passed|failed)\s*$"
    $storedDocsResult = Get-RegexValue $storedSummaryText "^\s*-\s*Docs product copy result\s*:\s*(pass|fail)\s*$"
    $regeneratedDocsResult = Get-RegexValue $regeneratedSummaryText "^\s*-\s*Docs product copy result\s*:\s*(pass|fail)\s*$"
    $storedBusinessResult = Get-RegexValue $storedSummaryText "^\s*-\s*Business readiness result\s*:\s*(pass|fail)\s*$"
    $regeneratedBusinessResult = Get-RegexValue $regeneratedSummaryText "^\s*-\s*Business readiness result\s*:\s*(pass|fail)\s*$"
    $storedPosture = Get-RegexValue $storedSummaryText "^\s*Recommended Module J posture\s*:\s*(no-go|go-candidate-requires-module-j-review)\s*$"
    $regeneratedPosture = Get-RegexValue $regeneratedSummaryText "^\s*Recommended Module J posture\s*:\s*(no-go|go-candidate-requires-module-j-review)\s*$"
    $storedAllowGoCandidate = Get-RegexValue $storedSummaryText "^\s*Allow go candidate\s*:\s*(true|false)\s*$"
    $regeneratedAllowGoCandidate = Get-RegexValue $regeneratedSummaryText "^\s*Allow go candidate\s*:\s*(true|false)\s*$"
    $storedReadinessMarkerCount = Get-NonreleaseMarkerCount $storedSummaryText
    $regeneratedReadinessMarkerCount = Get-NonreleaseMarkerCount $regeneratedSummaryText
    $storedReadinessE2EInput = Get-FieldValue $storedSummaryText "E2E evidence folder"
    $storedReadinessPackageInput = Get-FieldValue $storedSummaryText "Package evidence folder"
    $storedReadinessCompatibilityInput = Get-FieldValue $storedSummaryText "Compatibility evidence folder"
    $storedReadinessDocsInput = Get-FieldValue $storedSummaryText "Docs product copy evidence folder"
    $storedReadinessBusinessInput = Get-FieldValue $storedSummaryText "Business readiness folder"
    $storedReadinessCoverageSummaryInput = Get-FieldValue $storedSummaryText "Release coverage summary"
    $regeneratedReadinessE2EInput = Get-FieldValue $regeneratedSummaryText "E2E evidence folder"
    $regeneratedReadinessPackageInput = Get-FieldValue $regeneratedSummaryText "Package evidence folder"
    $regeneratedReadinessCompatibilityInput = Get-FieldValue $regeneratedSummaryText "Compatibility evidence folder"
    $regeneratedReadinessDocsInput = Get-FieldValue $regeneratedSummaryText "Docs product copy evidence folder"
    $regeneratedReadinessBusinessInput = Get-FieldValue $regeneratedSummaryText "Business readiness folder"
    $regeneratedReadinessCoverageSummaryInput = Get-FieldValue $regeneratedSummaryText "Release coverage summary"

    Add-Check "readiness-summary:stored-generated-status" ($storedSummaryStatus -eq "generated") "stored=$storedSummaryStatus"
    Add-Check "readiness-summary:stored-aggregate" (-not [string]::IsNullOrWhiteSpace($storedAggregate)) "stored=$storedAggregate"
    Add-Check "readiness-summary:stored-coverage" (-not [string]::IsNullOrWhiteSpace($storedCoverageVerification)) "stored=$storedCoverageVerification"
    Add-Check "readiness-summary:stored-coverage-status" (-not [string]::IsNullOrWhiteSpace($storedReadinessCoverageStatus)) "stored=$storedReadinessCoverageStatus"
    Add-Check "readiness-summary:stored-mvp-package-scope" (-not [string]::IsNullOrWhiteSpace($storedReadinessMvpPackageScope)) "stored=$storedReadinessMvpPackageScope"
    Add-Check "readiness-summary:stored-coverage-missing-count" (-not [string]::IsNullOrWhiteSpace($storedReadinessCoverageMissingCount)) "stored=$storedReadinessCoverageMissingCount"
    Add-Check "readiness-summary:stored-coverage-marker-count" (-not [string]::IsNullOrWhiteSpace($storedReadinessCoverageMarkerCount)) "stored=$storedReadinessCoverageMarkerCount"
    Add-Check "readiness-summary:stored-docs" (-not [string]::IsNullOrWhiteSpace($storedDocs)) "stored=$storedDocs"
    Add-Check "readiness-summary:stored-business" (-not [string]::IsNullOrWhiteSpace($storedBusiness)) "stored=$storedBusiness"
    Add-Check "readiness-summary:stored-docs-result" (-not [string]::IsNullOrWhiteSpace($storedDocsResult)) "stored=$storedDocsResult"
    Add-Check "readiness-summary:stored-business-result" (-not [string]::IsNullOrWhiteSpace($storedBusinessResult)) "stored=$storedBusinessResult"
    Add-Check "readiness-summary:stored-posture" (-not [string]::IsNullOrWhiteSpace($storedPosture)) "stored=$storedPosture"
    Add-Check "readiness-summary:stored-allow-go-candidate" (-not [string]::IsNullOrWhiteSpace($storedAllowGoCandidate)) "stored=$storedAllowGoCandidate"
    Add-Check "readiness-summary:stored-marker-section" ($storedReadinessMarkerCount -ge 0) "stored marker count=$storedReadinessMarkerCount"
    Add-Check "readiness-summary:e2e-input" (Test-PathReference $storedReadinessE2EInput $e2ePath) "stored=$storedReadinessE2EInput; expected=$(Get-RelativePath $e2ePath)"
    Add-Check "readiness-summary:package-input" (Test-PathReference $storedReadinessPackageInput $packagePath) "stored=$storedReadinessPackageInput; expected=$(Get-RelativePath $packagePath)"
    Add-Check "readiness-summary:compatibility-input" (Test-PathReference $storedReadinessCompatibilityInput $compatibilityPath) "stored=$storedReadinessCompatibilityInput; expected=$(Get-RelativePath $compatibilityPath)"
    Add-Check "readiness-summary:docs-input" (Test-PathReference $storedReadinessDocsInput $docsPath) "stored=$storedReadinessDocsInput; expected=$(Get-RelativePath $docsPath)"
    Add-Check "readiness-summary:business-input" (Test-PathReference $storedReadinessBusinessInput $businessPath) "stored=$storedReadinessBusinessInput; expected=$(Get-RelativePath $businessPath)"
    Add-Check "readiness-summary:coverage-summary-input" (Test-PathReference $storedReadinessCoverageSummaryInput $storedCoveragePath) "stored=$storedReadinessCoverageSummaryInput; expected=$(Get-RelativePath $storedCoveragePath)"
    Add-Check "readiness-summary:regenerated-generated-status" ($regeneratedSummaryStatus -eq "generated") "regenerated=$regeneratedSummaryStatus"
    Add-Check "readiness-summary:regenerated-aggregate" (-not [string]::IsNullOrWhiteSpace($regeneratedAggregate)) "regenerated=$regeneratedAggregate"
    Add-Check "readiness-summary:regenerated-coverage" (-not [string]::IsNullOrWhiteSpace($regeneratedCoverageVerification)) "regenerated=$regeneratedCoverageVerification"
    Add-Check "readiness-summary:regenerated-coverage-status" (-not [string]::IsNullOrWhiteSpace($regeneratedReadinessCoverageStatus)) "regenerated=$regeneratedReadinessCoverageStatus"
    Add-Check "readiness-summary:regenerated-mvp-package-scope" (-not [string]::IsNullOrWhiteSpace($regeneratedReadinessMvpPackageScope)) "regenerated=$regeneratedReadinessMvpPackageScope"
    Add-Check "readiness-summary:regenerated-coverage-missing-count" (-not [string]::IsNullOrWhiteSpace($regeneratedReadinessCoverageMissingCount)) "regenerated=$regeneratedReadinessCoverageMissingCount"
    Add-Check "readiness-summary:regenerated-coverage-marker-count" (-not [string]::IsNullOrWhiteSpace($regeneratedReadinessCoverageMarkerCount)) "regenerated=$regeneratedReadinessCoverageMarkerCount"
    Add-Check "readiness-summary:regenerated-docs" (-not [string]::IsNullOrWhiteSpace($regeneratedDocs)) "regenerated=$regeneratedDocs"
    Add-Check "readiness-summary:regenerated-business" (-not [string]::IsNullOrWhiteSpace($regeneratedBusiness)) "regenerated=$regeneratedBusiness"
    Add-Check "readiness-summary:regenerated-docs-result" (-not [string]::IsNullOrWhiteSpace($regeneratedDocsResult)) "regenerated=$regeneratedDocsResult"
    Add-Check "readiness-summary:regenerated-business-result" (-not [string]::IsNullOrWhiteSpace($regeneratedBusinessResult)) "regenerated=$regeneratedBusinessResult"
    Add-Check "readiness-summary:regenerated-posture" (-not [string]::IsNullOrWhiteSpace($regeneratedPosture)) "regenerated=$regeneratedPosture"
    Add-Check "readiness-summary:regenerated-allow-go-candidate" (-not [string]::IsNullOrWhiteSpace($regeneratedAllowGoCandidate)) "regenerated=$regeneratedAllowGoCandidate"
    Add-Check "readiness-summary:regenerated-marker-section" ($regeneratedReadinessMarkerCount -ge 0) "regenerated marker count=$regeneratedReadinessMarkerCount"
    Add-Check "readiness-summary:e2e-input-consistency" (Test-PathReference $storedReadinessE2EInput $regeneratedReadinessE2EInput) "stored=$storedReadinessE2EInput; regenerated=$regeneratedReadinessE2EInput"
    Add-Check "readiness-summary:package-input-consistency" (Test-PathReference $storedReadinessPackageInput $regeneratedReadinessPackageInput) "stored=$storedReadinessPackageInput; regenerated=$regeneratedReadinessPackageInput"
    Add-Check "readiness-summary:compatibility-input-consistency" (Test-PathReference $storedReadinessCompatibilityInput $regeneratedReadinessCompatibilityInput) "stored=$storedReadinessCompatibilityInput; regenerated=$regeneratedReadinessCompatibilityInput"
    Add-Check "readiness-summary:docs-input-consistency" (Test-PathReference $storedReadinessDocsInput $regeneratedReadinessDocsInput) "stored=$storedReadinessDocsInput; regenerated=$regeneratedReadinessDocsInput"
    Add-Check "readiness-summary:business-input-consistency" (Test-PathReference $storedReadinessBusinessInput $regeneratedReadinessBusinessInput) "stored=$storedReadinessBusinessInput; regenerated=$regeneratedReadinessBusinessInput"
    Add-Check "readiness-summary:coverage-summary-input-consistency" (Test-PathReference $storedReadinessCoverageSummaryInput $regeneratedReadinessCoverageSummaryInput) "stored=$storedReadinessCoverageSummaryInput; regenerated=$regeneratedReadinessCoverageSummaryInput"
    if (-not [string]::IsNullOrWhiteSpace($storedAggregate) -and -not [string]::IsNullOrWhiteSpace($regeneratedAggregate)) {
        Add-Check "readiness-summary:aggregate-consistency" ($storedAggregate -eq $regeneratedAggregate) "stored=$storedAggregate; regenerated=$regeneratedAggregate"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedCoverageVerification) -and -not [string]::IsNullOrWhiteSpace($regeneratedCoverageVerification)) {
        Add-Check "readiness-summary:coverage-consistency" ($storedCoverageVerification -eq $regeneratedCoverageVerification) "stored=$storedCoverageVerification; regenerated=$regeneratedCoverageVerification"
        Add-Check "readiness-summary:coverage-passed" ($storedCoverageVerification -eq "passed") "stored=$storedCoverageVerification"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedReadinessCoverageStatus) -and -not [string]::IsNullOrWhiteSpace($regeneratedReadinessCoverageStatus)) {
        Add-Check "readiness-summary:coverage-status-consistency" ($storedReadinessCoverageStatus -eq $regeneratedReadinessCoverageStatus) "stored=$storedReadinessCoverageStatus; regenerated=$regeneratedReadinessCoverageStatus"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedReadinessMvpPackageScope) -and -not [string]::IsNullOrWhiteSpace($regeneratedReadinessMvpPackageScope)) {
        $expectedReadinessMvpPackageScope = if ($WindowsOnlyMvp) { "windows-only" } else { "cross-platform" }
        Add-Check "readiness-summary:mvp-package-scope-consistency" ($storedReadinessMvpPackageScope -eq $regeneratedReadinessMvpPackageScope) "stored=$storedReadinessMvpPackageScope; regenerated=$regeneratedReadinessMvpPackageScope"
        Add-Check "readiness-summary:mvp-package-scope-switch-consistency" ($storedReadinessMvpPackageScope -eq $expectedReadinessMvpPackageScope) "switch=$expectedReadinessMvpPackageScope; stored=$storedReadinessMvpPackageScope"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedReadinessCoverageMissingCount) -and -not [string]::IsNullOrWhiteSpace($regeneratedReadinessCoverageMissingCount)) {
        Add-Check "readiness-summary:coverage-missing-count-consistency" ($storedReadinessCoverageMissingCount -eq $regeneratedReadinessCoverageMissingCount) "stored=$storedReadinessCoverageMissingCount; regenerated=$regeneratedReadinessCoverageMissingCount"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedReadinessCoverageMarkerCount) -and -not [string]::IsNullOrWhiteSpace($regeneratedReadinessCoverageMarkerCount)) {
        Add-Check "readiness-summary:coverage-marker-count-consistency" ($storedReadinessCoverageMarkerCount -eq $regeneratedReadinessCoverageMarkerCount) "stored=$storedReadinessCoverageMarkerCount; regenerated=$regeneratedReadinessCoverageMarkerCount"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedDocs) -and -not [string]::IsNullOrWhiteSpace($regeneratedDocs)) {
        Add-Check "readiness-summary:docs-consistency" ($storedDocs -eq $regeneratedDocs) "stored=$storedDocs; regenerated=$regeneratedDocs"
        Add-Check "readiness-summary:docs-passed" ($storedDocs -eq "passed") "stored=$storedDocs"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedBusiness) -and -not [string]::IsNullOrWhiteSpace($regeneratedBusiness)) {
        Add-Check "readiness-summary:business-consistency" ($storedBusiness -eq $regeneratedBusiness) "stored=$storedBusiness; regenerated=$regeneratedBusiness"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedDocsResult) -and -not [string]::IsNullOrWhiteSpace($regeneratedDocsResult)) {
        Add-Check "readiness-summary:docs-result-consistency" ($storedDocsResult -eq $regeneratedDocsResult) "stored=$storedDocsResult; regenerated=$regeneratedDocsResult"
        Add-Check "readiness-summary:docs-result-pass" ($storedDocsResult -eq "pass") "stored=$storedDocsResult"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedBusinessResult) -and -not [string]::IsNullOrWhiteSpace($regeneratedBusinessResult)) {
        Add-Check "readiness-summary:business-result-consistency" ($storedBusinessResult -eq $regeneratedBusinessResult) "stored=$storedBusinessResult; regenerated=$regeneratedBusinessResult"
        Add-Check "readiness-summary:business-result-pass" ($storedBusinessResult -eq "pass") "stored=$storedBusinessResult"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedPosture) -and -not [string]::IsNullOrWhiteSpace($regeneratedPosture)) {
        Add-Check "readiness-summary:posture-consistency" ($storedPosture -eq $regeneratedPosture) "stored=$storedPosture; regenerated=$regeneratedPosture"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedAllowGoCandidate) -and -not [string]::IsNullOrWhiteSpace($regeneratedAllowGoCandidate)) {
        Add-Check "readiness-summary:allow-go-candidate-consistency" ($storedAllowGoCandidate -eq $regeneratedAllowGoCandidate) "stored=$storedAllowGoCandidate; regenerated=$regeneratedAllowGoCandidate"
    }
    if ($storedReadinessMarkerCount -ge 0 -and $regeneratedReadinessMarkerCount -ge 0) {
        Add-Check "readiness-summary:marker-section-consistency" ($storedReadinessMarkerCount -eq $regeneratedReadinessMarkerCount) "stored=$storedReadinessMarkerCount; regenerated=$regeneratedReadinessMarkerCount"
        Add-Check "readiness-summary:no-nonrelease-markers" ($storedReadinessMarkerCount -eq 0) "stored=$storedReadinessMarkerCount"
    }
    if (-not [string]::IsNullOrWhiteSpace($storedAllowGoCandidate)) {
        $expectedAllowGoCandidate = if ($AllowGoCandidate) { "true" } else { "false" }
        Add-Check "readiness-summary:allow-go-candidate-switch-consistency" ($storedAllowGoCandidate -eq $expectedAllowGoCandidate) "switch=$expectedAllowGoCandidate; stored=$storedAllowGoCandidate"
    }
    if (-not [string]::IsNullOrWhiteSpace($indexAggregateResult) -and -not [string]::IsNullOrWhiteSpace($storedAggregate)) {
        Add-Check "handoff-index:aggregate-result-consistency" ($indexAggregateResult -eq $storedAggregate) "index=$indexAggregateResult; stored=$storedAggregate"
    }
    if (-not [string]::IsNullOrWhiteSpace($indexDocsResult) -and -not [string]::IsNullOrWhiteSpace($storedDocs)) {
        Add-Check "handoff-index:docs-result-consistency" ($indexDocsResult -eq $storedDocs) "index=$indexDocsResult; stored=$storedDocs"
    }
    if (-not [string]::IsNullOrWhiteSpace($indexBusinessResult) -and -not [string]::IsNullOrWhiteSpace($storedBusiness)) {
        Add-Check "handoff-index:business-result-consistency" ($indexBusinessResult -eq $storedBusiness) "index=$indexBusinessResult; stored=$storedBusiness"
    }
    if (-not [string]::IsNullOrWhiteSpace($indexReadinessPosture) -and -not [string]::IsNullOrWhiteSpace($storedPosture)) {
        Add-Check "handoff-index:readiness-posture-consistency" ($indexReadinessPosture -eq $storedPosture) "index=$indexReadinessPosture; stored=$storedPosture"
    }

    $finalReportText = Read-TextIfPresent $finalReportPath
    $finalReportRecommendation = Get-RegexValue $finalReportText "^\s*Recommendation\s*:\s*(go|go with accepted risks|no-go)\s*$"
    Add-Check "module-j-final-report:recommendation-field" (-not [string]::IsNullOrWhiteSpace($finalReportRecommendation)) "Final report recommendation: $finalReportRecommendation"
    if (-not [string]::IsNullOrWhiteSpace($indexFinalRecommendation) -and -not [string]::IsNullOrWhiteSpace($finalReportRecommendation)) {
        Add-Check "handoff-index:final-recommendation-consistency" ($indexFinalRecommendation -eq $finalReportRecommendation) "index=$indexFinalRecommendation; report=$finalReportRecommendation"
    }

    Invoke-Tool "module-j-final-report" "codex-plus-dev-plan\tools\verify-07-module-j-report.ps1" @(
        "-Root", $Root,
        "-ReportFile", $finalReportPath,
        "-CoverageSummaryFile", $storedCoveragePath,
        "-ReadinessSummaryFile", $storedSummaryPath
    ) "module-j-final-report.log" | Out-Null
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 release handoff verification failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    Write-Host "Verifier logs: $LogRoot"
    exit 1
}

Write-Host ""
Write-Host "07 release handoff verification passed."
Write-Host "This proves handoff consistency only; it does not execute E2E, build packages, run compatibility migration, or make the release go/no-go decision."
exit 0
