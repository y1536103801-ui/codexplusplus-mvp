param(
    [string]$Root,
    [string]$OutputRoot,
    [string]$Timestamp,
    [string]$EvidenceDir,
    [string]$ArtifactDir,
    [switch]$FixtureMode,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$PlanRoot = Join-Path $Root "codex-plus-dev-plan"
$DesktopRoot = Join-Path $Root "CodexPlusPlus-main"
$PackageGenerator = Join-Path $PlanRoot "tools\new-07-package-evidence.ps1"

if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $PlanRoot "test-runs"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot = Join-Path $Root $OutputRoot
}

if ([string]::IsNullOrWhiteSpace($Timestamp)) {
    $Timestamp = Get-Date -Format "yyyyMMdd-HHmm"
}

if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
    if ($Timestamp -match "^\d{8}-\d{4}-package$") {
        $runName = $Timestamp
    } elseif ($Timestamp -match "^\d{8}-\d{4}$") {
        $runName = "$Timestamp-package"
    } else {
        throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-package."
    }
    $EvidencePath = Join-Path $OutputRoot $runName
} elseif ([System.IO.Path]::IsPathRooted($EvidenceDir)) {
    $EvidencePath = $EvidenceDir
} else {
    $EvidencePath = Join-Path $Root $EvidenceDir
}

$EvidencePath = [System.IO.Path]::GetFullPath($EvidencePath)
$runName = Split-Path -Leaf $EvidencePath
if ($runName -notmatch "^\d{8}-\d{4}-package$") {
    throw "Package evidence directory must be named YYYYMMDD-HHMM-package; got $runName."
}

function Invoke-PackageScaffoldGenerator {
    $parent = Split-Path -Parent $EvidencePath
    $stamp = $runName -replace "-package$", ""
    $args = @(
        "-NoProfile",
        "-ExecutionPolicy", "Bypass",
        "-File", $PackageGenerator,
        "-Root", $Root,
        "-OutputRoot", $parent,
        "-Timestamp", $stamp
    )
    if ($Force) {
        $args += "-Force"
    }

    & powershell @args
    if ($LASTEXITCODE -ne 0) {
        throw "new-07-package-evidence.ps1 failed with exit code $LASTEXITCODE."
    }
}

$requiredEvidenceFiles = @(
    "00-artifact-metadata.md",
    "01-windows-x64-install.md",
    "02-macos-x64-dmg.md",
    "03-macos-arm64-dmg.md",
    "04-artifact-inspection.md",
    "05-package-gate-report.md"
)

$needsScaffold = -not (Test-Path -LiteralPath $EvidencePath -PathType Container)
if (-not $needsScaffold) {
    foreach ($file in $requiredEvidenceFiles) {
        if (-not (Test-Path -LiteralPath (Join-Path $EvidencePath $file) -PathType Leaf)) {
            $needsScaffold = $true
            break
        }
    }
}

if ($needsScaffold) {
    Invoke-PackageScaffoldGenerator
}

if ([string]::IsNullOrWhiteSpace($ArtifactDir)) {
    $ArtifactPath = Join-Path $DesktopRoot "dist"
} elseif ([System.IO.Path]::IsPathRooted($ArtifactDir)) {
    $ArtifactPath = $ArtifactDir
} else {
    $ArtifactPath = Join-Path $Root $ArtifactDir
}

function New-FixtureArtifacts {
    param([string]$Directory)
    New-Item -ItemType Directory -Force -Path $Directory | Out-Null
    $fixtureBody = @"
scanner precision fixture without credentials
no secret-like prefixes are embedded in this positive fixture
"@
    $fixtures = @{
        "CodexPlusPlus-1.0.0-test-windows-x64-setup.exe" = "fixture windows setup artifact without credentials`n$fixtureBody"
        "CodexPlusPlus-1.0.0-test-macos-x64.dmg" = "fixture macos x64 dmg artifact without credentials`n$fixtureBody"
        "CodexPlusPlus-1.0.0-test-macos-arm64.dmg" = "fixture macos arm64 dmg artifact without credentials`n$fixtureBody"
    }
    foreach ($entry in $fixtures.GetEnumerator()) {
        Set-Content -LiteralPath (Join-Path $Directory $entry.Key) -Encoding ASCII -Value $entry.Value
    }
}

if ($FixtureMode) {
    $ArtifactPath = Join-Path $EvidencePath "_fixture-artifacts"
    if (Test-Path -LiteralPath $ArtifactPath) {
        Remove-Item -LiteralPath $ArtifactPath -Recurse -Force
    }
    New-FixtureArtifacts $ArtifactPath
}

