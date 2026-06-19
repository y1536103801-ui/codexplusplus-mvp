param(
    [switch]$SkipDockerLoad,
    [switch]$SkipBackend,
    [switch]$SkipManager
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$DeployDir = Join-Path $Root "sub2api-main\deploy"
$EnvFile = Join-Path $DeployDir ".env.codexplus-local"
$ComposeFile = Join-Path $DeployDir "docker-compose.dev.yml"
$DockerImageDir = Join-Path $Root "_portable-env\docker-images"

function Test-DockerImage {
    param([string]$ImageName)
    $id = docker image inspect $ImageName --format "{{.Id}}" 2>$null
    return -not [string]::IsNullOrWhiteSpace($id)
}

function Wait-BackendHealth {
    param([string]$Url = "http://127.0.0.1:8081/health")
    for ($i = 0; $i -lt 60; $i++) {
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

Write-Host "Codex++ portable root: $Root"

if (-not $SkipDockerLoad -and (Test-Path -LiteralPath $DockerImageDir)) {
    $imageMap = @(
        @{ Name = "sub2api-codexplus-local:dev"; File = "sub2api-codexplus-local_dev.tar" },
        @{ Name = "postgres:18-alpine"; File = "postgres_18-alpine.tar" },
        @{ Name = "redis:8-alpine"; File = "redis_8-alpine.tar" }
    )

    foreach ($image in $imageMap) {
        $tarPath = Join-Path $DockerImageDir $image.File
        if ((-not (Test-DockerImage $image.Name)) -and (Test-Path -LiteralPath $tarPath)) {
            Write-Host "Loading Docker image $($image.Name) from $tarPath"
            docker load -i $tarPath
        }
    }
}

if (-not $SkipBackend) {
    if (-not (Test-Path -LiteralPath $EnvFile)) {
        throw "Missing $EnvFile"
    }
    if (-not (Test-Path -LiteralPath $ComposeFile)) {
        throw "Missing $ComposeFile"
    }

    Push-Location $DeployDir
    try {
        docker compose --env-file ".env.codexplus-local" -p sub2api-codexplus-local -f "docker-compose.dev.yml" up -d
    } finally {
        Pop-Location
    }
    Wait-BackendHealth
}

if (-not $SkipManager) {
    $managerCandidates = @(
        (Join-Path $Root "_portable-env\installed-codexpp\codex-plus-plus-manager.exe"),
        (Join-Path $Root "CodexPlusPlus-main\target\release\codex-plus-plus-manager.exe"),
        (Join-Path $Root "_handoff-artifacts\windows\codex-plus-plus-manager-20260619-1934.exe")
    )
    $manager = $managerCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
    if (-not $manager) {
        throw "Cannot find codex-plus-plus-manager.exe"
    }
    Write-Host "Starting Manager: $manager"
    Start-Process -FilePath $manager -WorkingDirectory (Split-Path -Parent $manager)
}

