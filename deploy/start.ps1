$ErrorActionPreference = "Stop"

$DeployDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$EnvFile = Join-Path $DeployDir ".env"
$ComposeFile = Join-Path $DeployDir "docker-compose.yml"

function New-CodexSecret {
  $bytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try {
    $rng.GetBytes($bytes)
  } finally {
    $rng.Dispose()
  }
  [Convert]::ToBase64String($bytes).TrimEnd("=").Replace("+", "-").Replace("/", "_")
}

function Write-CodexEnvFile {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$Content
  )
  $encoding = New-Object System.Text.UTF8Encoding -ArgumentList $false
  [System.IO.File]::WriteAllText($Path, $Content, $encoding)
}

function New-CodexEnvTemplate {
  param([Parameter(Mandatory = $true)][string]$Secret)
  @"
CODEXPPP_SECRET=$Secret
CODEXPPP_CLIENT_ORIGINS=
CODEXPPP_GATEWAY_RATE_LIMIT_PER_MINUTE=120
CODEXPPP_REDIS_DB=0
CODEXPPP_REDIS_PASSWORD=
CODEXPPP_CODEX_COMMAND=codex
CODEXPPP_DESKTOP_LATEST_VERSION=
CODEXPPP_DESKTOP_DOWNLOAD_URL=
CODEXPPP_DESKTOP_DOWNLOAD_SHA256=
CODEXPPP_DESKTOP_RELEASE_NOTES=
"@
}

function Get-CodexEnvSecret {
  param([Parameter(Mandatory = $true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) {
    return $null
  }
  foreach ($line in Get-Content -LiteralPath $Path) {
    if ($line -match '^\s*CODEXPPP_SECRET\s*=(.*)$') {
      return $Matches[1].Trim()
    }
  }
  return $null
}

function Set-CodexEnvSecret {
  param(
    [Parameter(Mandatory = $true)][string]$Path,
    [Parameter(Mandatory = $true)][string]$Secret
  )
  $lines = @()
  if (Test-Path -LiteralPath $Path) {
    $lines = @(Get-Content -LiteralPath $Path)
  }
  $found = $false
  $updated = foreach ($line in $lines) {
    if ($line -match '^\s*CODEXPPP_SECRET\s*=') {
      $found = $true
      "CODEXPPP_SECRET=$Secret"
    } else {
      $line
    }
  }
  if (-not $found) {
    $updated = @("CODEXPPP_SECRET=$Secret") + $updated
  }
  Write-CodexEnvFile -Path $Path -Content (($updated -join "`n") + "`n")
}

if (-not (Test-Path -LiteralPath $EnvFile)) {
  $secret = New-CodexSecret
  Write-CodexEnvFile -Path $EnvFile -Content (New-CodexEnvTemplate -Secret $secret)
  Write-Host "Created deploy/.env with a generated CODEXPPP_SECRET. Keep this file stable for this deployment."
} elseif ([string]::IsNullOrWhiteSpace((Get-CodexEnvSecret -Path $EnvFile))) {
  $secret = New-CodexSecret
  Set-CodexEnvSecret -Path $EnvFile -Secret $secret
  Write-Host "Updated deploy/.env with a generated CODEXPPP_SECRET. Keep this file stable for this deployment."
}

docker compose --env-file $EnvFile -f $ComposeFile up -d --build
docker compose --env-file $EnvFile -f $ComposeFile ps
