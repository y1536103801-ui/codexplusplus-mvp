param(
    [string]$Root,
    [string]$OutputRoot,
    [string]$Timestamp,
    [string]$ManagerBuildPath,
    [int]$RemoteDebuggingPort = 9327,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $Root "codex-plus-dev-plan\test-runs\_desktop-harness"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot = Join-Path $Root $OutputRoot
}

if ([string]::IsNullOrWhiteSpace($Timestamp)) {
    $Timestamp = Get-Date -Format "yyyyMMdd-HHmm"
}
if ($Timestamp -notmatch "^\d{8}-\d{4}$") {
    throw "Timestamp must match YYYYMMDD-HHMM."
}

if ([string]::IsNullOrWhiteSpace($ManagerBuildPath)) {
    $candidates = @(
        (Join-Path $Root "CodexPlusPlus-main\target\release\codex-plus-plus-manager.exe"),
        (Join-Path $Root "CodexPlusPlus-main\dist\windows\app\codex-plus-plus-manager.exe"),
        (Join-Path $Root "CodexPlusPlus-main\apps\codex-plus-manager\src-tauri\target\release\codex-plus-plus-manager.exe")
    )
    $ManagerBuildPath = @($candidates | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1)
    if ([string]::IsNullOrWhiteSpace($ManagerBuildPath)) {
        $ManagerBuildPath = $candidates[0]
    }
} elseif (-not [System.IO.Path]::IsPathRooted($ManagerBuildPath)) {
    $ManagerBuildPath = Join-Path $Root $ManagerBuildPath
}
$ManagerBuildPath = [System.IO.Path]::GetFullPath($ManagerBuildPath)

$runName = "$Timestamp-desktop-harness"
$runPath = Join-Path $OutputRoot $runName
if ((Test-Path -LiteralPath $runPath) -and -not $Force) {
    throw "Desktop compatibility harness already exists: $runPath. Use -Force only when intentionally regenerating helper files."
}

New-Item -ItemType Directory -Force -Path $runPath | Out-Null

$profileRoot = Join-Path $runPath "isolated-userprofile"
$codexHome = Join-Path $profileRoot ".codex"
$stateDir = Join-Path $profileRoot ".codex-session-delete"
$appData = Join-Path $runPath "AppData\Roaming"
$localAppData = Join-Path $runPath "AppData\Local"
$snapshotsDir = Join-Path $runPath "snapshots"
foreach ($dir in @($profileRoot, $codexHome, $stateDir, $appData, $localAppData, $snapshotsDir)) {
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
}

$configPath = Join-Path $codexHome "config.toml"
if (-not (Test-Path -LiteralPath $configPath) -or $Force) {
    $config = @"
model_provider = "manual-e2e"
model = "gpt-4.1-mini"

[model_providers.manual-e2e]
name = "manual-e2e"
wire_api = "responses"
requires_openai_auth = true
base_url = "https://manual-provider.invalid/v1"
experimental_bearer_token = "redacted-manual-provider-key-for-hash-only"
"@
    Set-Content -LiteralPath $configPath -Encoding UTF8 -Value $config
}

$settingsPath = Join-Path $stateDir "settings.json"
if (-not (Test-Path -LiteralPath $settingsPath) -or $Force) {
    $settings = @"
{
  "settings": {
    "activeRelayId": "manual-e2e",
    "relayProfiles": [
      {
        "id": "manual-e2e",
        "name": "manual-e2e",
        "baseUrl": "https://manual-provider.invalid/v1",
        "apiKey": "redacted-manual-provider-key-for-hash-only",
        "configContents": "model_provider = \"manual-e2e\"\n\n[model_providers.manual-e2e]\nname = \"manual-e2e\"\nwire_api = \"responses\"\nrequires_openai_auth = true\nbase_url = \"https://manual-provider.invalid/v1\"\nexperimental_bearer_token = \"redacted-manual-provider-key-for-hash-only\"\n",
        "authContents": "{}"
      }
    ]
  }
}
"@
    Set-Content -LiteralPath $settingsPath -Encoding UTF8 -Value $settings
}

$envScript = Join-Path $runPath "isolated-desktop-env.ps1"
$escapedProfile = $profileRoot -replace "'", "''"
$escapedCodexHome = $codexHome -replace "'", "''"
$escapedStateDir = $stateDir -replace "'", "''"
$escapedAppData = $appData -replace "'", "''"
$escapedLocalAppData = $localAppData -replace "'", "''"
$envContent = @"
`$env:USERPROFILE = '$escapedProfile'
`$env:HOME = '$escapedProfile'
`$env:APPDATA = '$escapedAppData'
`$env:LOCALAPPDATA = '$escapedLocalAppData'
`$env:CODEX_HOME = '$escapedCodexHome'
`$env:CODEX_PLUS_STATE_DIR = '$escapedStateDir'
`$env:__COMPAT_LAYER = 'RunAsInvoker'
`$env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = '--remote-debugging-port=$RemoteDebuggingPort --remote-allow-origins=*'
"@
Set-Content -LiteralPath $envScript -Encoding UTF8 -Value $envContent

