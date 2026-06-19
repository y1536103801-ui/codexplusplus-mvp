param(
    [string]$Root,
    [string]$ReportFile,
    [string]$CoverageSummaryFile,
    [string]$ReadinessSummaryFile
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($ReportFile)) {
    throw "ReportFile is required. Example: -ReportFile codex-plus-dev-plan/07-integration-release/reports/module-j-final-report.md"
}

if ([System.IO.Path]::IsPathRooted($ReportFile)) {
    $ReportPath = $ReportFile
} else {
    $ReportPath = Join-Path $Root $ReportFile
}

$ReadinessSummaryPath = ""
if (-not [string]::IsNullOrWhiteSpace($ReadinessSummaryFile)) {
    if ([System.IO.Path]::IsPathRooted($ReadinessSummaryFile)) {
        $ReadinessSummaryPath = $ReadinessSummaryFile
    } else {
        $ReadinessSummaryPath = Join-Path $Root $ReadinessSummaryFile
    }
}

$CoverageSummaryPath = ""
if (-not [string]::IsNullOrWhiteSpace($CoverageSummaryFile)) {
    if ([System.IO.Path]::IsPathRooted($CoverageSummaryFile)) {
        $CoverageSummaryPath = $CoverageSummaryFile
    } else {
        $CoverageSummaryPath = Join-Path $Root $CoverageSummaryFile
    }
}

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

function Add-Text-Check {
    param(
        [string]$Pattern,
        [string]$Name,
        [string]$Detail
    )
    Add-Check $Name ($script:reportText -match $Pattern) $Detail
}

