param(
    [string]$Root,
    [string]$OutputRoot,
    [string]$RunStamp,
    [string]$ReleaseDir,
    [string]$OutputFile,
    [switch]$FailOnGaps
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

function Resolve-WorkspacePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    if ([System.IO.Path]::IsPathRooted($Path)) {
        return [System.IO.Path]::GetFullPath($Path)
    }
    return [System.IO.Path]::GetFullPath((Join-Path $Root $Path))
}

function Normalize-RunStamp {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return ""
    }

    $leaf = Split-Path -Leaf $Value.Trim().Trim('"', "'")
    if ($leaf -match "^(\d{8}-\d{4})-(e2e|package|compatibility|docs|business|release)$") {
        return $Matches[1]
    }
    if ($leaf -match "^(\d{8}-\d{4})$") {
        return $Matches[1]
    }

    throw "RunStamp must match YYYYMMDD-HHMM or a 07 evidence folder name such as YYYYMMDD-HHMM-release."
}

function Find-LatestRunStamp {
    if (-not (Test-Path -LiteralPath $OutputRoot -PathType Container)) {
        return ""
    }

    $stamps = @(
        Get-ChildItem -LiteralPath $OutputRoot -Directory |
            Where-Object { $_.Name -match "^(\d{8}-\d{4})-(e2e|package|compatibility|docs|business|release)$" } |
            ForEach-Object {
                if ($_.Name -match "^(\d{8}-\d{4})-") {
                    $Matches[1]
                }
            } |
            Sort-Object -Descending -Unique
    )

    if ($stamps.Count -eq 0) {
        return ""
    }
    return $stamps[0]
}

if (-not [string]::IsNullOrWhiteSpace($ReleaseDir)) {
    $releasePath = Resolve-WorkspacePath $ReleaseDir
    $releaseLeaf = Split-Path -Leaf $releasePath
    if ($releaseLeaf -notmatch "^(\d{8}-\d{4})-release$") {
        throw "ReleaseDir must end with YYYYMMDD-HHMM-release. Got: $releaseLeaf"
    }
    $RunStamp = $Matches[1]
    $OutputRoot = Split-Path -Parent $releasePath
} else {
    $RunStamp = Normalize-RunStamp $RunStamp
}

if ([string]::IsNullOrWhiteSpace($RunStamp)) {
    $RunStamp = Find-LatestRunStamp
}

if ([string]::IsNullOrWhiteSpace($RunStamp)) {
    throw "No 07 release run stamp found under $(Get-RelativePath $OutputRoot). Create one with codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1 or pass -RunStamp YYYYMMDD-HHMM."
}

$lanes = @(
    @{
        Name = "e2e"
        Directory = "$RunStamp-e2e"
        VerifierScript = "codex-plus-dev-plan\tools\verify-07-evidence.ps1"
        RequiredFiles = @(
            "00-environment.md",
            "01-test-accounts.md",
            "02-contract-checks.md",
            "03-admin-setup.md",
            "04-client-api-e2e.md",
            "05-gateway-policy-e2e.md",
            "06-desktop-manager-e2e.md",
            "07-package-install-check.md",
            "08-compatibility-migration.md",
            "09-usage-events-audit.md",
            "10-rollback-notes.md",
            "11-defects.md",
            "12-release-gate-report.md"
        )
    },
    @{
        Name = "package"
        Directory = "$RunStamp-package"
        VerifierScript = "codex-plus-dev-plan\tools\verify-07-package-evidence.ps1"
        RequiredFiles = @(
            "00-artifact-metadata.md",
            "01-windows-x64-install.md",
            "02-macos-x64-dmg.md",
            "03-macos-arm64-dmg.md",
            "04-artifact-inspection.md",
            "05-package-gate-report.md"
        )
    },
    @{
        Name = "compatibility"
        Directory = "$RunStamp-compatibility"
        VerifierScript = "codex-plus-dev-plan\tools\verify-07-compatibility-evidence.ps1"
        RequiredFiles = @(
            "00-test-context.md",
            "01-pre-upgrade-snapshot.md",
            "02-post-upgrade-cloud.md",
            "03-cloud-logout-boundary.md",
            "04-manual-provider-switch.md",
            "05-provider-sync.md",
            "06-rollback-rehearsal.md",
            "07-compatibility-gate-report.md"
        )
    },
    @{
        Name = "docs"
        Directory = "$RunStamp-docs"
        VerifierScript = "codex-plus-dev-plan\tools\verify-07-docs-product-copy-evidence.ps1"
        RequiredFiles = @(
            "00-docs-sync-record.md",
            "01-user-guide.md",
            "02-admin-operations-guide.md",
            "03-release-notes.md",
            "04-html-sync-evidence.md",
            "05-html-visual-evidence\visual-review.md",
            "05-html-visual-evidence\product-spec-desktop.png",
            "05-html-visual-evidence\product-spec-mobile.png",
            "06-docs-product-copy-gate-report.md",
            "codex-plus-product-spec.html"
        )
    },
    @{
        Name = "business"
        Directory = "$RunStamp-business"
        VerifierScript = "codex-plus-dev-plan\tools\verify-07-business-readiness.ps1"
        RequiredFiles = @(
            "11-business-readiness.md"
        )
    },
    @{
        Name = "release"
        Directory = "$RunStamp-release"
        VerifierScript = "codex-plus-dev-plan\tools\verify-07-release-handoff.ps1"
        RequiredFiles = @(
            "00-release-evidence-index.md",
            "release-coverage-summary.md",
            "release-readiness-summary.md",
            "module-j-final-report.md"
        )
    }
)

