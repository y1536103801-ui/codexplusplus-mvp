param(
    [string]$Root,
    [string]$ImageTag = "sub2api-codexplus-local:dev",
    [string]$ExpectedImage = "",
    [string]$ContainerName = "sub2api-codexplus-local",
    [string]$SourceContainer = "sub2api",
    [string]$Network = "deploy_sub2api-network",
    [string]$BindHost = "127.0.0.1",
    [int]$HostPort = 8081,
    [switch]$SkipBuild,
    [switch]$ReplaceExisting,
    [switch]$ProbeOnly,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")
} else {
    $Root = Resolve-Path $Root
}

$Sub2ApiRoot = Join-Path $Root "sub2api-main"
$Dockerfile = Join-Path $Sub2ApiRoot "Dockerfile"
$Dockerignore = Join-Path $Sub2ApiRoot ".dockerignore"
$LegalDocGlob = "admin-compliance.*.md"
$LegalZh = Join-Path $Sub2ApiRoot "docs\legal\admin-compliance.zh.md"
$LegalEn = Join-Path $Sub2ApiRoot "docs\legal\admin-compliance.en.md"
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

function Test-RequiredFile {
    param(
        [string]$Name,
        [string]$Path
    )
    Add-Check $Name (Test-Path -LiteralPath $Path -PathType Leaf) $Path
}

function Invoke-DockerText {
    param([string[]]$Arguments)
    $output = & docker @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "docker $($Arguments -join ' ') failed with exit code $LASTEXITCODE. $output"
    }
    return $output
}

function Invoke-DockerCode {
    param([string[]]$Arguments)
    & docker @Arguments
    return $LASTEXITCODE
}

function Test-DockerAvailable {
    try {
        Invoke-DockerText -Arguments @("version", "--format", "{{.Server.Version}}") | Out-Null
        Add-Check "docker:available" $true "Docker daemon is reachable."
        return $true
    } catch {
        Add-Check "docker:available" $false "Docker daemon must be reachable to build or run the local source service."
        return $false
    }
}

function Get-ContainerName {
    param([string]$Name)
    $names = Invoke-DockerText -Arguments @("ps", "-a", "--filter", "name=^/$Name$", "--format", "{{.Names}}")
    return @($names | Where-Object { $_ -eq $Name })
}

function Get-RunningContainerName {
    param([string]$Name)
    $names = Invoke-DockerText -Arguments @("ps", "--filter", "name=^/$Name$", "--format", "{{.Names}}")
    return @($names | Where-Object { $_ -eq $Name })
}

function Get-ContainerImage {
    param([string]$Name)
    try {
        $images = Invoke-DockerText -Arguments @("inspect", $Name, "--format", "{{.Config.Image}}")
        return (($images | Select-Object -First 1) -as [string]).Trim()
    } catch {
        return ""
    }
}

function Invoke-EndpointProbe {
    param(
        [string]$Method,
        [string]$Url,
        [string]$Body = $null
    )

    $client = $null
    $content = $null
    try {
        Add-Type -AssemblyName System.Net.Http -ErrorAction SilentlyContinue | Out-Null
        $handler = New-Object System.Net.Http.HttpClientHandler
        $handler.AllowAutoRedirect = $false
        $client = New-Object System.Net.Http.HttpClient($handler)
        $client.Timeout = [TimeSpan]::FromSeconds(8)

        if ($Method -eq "GET") {
            $response = $client.GetAsync($Url).GetAwaiter().GetResult()
        } elseif ($Method -eq "POST") {
            if ($null -eq $Body) {
                $Body = ""
            }
            $content = New-Object System.Net.Http.StringContent -ArgumentList @($Body, [System.Text.Encoding]::UTF8, "application/json")
            $response = $client.PostAsync($Url, $content).GetAwaiter().GetResult()
        } else {
            return 0
        }

        return [int]$response.StatusCode
    } catch {
        return 0
    } finally {
        if ($null -ne $content) {
            $content.Dispose()
        }
        if ($null -ne $client) {
            $client.Dispose()
        }
    }
}

