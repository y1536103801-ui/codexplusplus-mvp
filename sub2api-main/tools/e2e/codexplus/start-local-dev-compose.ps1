param(
    [string]$Root,
    [string]$EnvFile,
    [string]$ProjectName = "sub2api-codexplus-local",
    [string]$ImageTag = "sub2api-codexplus-local:dev",
    [string]$AppContainerName = "sub2api-codexplus-local",
    [string]$PostgresContainerName = "sub2api-codexplus-postgres",
    [string]$RedisContainerName = "sub2api-codexplus-redis",
    [string]$BindHost = "127.0.0.1",
    [int]$HostPort = 8081,
    [switch]$InitEnv,
    [switch]$SkipBuild,
    [switch]$ReplaceExisting,
    [switch]$ProbeOnly,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = (Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")).Path
} else {
    $Root = (Resolve-Path $Root).Path
}

$DeployDir = Join-Path $Root "sub2api-main\deploy"
$ComposeFile = Join-Path $DeployDir "docker-compose.dev.yml"
$ExampleEnvFile = Join-Path $DeployDir ".env.codexplus-local.example"
if ([string]::IsNullOrWhiteSpace($EnvFile)) {
    $EnvFile = Join-Path $DeployDir ".env.codexplus-local"
} elseif (-not [System.IO.Path]::IsPathRooted($EnvFile)) {
    $EnvFile = Join-Path $Root $EnvFile
}
$EnvFile = [System.IO.Path]::GetFullPath($EnvFile)

$ProbeScript = Join-Path $Root "sub2api-main\tools\e2e\codexplus\start-local-source-service.ps1"
$BaseUrl = "http://${BindHost}:$HostPort"

$results = New-Object System.Collections.Generic.List[object]

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

function Invoke-DockerText {
    param([string[]]$Arguments)
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $output = & docker @Arguments 2>&1
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    if ($exitCode -ne 0) {
        throw "docker $($Arguments -join ' ') failed with exit code $exitCode. $output"
    }
    return $output
}

function Invoke-DockerCode {
    param([string[]]$Arguments)
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & docker @Arguments 2>&1 | ForEach-Object { Write-Host $_ }
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    return [int]$exitCode
}

function Test-DockerAvailable {
    try {
        Invoke-DockerText -Arguments @("version", "--format", "{{.Server.Version}}") | Out-Null
        Add-Check "docker:available" $true "Docker daemon is reachable."
        return $true
    } catch {
        Add-Check "docker:available" $false "Docker daemon must be reachable to run the isolated dev compose stack."
        return $false
    }
}

function Test-ContainerExists {
    param([string]$Name)
    $names = Invoke-DockerText -Arguments @("ps", "-a", "--filter", "name=^/$Name$", "--format", "{{.Names}}")
    return @($names | Where-Object { $_ -eq $Name }).Count -gt 0
}

function Get-ComposeLabel {
    param(
        [string]$Name,
        [string]$Label
    )
    try {
        $labelJson = Invoke-DockerText -Arguments @("inspect", $Name, "--format", "{{json .Config.Labels}}")
        $labels = ($labelJson | Select-Object -First 1) | ConvertFrom-Json
        if ($null -eq $labels -or -not ($labels.PSObject.Properties.Name -contains $Label)) {
            return ""
        }
        return [string]$labels.$Label
    } catch {
        return ""
    }
}

function Test-Or-Remove-ContainerConflict {
    param([string]$Name)
    if (-not (Test-ContainerExists $Name)) {
        Add-Check "container:${Name}:available" $true "No pre-existing container named $Name."
        return $true
    }

    $project = Get-ComposeLabel $Name "com.docker.compose.project"
    $service = Get-ComposeLabel $Name "com.docker.compose.service"
    if ($project -eq $ProjectName -and -not [string]::IsNullOrWhiteSpace($service) -and $service -ne "<no value>") {
        Add-Check "container:${Name}:compose-owned" $true "Container belongs to compose project $ProjectName service $service."
        return $true
    }

    if ($ReplaceExisting) {
        $removeCode = Invoke-DockerCode -Arguments @("rm", "-f", $Name)
        Add-Check "container:${Name}:removed-conflict" ($removeCode -eq 0) "Removed non-compose/conflicting container. Previous compose project label: $project."
        return $removeCode -eq 0
    }

    Add-Check "container:${Name}:no-conflict" $false "Container exists but is not owned by compose project $ProjectName. Re-run with -ReplaceExisting to replace only these local dev container names."
    return $false
}

function Invoke-Compose {
    param([string[]]$Arguments)
    $composeArgs = @("compose", "--env-file", $EnvFile, "-p", $ProjectName, "-f", $ComposeFile) + $Arguments
    return Invoke-DockerCode -Arguments $composeArgs
}

function Test-ComposeConfig {
    $composeArgs = @("compose", "--env-file", $EnvFile, "-p", $ProjectName, "-f", $ComposeFile, "config")
    try {
        Invoke-DockerText -Arguments $composeArgs | Out-Null
        return 0
    } catch {
        Write-Host $_
        return 1
    }
}

function Invoke-RouteProbe {
    $probeArgs = @(
        "-NoProfile",
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        $ProbeScript,
        "-Root",
        $Root,
        "-SkipBuild",
        "-ProbeOnly",
        "-ContainerName",
        $AppContainerName,
        "-ExpectedImage",
        $ImageTag,
        "-BindHost",
        $BindHost,
        "-HostPort",
        ([string]$HostPort)
    )
    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        & powershell @probeArgs 2>&1 | ForEach-Object { Write-Host $_ }
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    return [int]$exitCode
}

Add-Check "file:docker-compose.dev.yml" (Test-Path -LiteralPath $ComposeFile -PathType Leaf) $ComposeFile
Add-Check "file:env-example" (Test-Path -LiteralPath $ExampleEnvFile -PathType Leaf) $ExampleEnvFile
Add-Check "file:route-probe-script" (Test-Path -LiteralPath $ProbeScript -PathType Leaf) $ProbeScript

$envFileExists = Test-Path -LiteralPath $EnvFile -PathType Leaf
if (-not $envFileExists) {
    if ($DryRun -and $InitEnv) {
        Add-Check "env:file-would-create" $true "Would create ignored local env file from example: $EnvFile."
    } elseif ($InitEnv) {
        Copy-Item -LiteralPath $ExampleEnvFile -Destination $EnvFile -Force
        Add-Check "env:file-created" $true "Created ignored local env file from example: $EnvFile."
    } else {
        Add-Check "env:file-exists" $false "Missing $EnvFile. Re-run with -InitEnv or copy .env.codexplus-local.example first."
    }
} else {
    Add-Check "env:file-exists" $true $EnvFile
}

$env:BIND_HOST = $BindHost
$env:SUB2API_DEV_HOST_PORT = [string]$HostPort
$env:SUB2API_DEV_IMAGE = $ImageTag
$env:SUB2API_DEV_APP_CONTAINER = $AppContainerName
$env:SUB2API_DEV_POSTGRES_CONTAINER = $PostgresContainerName
$env:SUB2API_DEV_REDIS_CONTAINER = $RedisContainerName

if ($DryRun) {
    Add-Check "dry-run:compose-command" $true "Would run docker compose --env-file $EnvFile -p $ProjectName -f $ComposeFile up -d$(if ($SkipBuild) { '' } else { ' --build' }) and probe $BaseUrl."
    $results | Format-Table -AutoSize
    $failed = @($results | Where-Object { $_.Result -eq "FAIL" })
    if ($failed.Count -gt 0) {
        exit 1
    }
    Write-Host ""
    Write-Host "Dry run passed. No compose stack was started."
    exit 0
}

if (-not (Test-DockerAvailable)) {
    $results | Format-Table -AutoSize
    exit 1
}

$containersOk = $true
foreach ($name in @($AppContainerName, $PostgresContainerName, $RedisContainerName)) {
    if (-not (Test-Or-Remove-ContainerConflict $name)) {
        $containersOk = $false
    }
}
if (-not $containersOk) {
    $results | Format-Table -AutoSize
    Write-Host ""
    Write-Host "Isolated dev compose check failed before startup because one or more container names are already in use." -ForegroundColor Red
    exit 1
}

$configCode = Test-ComposeConfig
Add-Check "compose:config" ($configCode -eq 0) "docker compose config exit=$configCode."
if ($configCode -ne 0) {
    $results | Format-Table -AutoSize
    exit $configCode
}

if (-not $ProbeOnly) {
    $upArgs = @("up", "-d")
    if (-not $SkipBuild) {
        $upArgs += "--build"
    }
    $upCode = Invoke-Compose -Arguments $upArgs
    Add-Check "compose:up" ($upCode -eq 0) "docker compose $($upArgs -join ' ') exit=$upCode."
    if ($upCode -ne 0) {
        $results | Format-Table -AutoSize
        exit $upCode
    }
} else {
    Add-Check "compose:probe-only" $true "Skipped compose up; checking existing compose-owned containers and route probes."
}

$projectLabel = Get-ComposeLabel $AppContainerName "com.docker.compose.project"
$serviceLabel = Get-ComposeLabel $AppContainerName "com.docker.compose.service"
Add-Check "container:compose-project" ($projectLabel -eq $ProjectName) "Expected compose project $ProjectName; actual $projectLabel."
Add-Check "container:compose-service" ($serviceLabel -eq "sub2api") "Expected compose service sub2api; actual $serviceLabel."

$probeCode = Invoke-RouteProbe
Add-Check "probe:07-routes" ($probeCode -eq 0) "start-local-source-service.ps1 -ProbeOnly exit=$probeCode for $BaseUrl."

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "Isolated dev compose check failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Isolated local source compose stack is available at $BaseUrl."
Write-Host "This proves the current workspace source can run from docker-compose.dev.yml without replacing the upstream 8080 service. It is still prep/runtime evidence, not full 07 release evidence."
exit 0