$directoryRows = New-Object System.Collections.Generic.List[object]
$fileRows = New-Object System.Collections.Generic.List[object]
$commandRows = New-Object System.Collections.Generic.List[object]
$missingItems = New-Object System.Collections.Generic.List[string]

foreach ($lane in $lanes) {
    $dirPath = Join-Path $OutputRoot $lane.Directory
    $dirExists = Test-Path -LiteralPath $dirPath -PathType Container
    $directoryRows.Add([pscustomobject]@{
        Lane = $lane.Name
        Status = if ($dirExists) { "present" } else { "missing" }
        Path = Get-RelativePath $dirPath
    })
    if (-not $dirExists) {
        $missingItems.Add("$($lane.Name): missing sibling directory $(Get-RelativePath $dirPath)") | Out-Null
    }

    foreach ($file in $lane.RequiredFiles) {
        $filePath = Join-Path $dirPath $file
        $fileExists = Test-Path -LiteralPath $filePath -PathType Leaf
        $fileRows.Add([pscustomobject]@{
            Lane = $lane.Name
            Status = if ($fileExists) { "present" } else { "missing" }
            File = Get-RelativePath $filePath
        })
        if (-not $fileExists) {
            $missingItems.Add("$($lane.Name): missing key file $(Get-RelativePath $filePath)") | Out-Null
        }
    }

    $verifierPath = Join-Path $Root $lane.VerifierScript
    $verifierExists = Test-Path -LiteralPath $verifierPath -PathType Leaf
    $evidenceArg = if ($lane.Name -eq "release") { "-ReleaseDir" } else { "-EvidenceDir" }
    $commandRows.Add([pscustomobject]@{
        Lane = $lane.Name
        Status = if ($verifierExists) { "ready" } else { "missing-script" }
        Command = "powershell -ExecutionPolicy Bypass -File $($lane.VerifierScript) $evidenceArg $(Get-RelativePath $dirPath)"
    })
    if (-not $verifierExists) {
        $missingItems.Add("$($lane.Name): missing verifier script $($lane.VerifierScript)") | Out-Null
    }
}

$e2eDir = Join-Path $OutputRoot "$RunStamp-e2e"
$packageDir = Join-Path $OutputRoot "$RunStamp-package"
$compatibilityDir = Join-Path $OutputRoot "$RunStamp-compatibility"
$docsDir = Join-Path $OutputRoot "$RunStamp-docs"
$businessDir = Join-Path $OutputRoot "$RunStamp-business"
$releaseDir = Join-Path $OutputRoot "$RunStamp-release"
$coverageFile = Join-Path $releaseDir "release-coverage-summary.md"
$readinessFile = Join-Path $releaseDir "release-readiness-summary.md"
$moduleJFile = Join-Path $releaseDir "module-j-final-report.md"

