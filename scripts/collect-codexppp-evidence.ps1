$ErrorActionPreference = "SilentlyContinue"

$desktop = [Environment]::GetFolderPath("Desktop")
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$out = Join-Path $desktop ("codexppp-evidence-" + $stamp)
New-Item -ItemType Directory -Path $out -Force | Out-Null

function Mask-Secret {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) {
    return ""
  }
  $prefixLength = [Math]::Min(8, $Value.Length)
  return $Value.Substring(0, $prefixLength) + "...len=" + $Value.Length
}

function Redact-Text {
  param([string]$Text)
  if ($null -eq $Text) {
    return ""
  }

  $secretKeyPattern = '(?i)(["'']?(OPENAI_API_KEY|openai_api_key|access_token|refresh_token|id_token|api_key|authorization|bearerToken|bearer_token|token)["'']?\s*[:=]\s*["'']?)([^"''\r\n,}\s]+)'
  $Text = [regex]::Replace(
    $Text,
    $secretKeyPattern,
    [System.Text.RegularExpressions.MatchEvaluator]{
      param($m)
      return $m.Groups[1].Value + (Mask-Secret $m.Groups[3].Value)
    }
  )

  $Text = [regex]::Replace(
    $Text,
    'sk-[A-Za-z0-9_-]{16,}',
    [System.Text.RegularExpressions.MatchEvaluator]{
      param($m)
      return Mask-Secret $m.Value
    }
  )

  $Text = [regex]::Replace(
    $Text,
    '(?i)(Bearer\s+)([A-Za-z0-9._~+/=-]{16,})',
    [System.Text.RegularExpressions.MatchEvaluator]{
      param($m)
      return $m.Groups[1].Value + (Mask-Secret $m.Groups[2].Value)
    }
  )

  return $Text
}

function Write-RedactedFile {
  param(
    [string]$Source,
    [string]$Name
  )
  if (!(Test-Path -LiteralPath $Source)) {
    return
  }
  $text = Get-Content -LiteralPath $Source -Raw
  Set-Content -LiteralPath (Join-Path $out $Name) -Value (Redact-Text $text) -Encoding UTF8
}

function Write-OutputFile {
  param(
    [string]$Name,
    [scriptblock]$Command
  )
  $text = (& $Command 2>&1 | Out-String)
  Set-Content -LiteralPath (Join-Path $out $Name) -Value (Redact-Text $text) -Encoding UTF8
}

function Copy-RecentLogs {
  param(
    [string]$Directory,
    [string]$Prefix
  )
  if (!(Test-Path -LiteralPath $Directory)) {
    return
  }
  $i = 0
  Get-ChildItem -LiteralPath $Directory -File |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 5 |
    ForEach-Object {
      $i += 1
      $safeName = ($_.Name -replace '[^\w.\-]', '_')
      Write-RedactedFile $_.FullName ($Prefix + "-" + $i + "-" + $safeName + ".txt")
    }
}

$defaultCodexHome = Join-Path $env:USERPROFILE ".codex"
$managedCodexHome = Join-Path $env:LOCALAPPDATA "Codex+++\codex-home"
$codexpppDataHome = Join-Path $env:LOCALAPPDATA "Codex+++"

Write-RedactedFile (Join-Path $defaultCodexHome "config.toml") "user-config.toml"
Write-RedactedFile (Join-Path $defaultCodexHome "auth.json") "user-auth.json"
Write-RedactedFile (Join-Path $managedCodexHome "config.toml") "managed-config.toml"
Write-RedactedFile (Join-Path $managedCodexHome "auth.json") "managed-auth.json"
Write-RedactedFile (Join-Path $codexpppDataHome "codex-provider-account.txt") "codex-provider-account.txt"

Copy-RecentLogs (Join-Path $defaultCodexHome "log") "user-codex-log"
Copy-RecentLogs (Join-Path $managedCodexHome "log") "managed-codex-log"

Write-OutputFile "environment.txt" {
  "CollectedAt=$((Get-Date).ToString('o'))"
  "USERPROFILE=$env:USERPROFILE"
  "LOCALAPPDATA=$env:LOCALAPPDATA"
  "APPDATA=$env:APPDATA"
  "CODEXPPP_BACKEND_API_BASE=$env:CODEXPPP_BACKEND_API_BASE"
  "CODEXPPP_CODEX_COMMAND=$env:CODEXPPP_CODEX_COMMAND"
  "CODEX_HOME=$env:CODEX_HOME"
}

Write-OutputFile "where-codex.txt" { where.exe codex }
Write-OutputFile "appx-codex.txt" { Get-AppxPackage OpenAI.Codex | Format-List Name,PackageFullName,Version,InstallLocation }
Write-OutputFile "process-codex.txt" { Get-Process codex,Codex -ErrorAction SilentlyContinue | Select-Object ProcessName,Id,Path,StartTime | Format-List }
Write-OutputFile "codexppp-tree.txt" { Get-ChildItem -LiteralPath $codexpppDataHome -Force -Recurse | Select-Object FullName,Length,LastWriteTime | Format-Table -AutoSize }
Write-OutputFile "default-codex-tree.txt" { Get-ChildItem -LiteralPath $defaultCodexHome -Force -Recurse | Select-Object FullName,Length,LastWriteTime | Format-Table -AutoSize }

$zip = $out + ".zip"
if (Test-Path -LiteralPath $zip) {
  Remove-Item -LiteralPath $zip -Force
}
Compress-Archive -Path (Join-Path $out "*") -DestinationPath $zip -Force

Write-Host "Evidence package created:"
Write-Host $zip
