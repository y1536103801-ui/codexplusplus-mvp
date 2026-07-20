param(
  [switch]$Bundle
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$tauriDir = Join-Path $repoRoot "desktop-client\src-tauri"

Push-Location $tauriDir
try {
  if ($Bundle) {
    $hasTauriCli = $false
    try {
      cargo tauri --version | Out-Null
      $hasTauriCli = $true
    } catch {
      $hasTauriCli = $false
    }

    if (-not $hasTauriCli) {
      throw "cargo tauri is required for the Windows installer bundle. Install the Tauri CLI, then rerun this script with -Bundle."
    }

    cargo tauri build
  } else {
    cargo build
  }
} finally {
  Pop-Location
}