$supportCommands = @(
    "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1 -Timestamp $RunStamp",
    "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1 -E2EEvidenceDir $(Get-RelativePath $e2eDir) -PackageEvidenceDir $(Get-RelativePath $packageDir) -CompatibilityEvidenceDir $(Get-RelativePath $compatibilityDir) -DocsEvidenceDir $(Get-RelativePath $docsDir)",
    "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1 -E2EEvidenceDir $(Get-RelativePath $e2eDir) -PackageEvidenceDir $(Get-RelativePath $packageDir) -CompatibilityEvidenceDir $(Get-RelativePath $compatibilityDir) -DocsEvidenceDir $(Get-RelativePath $docsDir) -OutputFile $(Get-RelativePath $coverageFile) -FailOnIncomplete",
    "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1 -E2EEvidenceDir $(Get-RelativePath $e2eDir) -PackageEvidenceDir $(Get-RelativePath $packageDir) -CompatibilityEvidenceDir $(Get-RelativePath $compatibilityDir) -DocsEvidenceDir $(Get-RelativePath $docsDir) -BusinessEvidenceDir $(Get-RelativePath $businessDir) -CoverageSummaryFile $(Get-RelativePath $coverageFile) -OutputFile $(Get-RelativePath $readinessFile) -FailOnNoGo",
    "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-module-j-report.ps1 -ReportFile $(Get-RelativePath $moduleJFile) -CoverageSummaryFile $(Get-RelativePath $coverageFile) -ReadinessSummaryFile $(Get-RelativePath $readinessFile)",
    "powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-handoff.ps1 -ReleaseDir $(Get-RelativePath $releaseDir)"
)

function Format-MarkdownTable {
    param(
        [object[]]$Rows,
        [string[]]$Columns
    )

    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("| " + ($Columns -join " | ") + " |") | Out-Null
    $lines.Add("| " + (($Columns | ForEach-Object { "---" }) -join " | ") + " |") | Out-Null
    foreach ($row in $Rows) {
        $values = foreach ($column in $Columns) {
            $value = [string]$row.$column
            $value -replace "\|", "\|"
        }
        $lines.Add("| " + ($values -join " | ") + " |") | Out-Null
    }
    return ($lines -join [Environment]::NewLine)
}

$missingDirectoryCount = @($directoryRows | Where-Object { $_.Status -eq "missing" }).Count
$missingFileCount = @($fileRows | Where-Object { $_.Status -eq "missing" }).Count
$missingCommandCount = @($commandRows | Where-Object { $_.Status -eq "missing-script" }).Count
$gapStatus = if ($missingItems.Count -eq 0) { "none" } else { "open" }
$generatedAt = Get-Date -Format "yyyy-MM-dd HH:mm:ssK"

$missingLines = if ($missingItems.Count -eq 0) {
    "- none"
} else {
    ($missingItems | ForEach-Object { "- $_" }) -join [Environment]::NewLine
}

$supportCommandLines = ($supportCommands | ForEach-Object { "- " + [char]96 + $_ + [char]96 }) -join [Environment]::NewLine

$report = @(
    "# 07 Release Gap Report",
    "",
    "Report status: generated",
    "Generated at: $generatedAt",
    "Run stamp: $RunStamp",
    "Evidence root: $(Get-RelativePath $OutputRoot)",
    "Gap status: $gapStatus",
    "Missing sibling directories: $missingDirectoryCount",
    "Missing key files: $missingFileCount",
    "Missing verifier scripts: $missingCommandCount",
    "",
    "## Sibling Directories",
    "",
    (Format-MarkdownTable $directoryRows @("Lane", "Status", "Path")),
    "",
    "## Key Files",
    "",
    (Format-MarkdownTable $fileRows @("Lane", "Status", "File")),
    "",
    "## Verify Commands",
    "",
    (Format-MarkdownTable $commandRows @("Lane", "Status", "Command")),
    "",
    "## Suggested Run Order",
    "",
    $supportCommandLines,
    "",
    "## Missing Items",
    "",
    $missingLines,
    "",
    "## Boundary",
    "",
    "This helper is a read-only gap report. It does not execute E2E, build packages, run compatibility migration, finalize business or legal readiness, run verifiers, or make a Module J go/no-go decision."
) -join [Environment]::NewLine

if (-not [string]::IsNullOrWhiteSpace($OutputFile)) {
    $outputPath = Resolve-WorkspacePath $OutputFile
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outputPath) | Out-Null
    Set-Content -LiteralPath $outputPath -Encoding UTF8 -Value $report
    Write-Host "07 release gap report: $outputPath"
}

Write-Output $report

if ($FailOnGaps -and $missingItems.Count -gt 0) {
    exit 1
}
exit 0