function Add-RouteProbeCheck {
    param(
        [string]$Name,
        [int]$Status,
        [int[]]$AllowedStatuses
    )

    $allowedText = ($AllowedStatuses | ForEach-Object { [string]$_ }) -join "/"
    Add-Check $Name ($AllowedStatuses -contains $Status) "HTTP $Status. Expected one of $allowedText; 404, 5xx or connection failure is not accepted."
}

function Invoke-Probes {
    $health = Invoke-EndpointProbe -Method "GET" -Url "$BaseUrl/health"
    Add-Check "probe:health" ($health -eq 200) "HTTP $health from $BaseUrl/health."

    $root = Invoke-EndpointProbe -Method "GET" -Url $BaseUrl
    Add-Check "probe:root-html" ($root -eq 200) "HTTP $root from $BaseUrl."

    $clientBootstrap = Invoke-EndpointProbe -Method "GET" -Url "$BaseUrl/api/v1/client/bootstrap"
    Add-RouteProbeCheck "probe:client-bootstrap-route" $clientBootstrap @(401, 403)

    $desktopPoll = Invoke-EndpointProbe -Method "POST" -Url "$BaseUrl/api/v1/auth/desktop/poll" -Body "{}"
    Add-RouteProbeCheck "probe:desktop-poll-route" $desktopPoll @(400, 401, 403)

    $adminConfig = Invoke-EndpointProbe -Method "GET" -Url "$BaseUrl/api/v1/admin/codex-plus/config"
    Add-RouteProbeCheck "probe:admin-codex-plus-config-route" $adminConfig @(401, 403, 423)

    $gatewayResponses = Invoke-EndpointProbe -Method "POST" -Url "$BaseUrl/v1/responses" -Body "{}"
    Add-RouteProbeCheck "probe:gateway-responses-route" $gatewayResponses @(400, 401, 403)
}

Test-RequiredFile "file:Dockerfile" $Dockerfile
Test-RequiredFile "file:.dockerignore" $Dockerignore
Test-RequiredFile "file:admin-compliance.zh.md" $LegalZh
Test-RequiredFile "file:admin-compliance.en.md" $LegalEn

if (Test-Path -LiteralPath $Dockerfile -PathType Leaf) {
    $dockerfileText = Get-Content -Raw -LiteralPath $Dockerfile
    Add-Check "dockerfile:legal-docs-copied" ($dockerfileText -match "COPY\s+docs/legal/admin-compliance\.\*\.md\s+/app/docs/legal/") "Frontend builder must receive raw legal markdown imports."
}

if (Test-Path -LiteralPath $Dockerignore -PathType Leaf) {
    $dockerignoreText = Get-Content -Raw -LiteralPath $Dockerignore
    Add-Check "dockerignore:legal-docs-unignored" ($dockerignoreText -match "!docs/legal/admin-compliance\.\*\.md") "Docker context must include frontend-imported legal markdown files."
}

if ($DryRun) {
    Add-Check "dry-run:local-source-service" $true "Would build $ImageTag from $Dockerfile and run $ContainerName at $BaseUrl without binding the existing /app/data directory."
    $results | Format-Table -AutoSize
    $failed = @($results | Where-Object { $_.Result -eq "FAIL" })
    if ($failed.Count -gt 0) {
        exit 1
    }
    Write-Host ""
    Write-Host "Dry run passed. No Docker build, container replace, or HTTP probe was executed."
    exit 0
}

if (-not (Test-DockerAvailable)) {
    $results | Format-Table -AutoSize
    exit 1
}

if ($ProbeOnly) {
    $runningForProbe = Get-RunningContainerName $ContainerName
    Add-Check "container:running-for-probe" ($runningForProbe.Count -gt 0) "Expected running container $ContainerName before probing $BaseUrl."
    if (-not [string]::IsNullOrWhiteSpace($ExpectedImage)) {
        $actualImage = Get-ContainerImage $ContainerName
        Add-Check "container:expected-image" ($actualImage -eq $ExpectedImage) "Expected image $ExpectedImage; actual image $actualImage."
    }
}

