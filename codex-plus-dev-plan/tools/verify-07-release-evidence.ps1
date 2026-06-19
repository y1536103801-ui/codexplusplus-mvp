param(
    [string]$Root,
    [string]$E2EEvidenceDir,
    [string]$PackageEvidenceDir,
    [string]$CompatibilityEvidenceDir,
    [string]$DocsEvidenceDir,
    [string]$LogRoot,
    [switch]$WindowsOnlyMvp
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($LogRoot)) {
    $LogRoot = Join-Path $env:TEMP ("codexplus-07-release-evidence-" + (Get-Date -Format "yyyyMMdd-HHmmss"))
} elseif (-not [System.IO.Path]::IsPathRooted($LogRoot)) {
    $LogRoot = Join-Path $Root $LogRoot
}
New-Item -ItemType Directory -Force -Path $LogRoot | Out-Null

$results = New-Object System.Collections.Generic.List[object]
$knownVerifierCheckNames = @(
    "docs:verifier-passed"
)

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

function Resolve-EvidencePath {
    param([string]$EvidenceDir)
    if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
        return $null
    }
    if ([System.IO.Path]::IsPathRooted($EvidenceDir)) {
        return $EvidenceDir
    }
    return Join-Path $Root $EvidenceDir
}

function Invoke-EvidenceVerifier {
    param(
        [string]$Name,
        [string]$ScriptRelativePath,
        [string]$EvidenceDir,
        [string]$LogName,
        [string[]]$ExtraArgs = @()
    )

    $scriptPath = Join-Path $Root $ScriptRelativePath
    $scriptExists = Test-Path -LiteralPath $scriptPath -PathType Leaf
    Add-Check "${Name}:verifier-script-exists" $scriptExists $ScriptRelativePath
    if (-not $scriptExists) {
        return
    }

    $evidencePath = Resolve-EvidencePath $EvidenceDir
    Add-Check "${Name}:evidence-dir-provided" (-not [string]::IsNullOrWhiteSpace($evidencePath)) $EvidenceDir
    if ([string]::IsNullOrWhiteSpace($evidencePath)) {
        return
    }

    Add-Check "${Name}:evidence-dir-exists" (Test-Path -LiteralPath $evidencePath -PathType Container) $evidencePath

    $logPath = Join-Path $LogRoot $LogName
    & powershell -NoProfile -ExecutionPolicy Bypass -File $scriptPath -Root $Root -EvidenceDir $evidencePath @ExtraArgs *> $logPath
    $exitCode = $LASTEXITCODE
    Add-Check "${Name}:verifier-passed" ($exitCode -eq 0) "exit=$exitCode; log=$logPath"
}

Invoke-EvidenceVerifier "e2e" "codex-plus-dev-plan\tools\verify-07-evidence.ps1" $E2EEvidenceDir "e2e-verifier.log"
$packageVerifierArgs = @()
if ($WindowsOnlyMvp) {
    $packageVerifierArgs += "-WindowsOnlyMvp"
}
Invoke-EvidenceVerifier "package" "codex-plus-dev-plan\tools\verify-07-package-evidence.ps1" $PackageEvidenceDir "package-verifier.log" $packageVerifierArgs
Invoke-EvidenceVerifier "compatibility" "codex-plus-dev-plan\tools\verify-07-compatibility-evidence.ps1" $CompatibilityEvidenceDir "compatibility-verifier.log"
Invoke-EvidenceVerifier "docs" "codex-plus-dev-plan\tools\verify-07-docs-product-copy-evidence.ps1" $DocsEvidenceDir "docs-product-copy-verifier.log"

$e2ePath = Resolve-EvidencePath $E2EEvidenceDir
$packagePath = Resolve-EvidencePath $PackageEvidenceDir
$compatibilityPath = Resolve-EvidencePath $CompatibilityEvidenceDir
$docsPath = Resolve-EvidencePath $DocsEvidenceDir
if ($e2ePath -and $packagePath -and $compatibilityPath -and $docsPath) {
    $normalized = @(
        [System.IO.Path]::GetFullPath($e2ePath).TrimEnd('\', '/'),
        [System.IO.Path]::GetFullPath($packagePath).TrimEnd('\', '/'),
        [System.IO.Path]::GetFullPath($compatibilityPath).TrimEnd('\', '/'),
        [System.IO.Path]::GetFullPath($docsPath).TrimEnd('\', '/')
    )
    Add-Check "release-evidence:distinct-directories" (($normalized | Select-Object -Unique).Count -eq 4) ($normalized -join "; ")
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 release evidence verification failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    Write-Host "Verifier logs: $LogRoot"
    exit 1
}

Write-Host ""
Write-Host "07 release evidence verification passed."
Write-Host "This proves evidence hygiene only; Module J must still make the go/no-go recommendation from the verified evidence."
exit 0