$captureTool = Join-Path $Root "codex-plus-dev-plan\tools\capture-07-desktop-provider-snapshot.ps1"
$captureScript = Join-Path $runPath "capture-snapshot.ps1"
$escapedCaptureTool = $captureTool -replace "'", "''"
$escapedRoot = ([string]$Root) -replace "'", "''"
$escapedSnapshots = $snapshotsDir -replace "'", "''"
$captureContent = @"
param(
    [ValidateSet('pre-upgrade','post-upgrade','logout','rollback')]
    [string]`$Stage
)

`$ErrorActionPreference = 'Stop'
. "`$PSScriptRoot\isolated-desktop-env.ps1"

`$output = Join-Path '$escapedSnapshots' ("{0}.json" -f `$Stage)
powershell -NoProfile -ExecutionPolicy Bypass -File '$escapedCaptureTool' -Root '$escapedRoot' -ProfileRoot `$env:USERPROFILE -Label `$Stage -OutputPath `$output -Force
"@
Set-Content -LiteralPath $captureScript -Encoding UTF8 -Value $captureContent

$launchScript = Join-Path $runPath "launch-manager-isolated.ps1"
$escapedManager = $ManagerBuildPath -replace "'", "''"
$escapedManagerDir = (Split-Path -Parent $ManagerBuildPath) -replace "'", "''"
$launchContent = @"
`$ErrorActionPreference = 'Stop'
. "`$PSScriptRoot\isolated-desktop-env.ps1"
if (-not (Test-Path -LiteralPath '$escapedManager' -PathType Leaf)) {
    throw 'Manager executable not found: $escapedManager'
}
Start-Process -FilePath '$escapedManager' -WorkingDirectory '$escapedManagerDir'
"@
Set-Content -LiteralPath $launchScript -Encoding UTF8 -Value $launchContent

$readme = Join-Path $runPath "README.md"
$readmeContent = @"
# 07 Desktop Compatibility Harness

Status: helper generated, not evidence pass

This folder is an isolated Windows desktop harness for the remaining MVP Desktop Manager and compatibility evidence. It does not contain real credentials. It is safe preparation only until an owner explicitly authorizes local Desktop Manager/Codex execution.

## Isolation

- USERPROFILE: $profileRoot
- HOME: $profileRoot
- CODEX_HOME: $codexHome
- APPDATA: $appData
- LOCALAPPDATA: $localAppData
- Manager build path: $ManagerBuildPath
- WebView2 remote debugging port: $RemoteDebuggingPort

The seeded manual provider uses `.invalid` and a non-secret fake key string. Do not replace it with a real upstream key in this repository folder.

## Safe Preparation Command

Capture the pre-upgrade snapshot from the seeded isolated profile:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "$captureScript" -Stage pre-upgrade
```

## Authorized Runtime Sequence

Run these only after owner authorization for isolated local Desktop Manager/Codex E2E:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "$launchScript"
```

Then complete the local test flow in the visible Manager UI using test credentials only:

1. Complete Manager login through the local backend/browser handoff.
2. Confirm Codex++ Cloud provider is written/refreshed without removing `manual-e2e`.
3. Run `powershell -NoProfile -ExecutionPolicy Bypass -File "$captureScript" -Stage post-upgrade`.
4. Logout from cloud in Manager.
5. Run `powershell -NoProfile -ExecutionPolicy Bypass -File "$captureScript" -Stage logout`.
6. Select/use the manual provider and review provider sync logs with secrets redacted.
7. Rehearse rollback or restore the previous settings snapshot.
8. Run `powershell -NoProfile -ExecutionPolicy Bypass -File "$captureScript" -Stage rollback`.

## Compatibility Evidence Command

After all four snapshots exist, feed them to the compatibility inspector:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "$Root\codex-plus-dev-plan\tools\inspect-07-compatibility-snapshots.ps1" -Root "$Root" -EvidenceDir "$Root\codex-plus-dev-plan\test-runs\YYYYMMDD-HHMM-compatibility" -PreUpgradeSnapshot "$snapshotsDir\pre-upgrade.json" -PostUpgradeSnapshot "$snapshotsDir\post-upgrade.json" -LogoutSnapshot "$snapshotsDir\logout.json" -RollbackSnapshot "$snapshotsDir\rollback.json" -Force
```

Snapshot-only output is not enough for an MVP pass. The runtime evidence files must still record Manager login, provider write, manual provider switch/request, provider sync log review, Codex launch, and rollback rehearsal as `pass`.
"@
Set-Content -LiteralPath $readme -Encoding UTF8 -Value $readmeContent

Write-Host "Created isolated desktop compatibility harness: $runPath"
Write-Host "Safe prep only. It did not launch Manager, write real user config, or store real secrets."
Write-Host "Pre-upgrade snapshot command:"
Write-Host "powershell -NoProfile -ExecutionPolicy Bypass -File `"$captureScript`" -Stage pre-upgrade"
