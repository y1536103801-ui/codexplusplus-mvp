param(
    [string]$Root,
    [string]$EnvFile,
    [string]$AdminBaseUrl,
    [string]$EnvPrefix = "CODEXPLUS_07_E2E_",
    [string]$EvidenceDir,
    [string]$OutputPath,
    [switch]$AllowLocalComplianceAccept
)

$ErrorActionPreference = "Stop"

if (-not $AllowLocalComplianceAccept) {
    throw "Refusing to accept admin compliance without -AllowLocalComplianceAccept."
}

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

function Get-EnvValue {
    param([string]$Suffix)
    return [Environment]::GetEnvironmentVariable($EnvPrefix + $Suffix, "Process")
}

function Import-EnvFile {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }

    $resolved = if ([System.IO.Path]::IsPathRooted($Path)) { $Path } else { Join-Path $Root $Path }
    if (-not (Test-Path -LiteralPath $resolved -PathType Leaf)) {
        throw "Env file not found: $resolved"
    }

    foreach ($line in Get-Content -LiteralPath $resolved -Encoding UTF8) {
        if ($line -match '^\s*\$env:([A-Za-z0-9_]+)\s*=\s*''(.*)''\s*$') {
            $name = $Matches[1]
            $value = $Matches[2] -replace "''", "'"
            [Environment]::SetEnvironmentVariable($name, $value, "Process")
        }
    }
}

function Test-LocalAdminBaseUrl {
    param([string]$Url)
    if ([string]::IsNullOrWhiteSpace($Url)) {
        return $false
    }
    try {
        $uri = [Uri]$Url
    } catch {
        return $false
    }
    if ($uri.Scheme -ne "http") {
        return $false
    }
    return ($uri.Host -in @("localhost", "127.0.0.1", "::1"))
}

function Join-Url {
    param(
        [string]$Base,
        [string]$Path
    )
    return $Base.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Resolve-WorkspacePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $null
    }
    if ([System.IO.Path]::IsPathRooted($Path)) {
        return $Path
    }
    return Join-Path $Root $Path
}

Import-EnvFile $EnvFile

if ([string]::IsNullOrWhiteSpace($AdminBaseUrl)) {
    $AdminBaseUrl = Get-EnvValue "ADMIN_BASE_URL"
}
if (-not (Test-LocalAdminBaseUrl $AdminBaseUrl)) {
    throw "Refusing non-local admin compliance target. AdminBaseUrl must be http://localhost, http://127.0.0.1, or http://[::1]."
}

$adminToken = Get-EnvValue "ADMIN_TOKEN"
if ([string]::IsNullOrWhiteSpace($adminToken)) {
    throw "$($EnvPrefix)ADMIN_TOKEN must be set. Token value is intentionally not printed."
}

$headers = @{ Authorization = "Bearer $adminToken" }
$statusUrl = Join-Url $AdminBaseUrl "/api/v1/admin/compliance"
$acceptUrl = Join-Url $AdminBaseUrl "/api/v1/admin/compliance/accept"

$status = Invoke-RestMethod -Method Get -Uri $statusUrl -Headers $headers -TimeoutSec 15
$data = $status.data
$phrase = $null
$language = $null
$phraseCandidateGroups = @(
    @{ Language = "en"; Names = @("ack_phrase_en", "ackPhraseEn") },
    @{ Language = "zh"; Names = @("ack_phrase_zh", "ackPhraseZh") },
    @{ Language = "en"; Names = @("ack_phrase", "ackPhrase") }
)
foreach ($candidateGroup in $phraseCandidateGroups) {
    foreach ($candidate in $candidateGroup.Names) {
        if ($null -ne $data -and $null -ne $data.$candidate -and -not [string]::IsNullOrWhiteSpace([string]$data.$candidate)) {
            $phrase = [string]$data.$candidate
            $language = [string]$candidateGroup.Language
            break
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($phrase)) {
        break
    }
}
if ([string]::IsNullOrWhiteSpace($phrase)) {
    throw "Compliance acknowledgement phrase was not returned by the local admin API."
}

$body = @{ language = $language; phrase = $phrase } | ConvertTo-Json -Compress
$accept = Invoke-RestMethod -Method Post -Uri $acceptUrl -Headers $headers -ContentType "application/json" -Body $body -TimeoutSec 15
$after = Invoke-RestMethod -Method Get -Uri $statusUrl -Headers $headers -TimeoutSec 15

$acknowledged = $false
if ($null -ne $after.data -and $null -ne $after.data.required -and -not [bool]$after.data.required) {
    $acknowledged = $true
} elseif ($null -ne $after.data -and $null -ne $after.data.acknowledgement) {
    $acknowledged = $true
} elseif ($null -ne $accept.data -and $null -ne $accept.data.required -and -not [bool]$accept.data.required) {
    $acknowledged = $true
} elseif ($null -ne $accept.data -and $null -ne $accept.data.acknowledgement) {
    $acknowledged = $true
} elseif ($null -ne $after.data -and $null -ne $after.data.acknowledged) {
    $acknowledged = [bool]$after.data.acknowledged
} elseif ($null -ne $accept.data -and $null -ne $accept.data.acknowledged) {
    $acknowledged = [bool]$accept.data.acknowledged
}

Write-Host "Local admin compliance accept completed against $AdminBaseUrl."
Write-Host "Acknowledged: $acknowledged"
Write-Host "Token values were intentionally not printed."

$evidencePath = Resolve-WorkspacePath $OutputPath
if ([string]::IsNullOrWhiteSpace($evidencePath) -and -not [string]::IsNullOrWhiteSpace($EvidenceDir)) {
    $evidencePath = Join-Path (Resolve-WorkspacePath $EvidenceDir) "03-admin-setup.md"
}

if (-not [string]::IsNullOrWhiteSpace($evidencePath)) {
    $result = if ($acknowledged) { "pass" } else { "fail" }
    $evidenceParent = Split-Path -Parent $evidencePath
    if (-not [string]::IsNullOrWhiteSpace($evidenceParent)) {
        New-Item -ItemType Directory -Path $evidenceParent -Force | Out-Null
    }
    $evidenceRunFolder = if ([string]::IsNullOrWhiteSpace($evidenceParent)) { "" } else { Split-Path -Leaf $evidenceParent }

    $evidence = @(
        "# 03 Admin Setup",
        "",
        "Run folder: $evidenceRunFolder",
        "Status: executed",
        "Result: $result",
        "",
        "## Local Compliance Acceptance",
        "",
        "- Admin base URL: $AdminBaseUrl",
        "- Target policy: local-only admin URL; production targets rejected by the helper.",
        "- Helper used: accept-07-local-admin-compliance.ps1",
        "- Allow switch: -AllowLocalComplianceAccept",
        "- Acknowledged: $acknowledged",
        "- Executed at: $(Get-Date -Format o)",
        "",
        "## Redaction",
        "",
        "- Admin token value was intentionally not printed.",
        "- Authorization header was not written to evidence.",
        "- Compliance acknowledgement phrase was not written to evidence.",
        "- Raw API response body was not written to evidence.",
        "",
        "## Remaining E2E Scope",
        "",
        "This file proves only local admin compliance acceptance. Desktop Manager login, Codex++ Cloud provider write, Codex launch, gateway request, admin audit correlation, rollback, and compatibility runtime evidence must be supplied by the broader E2E and compatibility lanes."
    )
    Set-Content -LiteralPath $evidencePath -Encoding UTF8 -Value $evidence
    Write-Host "Sanitized admin setup evidence written to $evidencePath."
}
