param(
    [switch]$SkipBackend,
    [switch]$SkipManager,
    [switch]$ReplaceExisting
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path

function Wait-BackendHealth {
    param([string]$Url = "http://127.0.0.1:8081/health")
    for ($i = 0; $i -lt 90; $i++) {
        try {
            $response = Invoke-RestMethod -Uri $Url -TimeoutSec 2
            if ($response.status -eq "ok") {
                Write-Host "Backend health: ok ($Url)"
                return
            }
        } catch {
            Start-Sleep -Seconds 2
        }
    }
    throw "Backend did not become healthy at $Url"
}

Write-Host "Codex++ repo root: $Root"

if (-not $SkipBackend) {
    $composeScript = Join-Path $Root "sub2api-main\tools\e2e\codexplus\start-local-dev-compose.ps1"
    if (-not (Test-Path -LiteralPath $composeScript)) {
        throw "Missing backend compose helper: $composeScript"
    }

    $args = @(
        "-ExecutionPolicy", "Bypass",
        "-File", $composeScript,
        "-Root", $Root,
        "-InitEnv"
    )
    if ($ReplaceExisting) {
        $args += "-ReplaceExisting"
    }

    & powershell @args
    if ($LASTEXITCODE -ne 0) {
        throw "Backend compose helper failed with exit code $LASTEXITCODE"
    }
    Wait-BackendHealth
}

if (-not $SkipManager) {
    $managerCandidates = @(
        (Join-Path $Root "_handoff-artifacts\windows\codex-plus-plus-manager-20260619-1934.exe"),
        (Join-Path $Root "CodexPlusPlus-main\target\release\codex-plus-plus-manager.exe"),
        (Join-Path $env:LOCALAPPDATA "Programs\Codex++\codex-plus-plus-manager.exe")
    )
    $manager = $managerCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    if (-not $manager) {
        throw "Cannot find Manager executable. Build it with: cd CodexPlusPlus-main; cargo build -p codex-plus-manager --release --bin codex-plus-plus-manager"
    }

    Write-Host "Starting Manager: $manager"
    Start-Process -FilePath $manager -WorkingDirectory (Split-Path -Parent $manager)
}