if (-not $ProbeOnly -and -not $SkipBuild) {
    Write-Host "Building local source image $ImageTag..."
    $buildCode = Invoke-DockerCode -Arguments @("build", "-t", $ImageTag, "-f", $Dockerfile, $Sub2ApiRoot)
    Add-Check "docker-build:image" ($buildCode -eq 0) "docker build exit=$buildCode for $ImageTag."
    if ($buildCode -ne 0) {
        $results | Format-Table -AutoSize
        exit $buildCode
    }
}

$existing = Get-ContainerName $ContainerName
if ($existing.Count -gt 0 -and $ReplaceExisting) {
    Write-Host "Removing existing container $ContainerName..."
    $removeCode = Invoke-DockerCode -Arguments @("rm", "-f", $ContainerName)
    Add-Check "container:removed-existing" ($removeCode -eq 0) "docker rm -f exit=$removeCode."
    if ($removeCode -ne 0) {
        $results | Format-Table -AutoSize
        exit $removeCode
    }
    $existing = @()
}

if (-not $ProbeOnly -and $existing.Count -eq 0) {
    $networkNames = Invoke-DockerText -Arguments @("network", "ls", "--format", "{{.Name}}")
    $hasNetwork = @($networkNames | Where-Object { $_ -eq $Network }).Count -gt 0
    Add-Check "docker-network:exists" $hasNetwork "Network $Network must exist so the local source container can reach postgres/redis."
    if (-not $hasNetwork) {
        $results | Format-Table -AutoSize
        exit 1
    }

    $sourceNames = Get-ContainerName $SourceContainer
    $hasSource = $sourceNames.Count -gt 0
    Add-Check "source-container:exists" $hasSource "Source container $SourceContainer provides database/redis environment values."
    if (-not $hasSource) {
        $results | Format-Table -AutoSize
        exit 1
    }

    $envJson = Invoke-DockerText -Arguments @("inspect", $SourceContainer, "--format", "{{json .Config.Env}}")
    $sourceEnv = @($envJson | ConvertFrom-Json)

    $runArgs = @("run", "-d", "--name", $ContainerName, "--network", $Network, "-p", "${BindHost}:$HostPort`:8080")
    foreach ($item in $sourceEnv) {
        $envItem = [string]$item
        if ($envItem -match "^SERVER_MODE=") {
            $envItem = "SERVER_MODE=debug"
        }
        $runArgs += @("-e", $envItem)
    }
    $runArgs += $ImageTag

    Write-Host "Starting local source container $ContainerName at $BaseUrl..."
    $containerId = Invoke-DockerText -Arguments $runArgs
    Add-Check "container:started" (-not [string]::IsNullOrWhiteSpace(($containerId -join ""))) "Started container $ContainerName from $ImageTag."
} elseif ($existing.Count -gt 0) {
    $running = Get-RunningContainerName $ContainerName
    if ($running.Count -eq 0 -and -not $ProbeOnly) {
        Write-Host "Starting existing container $ContainerName..."
        $startCode = Invoke-DockerCode -Arguments @("start", $ContainerName)
        Add-Check "container:started-existing" ($startCode -eq 0) "docker start exit=$startCode."
    } else {
        Add-Check "container:already-present" $true "Container $ContainerName already exists; use -ReplaceExisting to recreate it with a freshly built image."
    }
}

$deadline = (Get-Date).AddSeconds(90)
do {
    $healthStatus = Invoke-EndpointProbe -Method "GET" -Url "$BaseUrl/health"
    if ($healthStatus -eq 200) {
        break
    }
    Start-Sleep -Seconds 3
} while ((Get-Date) -lt $deadline)

Invoke-Probes

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "Local source service check failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Local source service is available at $BaseUrl."
Write-Host "This proves the current workspace source image exposes the 07 routes locally. It does not replace real E2E tokens, browser handoff, desktop launch, gateway request, package install, compatibility or Module J release evidence."
exit 0