function Get-RelativePath {
    param([string]$Path)
    $base = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    $full = [System.IO.Path]::GetFullPath($Path)
    if ($full.StartsWith($base, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $full.Substring($base.Length).TrimStart('\', '/')
    }
    return $full
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

function Get-ArtifactKind {
    param([string]$FileName)
    if ($FileName -match "(?i)windows.*x64.*setup.*\.exe$") {
        return "windows-x64-setup"
    }
    if ($FileName -match "(?i)macos.*x64.*\.dmg$") {
        return "macos-x64-dmg"
    }
    if ($FileName -match "(?i)macos.*arm64.*\.dmg$") {
        return "macos-arm64-dmg"
    }
    return "other"
}

function Test-BytePattern {
    param(
        [byte[]]$Bytes,
        [byte[]]$Pattern
    )
    if ($Pattern.Length -eq 0 -or $Bytes.Length -lt $Pattern.Length) {
        return $false
    }
    for ($index = 0; $index -le ($Bytes.Length - $Pattern.Length); $index++) {
        $matched = $true
        for ($patternIndex = 0; $patternIndex -lt $Pattern.Length; $patternIndex++) {
            if ($Bytes[$index + $patternIndex] -ne $Pattern[$patternIndex]) {
                $matched = $false
                break
            }
        }
        if ($matched) {
            return $true
        }
    }
    return $false
}

function Test-FileContainsPattern {
    param(
        [string]$Path,
        [byte[]]$Pattern
    )
    if ($Pattern.Length -eq 0) {
        return $false
    }

    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $buffer = New-Object byte[] 1048576
        $tail = New-Object byte[] 0
        while (($read = $stream.Read($buffer, 0, $buffer.Length)) -gt 0) {
            $chunk = New-Object byte[] ($tail.Length + $read)
            if ($tail.Length -gt 0) {
                [Array]::Copy($tail, 0, $chunk, 0, $tail.Length)
            }
            [Array]::Copy($buffer, 0, $chunk, $tail.Length, $read)

            if (Test-BytePattern $chunk $Pattern) {
                return $true
            }

            $tailLength = [Math]::Min($Pattern.Length - 1, $chunk.Length)
            $tail = New-Object byte[] $tailLength
            if ($tailLength -gt 0) {
                [Array]::Copy($chunk, $chunk.Length - $tailLength, $tail, 0, $tailLength)
            }
        }
    } finally {
        $stream.Dispose()
    }

    return $false
}

function Test-FileContainsRegex {
    param(
        [string]$Path,
        [string]$Pattern
    )
    if ([string]::IsNullOrWhiteSpace($Pattern)) {
        return $false
    }

    $regex = [regex]::new($Pattern, [System.Text.RegularExpressions.RegexOptions]::IgnoreCase)
    $singleByteEncoding = [System.Text.Encoding]::GetEncoding("iso-8859-1")
    $utf16Encoding = [System.Text.Encoding]::Unicode
    $stream = [System.IO.File]::OpenRead($Path)
    try {
        $buffer = New-Object byte[] 1048576
        $tail = New-Object byte[] 0
        $tailLimit = 65536
        while (($read = $stream.Read($buffer, 0, $buffer.Length)) -gt 0) {
            $chunk = New-Object byte[] ($tail.Length + $read)
            if ($tail.Length -gt 0) {
                [Array]::Copy($tail, 0, $chunk, 0, $tail.Length)
            }
            [Array]::Copy($buffer, 0, $chunk, $tail.Length, $read)

            $singleByteText = $singleByteEncoding.GetString($chunk)
            if ($regex.IsMatch($singleByteText)) {
                return $true
            }

            foreach ($offset in @(0, 1)) {
                if ($chunk.Length -le $offset) {
                    continue
                }
                $length = $chunk.Length - $offset
                if (($length % 2) -ne 0) {
                    $length--
                }
                if ($length -le 0) {
                    continue
                }
                $utf16Text = $utf16Encoding.GetString($chunk, $offset, $length)
                if ($regex.IsMatch($utf16Text)) {
                    return $true
                }
            }

            $tailLength = [Math]::Min($tailLimit, $chunk.Length)
            $tail = New-Object byte[] $tailLength
            if ($tailLength -gt 0) {
                [Array]::Copy($chunk, $chunk.Length - $tailLength, $tail, 0, $tailLength)
            }
        }
    } finally {
        $stream.Dispose()
    }

    return $false
}

function Get-AsciiPattern {
    param([string]$Text)
    return [System.Text.Encoding]::ASCII.GetBytes($Text)
}

function Get-Utf16Pattern {
    param([string]$Text)
    return [System.Text.Encoding]::Unicode.GetBytes($Text)
}

function Test-InstallerScriptRisk {
    param([string]$RelativePath)
    $path = Join-Path $Root $RelativePath
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        return [pscustomobject]@{
            Script = $RelativePath
            Missing = $true
            RiskCount = 0
        }
    }
    $text = Get-Content -Raw -LiteralPath $path
    $riskPatterns = @(
        "(?i)\b(auth\.json|config\.toml)\b",
        "(?i)\b(api_key|upstream_key|access_token|refresh_token|session_token)\b",
        "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\b",
        "(?i)\bsk-(proj-|ant-)?[A-Za-z0-9_-]{8,}\b"
    )
    $count = 0
    foreach ($pattern in $riskPatterns) {
        if ($text -match $pattern) {
            $count++
        }
    }
    return [pscustomobject]@{
        Script = $RelativePath
        Missing = $false
        RiskCount = $count
    }
}

$artifacts = @()
if (Test-Path -LiteralPath $ArtifactPath -PathType Container) {
    $artifactExtensions = @(".dmg", ".exe", ".msi", ".zip", ".tar", ".gz")
    $artifacts = @(Get-ChildItem -LiteralPath $ArtifactPath -Recurse -File |
        Where-Object { $artifactExtensions -contains $_.Extension.ToLowerInvariant() } |
        Sort-Object FullName)
}

$artifactRows = New-Object System.Collections.Generic.List[object]
foreach ($artifact in $artifacts) {
    $hash = Get-FileHash -LiteralPath $artifact.FullName -Algorithm SHA256
    $artifactRows.Add([pscustomobject]@{
        Kind = Get-ArtifactKind $artifact.Name
        Name = ConvertTo-SafeEvidenceText $artifact.Name
        RelativePath = ConvertTo-SafeEvidenceText (Get-RelativePath $artifact.FullName)
        SizeBytes = $artifact.Length
        Sha256 = $hash.Hash.ToLowerInvariant()
    })
}

$coverage = @{
    "windows-x64-setup" = $false
    "macos-x64-dmg" = $false
    "macos-arm64-dmg" = $false
}
foreach ($row in $artifactRows) {
    if ($coverage.ContainsKey($row.Kind)) {
        $coverage[$row.Kind] = $true
    }
}

$scanRules = @(
    @{ Name = "api-key-like"; Category = "shared-key-or-user-credential"; Kind = "regex"; Patterns = @("(?<![A-Za-z0-9_-])sk-(?:proj-|ant-)?[A-Za-z0-9_-]{20,}(?![A-Za-z0-9_-])") },
    @{ Name = "authorization-token"; Category = "user-credential"; Kind = "regex"; Patterns = @("\bAuthorization\s*:\s*(?:Bearer|Basic)\s+[A-Za-z0-9._~+/=-]{12,}(?![A-Za-z0-9._~+/=-])") },
    @{ Name = "jwt-like"; Category = "user-credential"; Kind = "regex"; Patterns = @("(?<![A-Za-z0-9_-])eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}(?![A-Za-z0-9_-])") },
    @{ Name = "fixed-plan-policy-file"; Category = "fixed-commercial-policy"; Kind = "literal"; Patterns = @("plan-catalog.json", "model-catalog.json", "usage-policy.json", "plan_catalog", "model_catalog", "usage_policy", "usage_policy_id", "price_amount", "price_amount_minor", "plan_id", "entitlement", "multiplier", "quota") }
)

$scanFindings = New-Object System.Collections.Generic.List[object]
foreach ($artifact in $artifacts) {
    foreach ($rule in $scanRules) {
        foreach ($patternText in $rule.Patterns) {
            $matched = $false
            if ($rule.Kind -eq "regex") {
                if (Test-FileContainsRegex $artifact.FullName $patternText) {
                    $matched = $true
                }
            } else {
                $patterns = @((Get-AsciiPattern $patternText), (Get-Utf16Pattern $patternText))
                foreach ($pattern in $patterns) {
                    if (Test-FileContainsPattern $artifact.FullName $pattern) {
                        $matched = $true
                        break
                    }
                }
            }
            if ($matched) {
                $scanFindings.Add([pscustomobject]@{
                    Artifact = ConvertTo-SafeEvidenceText (Get-RelativePath $artifact.FullName)
                    Rule = $rule.Name
                    Category = $rule.Category
                })
                break
            }
        }
    }
}

$scriptRisks = New-Object System.Collections.Generic.List[object]
$scriptRisks.Add((Test-InstallerScriptRisk "CodexPlusPlus-main\scripts\installer\windows\CodexPlusPlus.nsi"))
$scriptRisks.Add((Test-InstallerScriptRisk "CodexPlusPlus-main\scripts\installer\macos\package-dmg.sh"))

$missingCoverage = @($coverage.GetEnumerator() | Where-Object { -not $_.Value } | ForEach-Object { $_.Key })
$scriptRiskCount = @($scriptRisks | Where-Object { $_.Missing -or $_.RiskCount -gt 0 }).Count
$inspectionPassed = (
    (Test-Path -LiteralPath $ArtifactPath -PathType Container) -and
    ($artifactRows.Count -gt 0) -and
    ($missingCoverage.Count -eq 0) -and
    ($scanFindings.Count -eq 0) -and
    ($scriptRiskCount -eq 0)
)

$artifactList = if ($artifactRows.Count -gt 0) {
    ($artifactRows | ForEach-Object {
        "- $($_.Kind): $($_.Name); size_bytes=$($_.SizeBytes); sha256=$($_.Sha256)"
    }) -join [Environment]::NewLine
} else {
    "- none found"
}

$coverageList = ($coverage.GetEnumerator() | Sort-Object Name | ForEach-Object {
    $result = if ($_.Value) { "present" } else { "missing" }
    "- $($_.Key): $result"
}) -join [Environment]::NewLine

$scanList = if ($scanFindings.Count -gt 0) {
    ($scanFindings | ForEach-Object {
        "- $($_.Artifact): $($_.Rule) / $($_.Category); value intentionally not printed"
    }) -join [Environment]::NewLine
} else {
    "- none"
}

$scriptRiskList = ($scriptRisks | ForEach-Object {
    if ($_.Missing) {
        "- $($_.Script): missing"
    } elseif ($_.RiskCount -gt 0) {
        "- $($_.Script): $($_.RiskCount) risk pattern(s); value intentionally not printed"
    } else {
        "- $($_.Script): no credential-write risk pattern found"
    }
}) -join [Environment]::NewLine

$resultText = if ($inspectionPassed) { "pass" } else { "fail" }
$artifactRootRelative = ConvertTo-SafeEvidenceText (Get-RelativePath $ArtifactPath)

$metadata = @"
# 00 Artifact Metadata

Run folder: $runName
Status: executed
Result: $resultText

Version/tag: not supplied by this local artifact inspection runner.
Build source: local artifact directory $artifactRootRelative.
Build URL or CI run: not supplied by this local artifact inspection runner.

## Artifact names

$artifactList

## Artifact hashes

$artifactList

## Expected Artifact Coverage

$coverageList

## Redaction

Only artifact names, sizes, hashes and scanner rule names are recorded. Token values are intentionally not printed.
Credential scanning uses complete API-key-like, JWT-like, and Authorization token shapes. Prefix-only strings such as sk- or eyJ are not findings.
"@

Set-Content -LiteralPath (Join-Path $EvidencePath "00-artifact-metadata.md") -Encoding UTF8 -Value $metadata

$sharedKeyResult = if (@($scanFindings | Where-Object { $_.Category -eq "shared-key-or-user-credential" }).Count -eq 0) { "pass" } else { "fail" }
$credentialResult = if (@($scanFindings | Where-Object { $_.Category -eq "user-credential" -or $_.Category -eq "shared-key-or-user-credential" }).Count -eq 0) { "pass" } else { "fail" }
$policyResult = if (@($scanFindings | Where-Object { $_.Category -eq "fixed-commercial-policy" }).Count -eq 0) { "pass" } else { "fail" }
$installerScriptResult = if ($scriptRiskCount -eq 0) { "pass" } else { "fail" }

$inspection = @"
# 04 Artifact Inspection

Result: $resultText

Package does not embed a shared Key: $sharedKeyResult.
Package does not embed user credentials: $credentialResult.
Package does not embed fixed price, plan, or model policy: $policyResult.
Installer scripts do not write Codex credentials: $installerScriptResult.
Package install does not overwrite existing manual provider configuration: covered by platform install and compatibility evidence; artifact inspection records package hygiene only.

## Commands Executed

- `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1 -ArtifactDir <artifact-dir> -EvidenceDir <package-evidence-dir>`

## Artifact Coverage

$coverageList

## High Confidence Scanner Findings

$scanList

Scanner precision note: credential rules require complete token shapes. Prefix-only strings such as sk- or eyJ are not treated as credential evidence.

## Installer Script Scan

$scriptRiskList

## Release Boundary

This runner proves only local artifact-inspection hygiene. Windows install, macOS x64 install, macOS arm64 install, missing-Codex first-run, overwrite install, uninstall, reinstall, E2E and compatibility migration evidence remain separate release gates.
"@

Set-Content -LiteralPath (Join-Path $EvidencePath "04-artifact-inspection.md") -Encoding UTF8 -Value $inspection

Write-Host "07 package artifact inspection evidence: $EvidencePath"
Write-Host "Artifact directory: $ArtifactPath"
Write-Host "Artifact inspection result: $resultText"
Write-Host "Scanner findings: $($scanFindings.Count); values intentionally not printed."

if (-not $inspectionPassed) {
    Write-Host "07 package artifact inspection failed. Missing coverage: $($missingCoverage -join ', ')." -ForegroundColor Red
    exit 1
}

Write-Host "07 package artifact inspection passed."
exit 0
