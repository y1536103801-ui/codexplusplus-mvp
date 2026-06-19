param(
    [switch]$Force
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$SourceCodexHome = Join-Path $Root "_portable-env\codex-home"
$TargetCodexHome = Join-Path $env:USERPROFILE ".codex"

if (-not (Test-Path -LiteralPath $SourceCodexHome)) {
    throw "Missing portable Codex profile: $SourceCodexHome"
}

Write-Host "Portable Codex profile: $SourceCodexHome"
Write-Host "Target Codex profile:   $TargetCodexHome"

if ((Test-Path -LiteralPath $TargetCodexHome) -and -not $Force) {
    $stamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $backup = Join-Path $env:USERPROFILE ".codex.backup-before-codexplus-$stamp"
    Write-Host "Backing up existing profile to: $backup"
    Copy-Item -LiteralPath $TargetCodexHome -Destination $backup -Recurse -Force
}

if ((Test-Path -LiteralPath $TargetCodexHome) -and $Force) {
    Remove-Item -LiteralPath $TargetCodexHome -Recurse -Force
}

if (-not (Test-Path -LiteralPath $TargetCodexHome)) {
    New-Item -ItemType Directory -Force -Path $TargetCodexHome | Out-Null
}

Copy-Item -LiteralPath (Join-Path $SourceCodexHome "*") -Destination $TargetCodexHome -Recurse -Force
Write-Host "Codex profile restored. Restart Codex after this step."