function Get-ReportSectionText {
    param([string]$Title)
    $escapedTitle = [regex]::Escape($Title)
    $match = [regex]::Match($script:reportText, "(?ims)^##\s+$escapedTitle\s*\r?\n(?<body>.*?)(?=^##\s+|\z)")
    if (-not $match.Success) {
        return ""
    }
    return $match.Groups["body"].Value
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
    if (-not $match.Success) {
        return ""
    }
    return $match.Groups["body"].Value
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
        $actualFull = if ([System.IO.Path]::IsPathRooted($actualComparable)) {
            [System.IO.Path]::GetFullPath($actualComparable)
        } else {
            [System.IO.Path]::GetFullPath((Join-Path $Root $actualComparable))
        }
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

function Test-FailurePathSignal {
    param(
        [string]$Text,
        [string]$PersonaPattern
    )
    if ([string]::IsNullOrWhiteSpace($Text)) {
        return $false
    }

    $outcomePattern = "(?:reject(?:ed|ion)?|den(?:ied|y)|blocked|fail(?:ed)?|pass(?:ed)?)"
    $pattern = "(?is)(?:" + $PersonaPattern + ".{0,160}\b" + $outcomePattern + "\b|\b" + $outcomePattern + "\b.{0,160}" + $PersonaPattern + ")"
    return $Text -match $pattern
}

Add-Check "module-j-report-exists" (Test-Path -LiteralPath $ReportPath -PathType Leaf) $ReportPath
if (-not [string]::IsNullOrWhiteSpace($CoverageSummaryPath)) {
    Add-Check "coverage-summary-exists" (Test-Path -LiteralPath $CoverageSummaryPath -PathType Leaf) $CoverageSummaryPath
}
if (-not [string]::IsNullOrWhiteSpace($ReadinessSummaryPath)) {
    Add-Check "readiness-summary-exists" (Test-Path -LiteralPath $ReadinessSummaryPath -PathType Leaf) $ReadinessSummaryPath
}

if (Test-Path -LiteralPath $ReportPath -PathType Leaf) {
    $reportText = Get-Content -Raw -LiteralPath $ReportPath
    $script:reportText = $reportText

    Add-Text-Check "(?im)^\s*Report status\s*:\s*final\s*$" "metadata:final" "Report must be final."
    Add-Text-Check "(?im)^\s*Worker lane\s*:\s*Module J\s*$" "metadata:lane" "Worker lane must be Module J."
    Add-Text-Check "(?im)^\s*Forbidden edits\s*:\s*none\s*$" "metadata:forbidden-edits" "Forbidden edits must be none."

    foreach ($section in @(
        "Modules merged",
        "Builds and versions",
        "Conflicts resolved",
        "Contract changes from original plan",
        "Verification commands",
        "Release evidence hygiene",
        "E2E result",
        "Docs and HTML sync",
        "Remaining risks",
        "Rollback notes",
        "Final recommendation"
    )) {
        Add-Text-Check ("(?im)^##\s+" + [regex]::Escape($section) + "\s*$") "section:$section" "$section section must be present."
    }

    Add-Text-Check "(?im)^\s*Recommendation\s*:\s*(go|go with accepted risks|no-go)\s*$" "recommendation:valid-value" "Recommendation must be go, go with accepted risks, or no-go."
    Add-Text-Check "(?i)verify-07-release-evidence\.ps1" "release-evidence:aggregate-verifier" "Final report must cite the aggregate release evidence verifier."
    Add-Text-Check "(?im)^\s*(?:-\s*)?Aggregate evidence result\s*:\s*(passed|failed)\s*$" "release-evidence:aggregate-result" "Aggregate evidence result must be passed or failed."

    $releaseEvidenceSection = Get-ReportSectionText "Release evidence hygiene"
    foreach ($field in @("verifier", "E2E evidence folder", "Package evidence folder", "Compatibility evidence folder", "Docs product copy evidence folder", "Business readiness folder", "Release coverage summary", "Release readiness summary", "Aggregate evidence result", "Coverage summary status", "Docs product copy verification", "Business readiness verification")) {
        Add-Check ("release-evidence-field:$field") ($releaseEvidenceSection -match ("(?im)^\s*(?:-\s*)?" + [regex]::Escape($field) + "\s*:\s*\S")) "$field field must be present in Release evidence hygiene."
    }
    Add-Check "release-evidence:aggregate-result-scoped" ($releaseEvidenceSection -match "(?im)^\s*(?:-\s*)?Aggregate evidence result\s*:\s*(passed|failed)\s*$") "Release evidence hygiene must record aggregate evidence result as passed or failed."
    Add-Check "release-evidence:coverage-status" ($releaseEvidenceSection -match "(?im)^\s*(?:-\s*)?Coverage summary status\s*:\s*(complete|incomplete)\s*$") "Release evidence hygiene must record coverage summary status as complete or incomplete."
    Add-Check "release-evidence:docs-product-copy-result" ($releaseEvidenceSection -match "(?im)^\s*(?:-\s*)?Docs product copy verification\s*:\s*(passed|failed)\s*$") "Release evidence hygiene must record docs product copy verification as passed or failed."
    Add-Check "release-evidence:business-readiness-result" ($releaseEvidenceSection -match "(?im)^\s*(?:-\s*)?Business readiness verification\s*:\s*(passed|failed)\s*$") "Release evidence hygiene must record business readiness verification as passed or failed."
    $aggregatePassed = $releaseEvidenceSection -match "(?im)^\s*(?:-\s*)?Aggregate evidence result\s*:\s*passed\s*$"
    $coverageComplete = $releaseEvidenceSection -match "(?im)^\s*(?:-\s*)?Coverage summary status\s*:\s*complete\s*$"
    $docsProductCopyPassed = $releaseEvidenceSection -match "(?im)^\s*(?:-\s*)?Docs product copy verification\s*:\s*passed\s*$"
    $businessReadinessPassed = $releaseEvidenceSection -match "(?im)^\s*(?:-\s*)?Business readiness verification\s*:\s*passed\s*$"

    $modulesSection = Get-ReportSectionText "Modules merged"
    foreach ($field in @("module reports", "merge order", "out of scope modules")) {
        Add-Check ("modules-field:$field") ($modulesSection -match ("(?im)^\s*(?:-\s*)?" + [regex]::Escape($field) + "\s*:\s*\S")) "$field field must be present in Modules merged."
    }

    foreach ($field in @("backend", "admin frontend", "desktop manager", "contract version", "config version")) {
        Add-Text-Check ("(?im)^\s*-\s*" + [regex]::Escape($field) + "\s*:") "build-field:$field" "$field build/version field must be present."
    }

    $verificationSection = Get-ReportSectionText "Verification commands"
    foreach ($field in @("command", "result", "evidence", "skipped or unavailable", "reason unavailable", "replacement narrower check", "owner needed")) {
        Add-Check ("verification-field:$field") ($verificationSection -match ("(?im)^\s*(?:-\s*)?" + [regex]::Escape($field) + "\s*:\s*\S")) "$field verification field must be present in Verification commands."
    }

    $conflictsSection = Get-ReportSectionText "Conflicts resolved"
    foreach ($field in @("file", "modules", "rule used", "result")) {
        Add-Check ("conflict-field:$field") ($conflictsSection -match ("(?im)^\s*(?:-\s*)?" + [regex]::Escape($field) + "\s*:\s*\S")) "$field conflict resolution field must be present in Conflicts resolved."
    }

    $contractSection = Get-ReportSectionText "Contract changes from original plan"
    foreach ($field in @("drift status", "affected surface", "change review evidence", "owner", "impact")) {
        Add-Check ("contract-field:$field") ($contractSection -match ("(?im)^\s*(?:-\s*)?" + [regex]::Escape($field) + "\s*:\s*\S")) "$field contract change field must be present in Contract changes from original plan."
    }

    $e2eSection = Get-ReportSectionText "E2E result"
    $failurePaths = Get-FieldValue $e2eSection "failure paths"
    foreach ($field in @("decision", "evidence folder", "level 3 result", "happy path", "failure paths", "admin config bootstrap")) {
        Add-Text-Check ("(?im)^\s*-\s*" + [regex]::Escape($field) + "\s*:") "e2e-field:$field" "$field E2E field must be present."
    }

    foreach ($field in @("severity", "owner", "impact", "mitigation", "target date")) {
        Add-Text-Check ("(?im)^\s*(?:-\s*)?" + [regex]::Escape($field) + "\s*:") "risk-field:$field" "$field risk field must be present."
    }

    foreach ($field in @("config rollback", "backend rollback", "desktop rollback", "manual provider recovery")) {
        Add-Text-Check ("(?im)^\s*-\s*" + [regex]::Escape($field) + "\s*:") "rollback-field:$field" "$field rollback field must be present."
    }

    Add-Text-Check "(?i)HTML" "docs-sync:html" "Docs and HTML sync must mention HTML."
    Add-Text-Check "(?i)manual provider" "rollback:manual-provider" "Rollback notes must mention manual provider recovery."
    $docsSection = Get-ReportSectionText "Docs and HTML sync"

    $recommendationMatch = [regex]::Match($reportText, "(?im)^\s*Recommendation\s*:\s*(go|go with accepted risks|no-go)\s*$")
    if ($recommendationMatch.Success) {
        $recommendation = $recommendationMatch.Groups[1].Value.ToLowerInvariant()
        if ($recommendation -eq "go" -or $recommendation -eq "go with accepted risks") {
            Add-Check "recommendation:go-requires-coverage-summary" (-not [string]::IsNullOrWhiteSpace($CoverageSummaryPath) -and (Test-Path -LiteralPath $CoverageSummaryPath -PathType Leaf)) "Go or accepted-risk recommendation requires a generated coverage summary file."
            Add-Check "recommendation:go-requires-readiness-summary" (-not [string]::IsNullOrWhiteSpace($ReadinessSummaryPath) -and (Test-Path -LiteralPath $ReadinessSummaryPath -PathType Leaf)) "Go or accepted-risk recommendation requires a generated readiness summary file."
            Add-Check "recommendation:go-requires-aggregate-pass" $aggregatePassed "Go or accepted-risk recommendation requires aggregate evidence result: passed."
            Add-Check "recommendation:go-requires-coverage-complete" $coverageComplete "Go or accepted-risk recommendation requires coverage summary status: complete."
            Add-Check "recommendation:go-requires-docs-product-copy-pass" $docsProductCopyPassed "Go or accepted-risk recommendation requires docs product copy verification: passed."
            Add-Check "recommendation:go-requires-business-readiness-report-pass" $businessReadinessPassed "Go or accepted-risk recommendation requires business readiness verification: passed in Release evidence hygiene."
            Add-Check "recommendation:no-open-p0-p1" ($reportText -notmatch "(?i)\b(open P0|open P1|P0 open|P1 open)\b") "Go or accepted-risk recommendation must not report open P0/P1."
            Add-Check "recommendation:go-requires-module-reports-a-i" ($modulesSection -match "(?is)module reports\s*:.*\bA\b.*\bB\b.*\bC\b.*\bD\b.*\bE\b.*\bF\b.*\bG\b.*\bH\b.*\bI\b.*final report") "Go or accepted-risk recommendation requires final reports for Modules A through I."
            Add-Check "recommendation:go-requires-no-out-of-scope-modules" ($modulesSection -match "(?im)^\s*(?:-\s*)?out of scope modules\s*:\s*none\s*$") "Go or accepted-risk recommendation requires no out-of-scope modules."
            Add-Check "recommendation:go-requires-merge-order" ($modulesSection -match "(?is)merge order\s*:.*\bA\b.*\bB\b.*\bC\b.*\bD\b.*\bE\b.*\bF\b.*\bH\b.*\bG\b.*\bI\b") "Go or accepted-risk recommendation requires documented Module J merge order A, B, C, D, E, F, H, G, I."
            Add-Check "recommendation:no-unapproved-contract-drift" (($contractSection -match "(?im)^\s*(?:-\s*)?drift status\s*:\s*(none|approved)\b") -and ($contractSection -notmatch "(?i)\b(unapproved|pending|unreviewed)\b")) "Go or accepted-risk recommendation requires no unapproved, pending, or unreviewed contract drift."
            Add-Check "go-policy:level3-pass" ($e2eSection -match "(?im)^\s*-\s*level 3 result\s*:\s*pass\s*$") "Go or accepted-risk recommendation requires Level 3 result: pass."
            Add-Check "go-policy:e2e-happy-path-pass" ($e2eSection -match "(?im)^\s*-\s*happy path\s*:\s*(?=.*\bpass(?:ed)?\b).+$") "Go or accepted-risk recommendation requires an E2E happy path pass signal."
            Add-Check "go-policy:not-purchased-rejected" (Test-FailurePathSignal $failurePaths "\b(not purchased|unpaid|no entitlement)\b") "Go or accepted-risk recommendation requires not-purchased or unpaid rejection evidence in failure paths."
            Add-Check "go-policy:expired-rejected" (Test-FailurePathSignal $failurePaths "\bexpired\b") "Go or accepted-risk recommendation requires expired-user rejection evidence in failure paths."
            Add-Check "go-policy:revoked-device-rejected" (Test-FailurePathSignal $failurePaths "\brevoked device\b") "Go or accepted-risk recommendation requires revoked-device rejection evidence in failure paths."
            Add-Check "go-policy:unauthorized-model-rejected" (Test-FailurePathSignal $failurePaths "\b(unauthorized model|model denied|model not allowed)\b") "Go or accepted-risk recommendation requires unauthorized-model rejection evidence in failure paths."
            Add-Check "go-policy:admin-config-bootstrap" ($e2eSection -match "(?is)admin config.*bootstrap|bootstrap.*admin config") "Go or accepted-risk recommendation requires admin config reflected in bootstrap evidence."
            Add-Check "go-policy:manual-provider-preserved" ($reportText -match "(?is)manual provider.*(preserv|recover|remain|tested|documented)") "Go or accepted-risk recommendation requires manual provider preservation or recovery evidence."
            Add-Check "go-policy:docs-html-implemented-scope" (($docsSection -match "(?i)\bHTML\b") -and ($docsSection -match "(?i)implemented scope|match(?:es)? implemented|aligned with implemented")) "Go or accepted-risk recommendation requires docs and HTML to match implemented scope."
            if ($recommendation -eq "go with accepted risks") {
                Add-Check "recommendation:accepted-risks-require-impact" ($reportText -match "(?im)^\s*(?:-\s*)?impact\s*:\s*(?=.*\baccepted\b).+$") "Accepted-risk recommendation requires an accepted impact entry for remaining risks."
            }
        } elseif ($recommendation -eq "no-go") {
            Add-Text-Check "(?i)blocking reason|release blocker|no-go reason" "recommendation:no-go-reason" "No-go report must include a blocking reason."
        }
    }

    if (-not [string]::IsNullOrWhiteSpace($CoverageSummaryPath) -and (Test-Path -LiteralPath $CoverageSummaryPath -PathType Leaf)) {
        $coverageText = Get-Content -Raw -LiteralPath $CoverageSummaryPath
        $coverageStatusMatch = [regex]::Match($coverageText, "(?im)^\s*Coverage status\s*:\s*(complete|incomplete)\s*$")
        $coverageMissingMatch = [regex]::Match($coverageText, "(?im)^\s*Missing coverage count\s*:\s*(\d+)\s*$")
        $coverageMarkerMatch = [regex]::Match($coverageText, "(?im)^\s*Nonrelease marker count\s*:\s*(\d+)\s*$")
        $reportCoverageSummary = Get-FieldValue $releaseEvidenceSection "Release coverage summary"
        $reportE2EFolder = Get-FieldValue $releaseEvidenceSection "E2E evidence folder"
        $reportPackageFolder = Get-FieldValue $releaseEvidenceSection "Package evidence folder"
        $reportCompatibilityFolder = Get-FieldValue $releaseEvidenceSection "Compatibility evidence folder"
        $reportDocsFolder = Get-FieldValue $releaseEvidenceSection "Docs product copy evidence folder"
        $coverageE2EFolder = Get-FieldValue $coverageText "E2E evidence folder"
        $coveragePackageFolder = Get-FieldValue $coverageText "Package evidence folder"
        $coverageCompatibilityFolder = Get-FieldValue $coverageText "Compatibility evidence folder"
        $coverageDocsFolder = Get-FieldValue $coverageText "Docs product copy evidence folder"

        Add-Check "coverage-summary:status" $coverageStatusMatch.Success "Coverage summary must record coverage status."
        Add-Check "coverage-summary:missing-count" $coverageMissingMatch.Success "Coverage summary must record missing coverage count."
        Add-Check "coverage-summary:nonrelease-marker-count" $coverageMarkerMatch.Success "Coverage summary must record nonrelease marker count."
        Add-Check "release-evidence:coverage-summary-path" (Test-PathReference $reportCoverageSummary $CoverageSummaryPath) "Report coverage summary path must match the supplied coverage summary file."
        Add-Check "release-evidence:e2e-path-coverage-consistency" (Test-PathReference $reportE2EFolder $coverageE2EFolder) "Report E2E evidence folder must match the coverage summary input."
        Add-Check "release-evidence:package-path-coverage-consistency" (Test-PathReference $reportPackageFolder $coveragePackageFolder) "Report package evidence folder must match the coverage summary input."
        Add-Check "release-evidence:compatibility-path-coverage-consistency" (Test-PathReference $reportCompatibilityFolder $coverageCompatibilityFolder) "Report compatibility evidence folder must match the coverage summary input."
        Add-Check "release-evidence:docs-path-coverage-consistency" (Test-PathReference $reportDocsFolder $coverageDocsFolder) "Report docs product copy evidence folder must match the coverage summary input."
        if ($coverageStatusMatch.Success) {
            $reportCoverage = if ($coverageComplete) { "complete" } else { "incomplete" }
            Add-Check "coverage-summary:status-consistency" ($coverageStatusMatch.Groups[1].Value.ToLowerInvariant() -eq $reportCoverage) "Report coverage status must match coverage summary status."
        }

        if ($recommendationMatch.Success) {
            $recommendation = $recommendationMatch.Groups[1].Value.ToLowerInvariant()
            $reportIsGoCandidate = ($recommendation -eq "go" -or $recommendation -eq "go with accepted risks")
            if ($reportIsGoCandidate -and $coverageStatusMatch.Success) {
                Add-Check "recommendation:go-requires-coverage-summary-complete" ($coverageStatusMatch.Groups[1].Value.ToLowerInvariant() -eq "complete") "Go or accepted-risk recommendation requires coverage summary status: complete."
            }
            if ($reportIsGoCandidate -and $coverageMissingMatch.Success) {
                Add-Check "recommendation:go-requires-no-missing-coverage" ([int]$coverageMissingMatch.Groups[1].Value -eq 0) "Go or accepted-risk recommendation requires missing coverage count: 0."
            }
            if ($reportIsGoCandidate -and $coverageMarkerMatch.Success) {
                Add-Check "recommendation:go-requires-no-coverage-markers" ([int]$coverageMarkerMatch.Groups[1].Value -eq 0) "Go or accepted-risk recommendation requires nonrelease marker count: 0."
            }
        }
    }

    if (-not [string]::IsNullOrWhiteSpace($ReadinessSummaryPath) -and (Test-Path -LiteralPath $ReadinessSummaryPath -PathType Leaf)) {
        $summaryText = Get-Content -Raw -LiteralPath $ReadinessSummaryPath
        $summaryStatusMatch = [regex]::Match($summaryText, "(?im)^\s*Report status\s*:\s*(generated)\s*$")
        $summaryAggregateMatch = [regex]::Match($summaryText, "(?im)^\s*Aggregate evidence result\s*:\s*(passed|failed)\s*$")
        $summaryCoverageVerificationMatch = [regex]::Match($summaryText, "(?im)^\s*Coverage summary verification\s*:\s*(passed|failed)\s*$")
        $summaryCoverageStatusMatch = [regex]::Match($summaryText, "(?im)^\s*Coverage status\s*:\s*(complete|incomplete|not recorded)\s*$")
        $summaryCoverageMissingMatch = [regex]::Match($summaryText, "(?im)^\s*Coverage missing count\s*:\s*(\d+|not recorded)\s*$")
        $summaryCoverageMarkerMatch = [regex]::Match($summaryText, "(?im)^\s*Coverage nonrelease marker count\s*:\s*(\d+|not recorded)\s*$")
        $summaryDocsMatch = [regex]::Match($summaryText, "(?im)^\s*Docs product copy verification\s*:\s*(passed|failed)\s*$")
        $summaryDocsResultMatch = [regex]::Match($summaryText, "(?im)^\s*-\s*Docs product copy result\s*:\s*(pass|fail)\s*$")
        $summaryBusinessMatch = [regex]::Match($summaryText, "(?im)^\s*Business readiness verification\s*:\s*(passed|failed)\s*$")
        $summaryBusinessResultMatch = [regex]::Match($summaryText, "(?im)^\s*-\s*Business readiness result\s*:\s*(pass|fail)\s*$")
        $summaryE2ELevel3Match = [regex]::Match($summaryText, "(?im)^\s*-\s*E2E Level 3 result\s*:\s*(pass|fail)\s*$")
        $summaryPostureMatch = [regex]::Match($summaryText, "(?im)^\s*Recommended Module J posture\s*:\s*(no-go|go-candidate-requires-module-j-review)\s*$")
        $summaryAllowGoCandidateMatch = [regex]::Match($summaryText, "(?im)^\s*Allow go candidate\s*:\s*(true|false)\s*$")
        $summaryMarkerCount = Get-NonreleaseMarkerCount $summaryText
        $reportReadinessSummary = Get-FieldValue $releaseEvidenceSection "Release readiness summary"
        $summaryCoverageSummary = Get-FieldValue $summaryText "Release coverage summary"
        $reportE2EFolder = Get-FieldValue $releaseEvidenceSection "E2E evidence folder"
        $reportPackageFolder = Get-FieldValue $releaseEvidenceSection "Package evidence folder"
        $reportCompatibilityFolder = Get-FieldValue $releaseEvidenceSection "Compatibility evidence folder"
        $reportDocsFolder = Get-FieldValue $releaseEvidenceSection "Docs product copy evidence folder"
        $reportBusinessFolder = Get-FieldValue $releaseEvidenceSection "Business readiness folder"
        $summaryE2EFolder = Get-FieldValue $summaryText "E2E evidence folder"
        $summaryPackageFolder = Get-FieldValue $summaryText "Package evidence folder"
        $summaryCompatibilityFolder = Get-FieldValue $summaryText "Compatibility evidence folder"
        $summaryDocsFolder = Get-FieldValue $summaryText "Docs product copy evidence folder"
        $summaryBusinessFolder = Get-FieldValue $summaryText "Business readiness folder"

        Add-Check "readiness-summary:generated-status" $summaryStatusMatch.Success "Readiness summary must record Report status: generated."
        Add-Check "readiness-summary:aggregate-result" $summaryAggregateMatch.Success "Readiness summary must record aggregate evidence result."
        Add-Check "readiness-summary:coverage-verification" $summaryCoverageVerificationMatch.Success "Readiness summary must record coverage summary verification."
        Add-Check "readiness-summary:coverage-status" $summaryCoverageStatusMatch.Success "Readiness summary must record coverage status."
        Add-Check "readiness-summary:coverage-missing-count" $summaryCoverageMissingMatch.Success "Readiness summary must record coverage missing count."
        Add-Check "readiness-summary:coverage-marker-count" $summaryCoverageMarkerMatch.Success "Readiness summary must record coverage nonrelease marker count."
        Add-Check "readiness-summary:docs-verification" $summaryDocsMatch.Success "Readiness summary must record docs product copy verification."
        Add-Check "readiness-summary:docs-result" $summaryDocsResultMatch.Success "Readiness summary must record docs product copy result."
        Add-Check "readiness-summary:business-verification" $summaryBusinessMatch.Success "Readiness summary must record business readiness verification."
        Add-Check "readiness-summary:business-result" $summaryBusinessResultMatch.Success "Readiness summary must record business readiness result."
        Add-Check "readiness-summary:e2e-level3-result" $summaryE2ELevel3Match.Success "Readiness summary must record E2E Level 3 result."
        Add-Check "readiness-summary:posture" $summaryPostureMatch.Success "Readiness summary must record the recommended Module J posture."
        Add-Check "readiness-summary:allow-go-candidate" $summaryAllowGoCandidateMatch.Success "Readiness summary must record whether go-candidate allowance was used."
        Add-Check "readiness-summary:nonrelease-marker-section" ($summaryMarkerCount -ge 0) "Readiness summary must include a Nonrelease Markers section."
        Add-Check "readiness-summary:coverage-summary-path" (Test-PathReference $summaryCoverageSummary $CoverageSummaryPath) "Readiness summary coverage summary path must match the supplied coverage summary file."
        Add-Check "release-evidence:readiness-summary-path" (Test-PathReference $reportReadinessSummary $ReadinessSummaryPath) "Report readiness summary path must match the supplied readiness summary file."
        Add-Check "release-evidence:e2e-path-readiness-consistency" (Test-PathReference $reportE2EFolder $summaryE2EFolder) "Report E2E evidence folder must match the readiness summary input."
        Add-Check "release-evidence:package-path-readiness-consistency" (Test-PathReference $reportPackageFolder $summaryPackageFolder) "Report package evidence folder must match the readiness summary input."
        Add-Check "release-evidence:compatibility-path-readiness-consistency" (Test-PathReference $reportCompatibilityFolder $summaryCompatibilityFolder) "Report compatibility evidence folder must match the readiness summary input."
        Add-Check "release-evidence:docs-path-readiness-consistency" (Test-PathReference $reportDocsFolder $summaryDocsFolder) "Report docs product copy evidence folder must match the readiness summary input."
        Add-Check "release-evidence:business-path-readiness-consistency" (Test-PathReference $reportBusinessFolder $summaryBusinessFolder) "Report business readiness folder must match the readiness summary input."

        if ($summaryAggregateMatch.Success) {
            $reportAggregate = if ($aggregatePassed) { "passed" } else { "failed" }
            Add-Check "readiness-summary:aggregate-consistency" ($summaryAggregateMatch.Groups[1].Value.ToLowerInvariant() -eq $reportAggregate) "Report aggregate result must match readiness summary aggregate result."
        }
        if ($summaryCoverageStatusMatch.Success -and $CoverageSummaryPath -and (Test-Path -LiteralPath $CoverageSummaryPath -PathType Leaf)) {
            $coverageTextForReadiness = Get-Content -Raw -LiteralPath $CoverageSummaryPath
            $coverageStatusForReadinessMatch = [regex]::Match($coverageTextForReadiness, "(?im)^\s*Coverage status\s*:\s*(complete|incomplete)\s*$")
            $coverageMissingForReadinessMatch = [regex]::Match($coverageTextForReadiness, "(?im)^\s*Missing coverage count\s*:\s*(\d+)\s*$")
            $coverageMarkerForReadinessMatch = [regex]::Match($coverageTextForReadiness, "(?im)^\s*Nonrelease marker count\s*:\s*(\d+)\s*$")
            if ($coverageStatusForReadinessMatch.Success) {
                Add-Check "readiness-summary:coverage-status-consistency" ($summaryCoverageStatusMatch.Groups[1].Value.ToLowerInvariant() -eq $coverageStatusForReadinessMatch.Groups[1].Value.ToLowerInvariant()) "Readiness summary coverage status must match coverage summary."
            }
            if ($summaryCoverageMissingMatch.Success -and $coverageMissingForReadinessMatch.Success) {
                Add-Check "readiness-summary:coverage-missing-count-consistency" ($summaryCoverageMissingMatch.Groups[1].Value -eq $coverageMissingForReadinessMatch.Groups[1].Value) "Readiness summary missing coverage count must match coverage summary."
            }
            if ($summaryCoverageMarkerMatch.Success -and $coverageMarkerForReadinessMatch.Success) {
                Add-Check "readiness-summary:coverage-marker-count-consistency" ($summaryCoverageMarkerMatch.Groups[1].Value -eq $coverageMarkerForReadinessMatch.Groups[1].Value) "Readiness summary nonrelease marker count must match coverage summary."
            }
        }

        if ($recommendationMatch.Success -and $summaryPostureMatch.Success) {
            $recommendation = $recommendationMatch.Groups[1].Value.ToLowerInvariant()
            $summaryPosture = $summaryPostureMatch.Groups[1].Value.ToLowerInvariant()
            $reportIsGoCandidate = ($recommendation -eq "go" -or $recommendation -eq "go with accepted risks")
            Add-Check "recommendation:go-requires-readiness-go-candidate" (-not ($reportIsGoCandidate -and $summaryPosture -eq "no-go")) "Go or accepted-risk recommendation cannot override a readiness summary no-go."
            if ($summaryPosture -eq "go-candidate-requires-module-j-review" -and $summaryMarkerCount -ge 0) {
                Add-Check "readiness-summary:go-candidate-has-no-markers" ($summaryMarkerCount -eq 0) "Go-candidate readiness posture cannot contain nonrelease markers."
            }
            if ($reportIsGoCandidate -and $summaryCoverageVerificationMatch.Success) {
                Add-Check "recommendation:go-requires-readiness-coverage-pass" ($summaryCoverageVerificationMatch.Groups[1].Value.ToLowerInvariant() -eq "passed") "Go or accepted-risk recommendation requires readiness summary coverage verification: passed."
            }
            if ($reportIsGoCandidate -and $summaryDocsMatch.Success) {
                Add-Check "recommendation:go-requires-readiness-docs-pass" ($summaryDocsMatch.Groups[1].Value.ToLowerInvariant() -eq "passed") "Go or accepted-risk recommendation requires readiness summary docs product copy verification: passed."
            }
            if ($reportIsGoCandidate -and $summaryDocsResultMatch.Success) {
                Add-Check "recommendation:go-requires-docs-result-pass" ($summaryDocsResultMatch.Groups[1].Value.ToLowerInvariant() -eq "pass") "Go or accepted-risk recommendation requires docs product copy result: pass."
            }
            if ($reportIsGoCandidate -and $summaryE2ELevel3Match.Success) {
                Add-Check "recommendation:go-requires-readiness-level3-pass" ($summaryE2ELevel3Match.Groups[1].Value.ToLowerInvariant() -eq "pass") "Go or accepted-risk recommendation requires readiness summary E2E Level 3 result: pass."
            }
            if ($reportIsGoCandidate -and $summaryMarkerCount -ge 0) {
                Add-Check "recommendation:go-requires-no-readiness-markers" ($summaryMarkerCount -eq 0) "Go or accepted-risk recommendation requires no readiness nonrelease markers."
            }
            if ($reportIsGoCandidate -and $summaryAllowGoCandidateMatch.Success) {
                Add-Check "recommendation:go-requires-readiness-allow-go-candidate" ($summaryAllowGoCandidateMatch.Groups[1].Value.ToLowerInvariant() -eq "true") "Go or accepted-risk recommendation requires readiness summary Allow go candidate: true."
            }
            if ($reportIsGoCandidate -and $summaryBusinessMatch.Success) {
                Add-Check "recommendation:go-requires-business-readiness-pass" ($summaryBusinessMatch.Groups[1].Value.ToLowerInvariant() -eq "passed") "Go or accepted-risk recommendation requires business readiness verification: passed."
            }
            if ($reportIsGoCandidate -and $summaryBusinessResultMatch.Success) {
                Add-Check "recommendation:go-requires-business-result-pass" ($summaryBusinessResultMatch.Groups[1].Value.ToLowerInvariant() -eq "pass") "Go or accepted-risk recommendation requires business readiness result: pass."
            }
        }
    }

    $leakRules = @(
        @{ Name = "api-key"; Pattern = "(?i)\b(sk-[a-z0-9][a-z0-9_-]{18,}|sk-proj-[a-z0-9][a-z0-9_-]{18,}|sk-ant-[a-z0-9][a-z0-9_-]{18,})\b" },
        @{ Name = "authorization-header"; Pattern = "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{8,}" },
        @{ Name = "jwt"; Pattern = "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b" },
        @{ Name = "token-field"; Pattern = "(?i)[""']?(access_token|accessToken|refresh_token|refreshToken|id_token|idToken|poll_token|pollToken|session_token|sessionToken|desktopAccessToken|desktopRefreshToken|bearerToken|authorization|api_key|apiKey|upstream_key|upstreamKey)[""']?\s*[:=]\s*[""']?(?!\[?redacted|<redacted|REDACTED|\*{3,}|redacted\b)[A-Za-z0-9._~+/=-]{12,}" },
        @{ Name = "env-secret"; Pattern = "(?i)^[A-Z0-9_]*(KEY|TOKEN|SECRET|PASSWORD)[A-Z0-9_]*\s*=\s*(?!\[?redacted|<redacted|REDACTED|\*{3,}|redacted\b).{8,}" },
        @{ Name = "unfinished-placeholder"; Pattern = "(?i)\b(TODO|TBD|PLACEHOLDER|NOT_EXECUTED|FILL_ME|pending)\b" }
    )

    $lines = Get-Content -LiteralPath $ReportPath
    for ($index = 0; $index -lt $lines.Count; $index++) {
        $lineNumber = $index + 1
        foreach ($rule in $leakRules) {
            if ($lines[$index] -match $rule.Pattern) {
                Add-Check ("scan:{0}:line-{1}" -f $rule.Name, $lineNumber) $false "Potential $($rule.Name) match; value intentionally not printed."
            }
        }
    }
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 Module J final report verification failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "07 Module J final report verification passed."
exit 0
