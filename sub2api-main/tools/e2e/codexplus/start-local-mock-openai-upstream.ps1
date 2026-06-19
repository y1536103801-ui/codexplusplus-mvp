param(
    [string]$HostName = "0.0.0.0",
    [int]$Port = 18081,
    [int]$ProbeTimeoutSeconds = 10,
    [switch]$ReplaceExisting,
    [switch]$ProbeOnly,
    [switch]$Stop
)

$ErrorActionPreference = "Stop"

$ScriptPath = Join-Path $PSScriptRoot "mock-openai-upstream.js"
$PidFile = Join-Path $PSScriptRoot ".mock-openai-upstream.pid"
$OutLog = Join-Path $PSScriptRoot ".mock-openai-upstream.log"
$ErrLog = Join-Path $PSScriptRoot ".mock-openai-upstream.err.log"
$HealthUrl = "http://127.0.0.1:$Port/health"

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

function Get-ExistingPid {
    if (-not (Test-Path -LiteralPath $PidFile -PathType Leaf)) {
        return $null
    }
    $raw = (Get-Content -LiteralPath $PidFile -Encoding UTF8 | Select-Object -First 1)
    if ($raw -match "^\d+$") {
        return [int]$raw
    }
    return $null
}

function Test-ProcessAlive {
    param([Nullable[int]]$ProcessId)
    if ($null -eq $ProcessId) {
        return $false
    }
    return $null -ne (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)
}

function Stop-ExistingProcess {
    param([Nullable[int]]$ProcessId)
    if (Test-ProcessAlive $ProcessId) {
        Stop-Process -Id $ProcessId -Force -ErrorAction Stop
        Start-Sleep -Milliseconds 250
    }
    if (Test-Path -LiteralPath $PidFile -PathType Leaf) {
        Remove-Item -LiteralPath $PidFile -Force
    }
}

function Invoke-HealthProbe {
    try {
        $response = Invoke-WebRequest -Method Get -Uri $HealthUrl -TimeoutSec 2 -UseBasicParsing
        return [int]$response.StatusCode
    } catch {
        if ($_.Exception.Response) {
            return [int]$_.Exception.Response.StatusCode
        }
        return 0
    }
}

function Wait-Healthy {
    $deadline = [DateTimeOffset]::UtcNow.AddSeconds($ProbeTimeoutSeconds)
    do {
        $status = Invoke-HealthProbe
        if ($status -eq 200) {
            return $true
        }
        Start-Sleep -Milliseconds 300
    } while ([DateTimeOffset]::UtcNow -lt $deadline)
    return $false
}

Add-Check "file:mock-openai-upstream-js" (Test-Path -LiteralPath $ScriptPath -PathType Leaf) $ScriptPath
$nodeCommand = Get-Command node -ErrorAction SilentlyContinue
Add-Check "node:available" ($null -ne $nodeCommand) "node must be available on PATH."

$existingPid = Get-ExistingPid
$existingAlive = Test-ProcessAlive $existingPid
Add-Check "pid-file:existing-process" ($null -eq $existingPid -or $existingAlive) "Existing PID is either absent or points to a running process."

if ($Stop) {
    Stop-ExistingProcess $existingPid
    Add-Check "mock-openai:stopped" $true "Stopped local mock OpenAI upstream if it was running."
    $results | Format-Table -AutoSize
    exit 0
}

if ($ProbeOnly) {
    $status = Invoke-HealthProbe
    Add-Check "mock-openai:health" ($status -eq 200) "HTTP $status from $HealthUrl."
    $results | Format-Table -AutoSize
    $failed = @($results | Where-Object { $_.Result -eq "FAIL" })
    if ($failed.Count -gt 0) { exit 1 }
    Write-Host ""
    Write-Host "Local mock OpenAI upstream is healthy at $HealthUrl."
    exit 0
}

if ($existingAlive) {
    $status = Invoke-HealthProbe
    if ($status -eq 200 -and -not $ReplaceExisting) {
        Add-Check "mock-openai:already-running" $true "Existing process $existingPid is healthy at $HealthUrl."
        $results | Format-Table -AutoSize
        Write-Host ""
        Write-Host "Local mock OpenAI upstream already running at $HealthUrl."
        exit 0
    }
    if (-not $ReplaceExisting) {
        Add-Check "mock-openai:replace-required" $false "Existing process $existingPid is not healthy. Re-run with -ReplaceExisting."
        $results | Format-Table -AutoSize
        exit 1
    }
    Stop-ExistingProcess $existingPid
    Add-Check "mock-openai:replaced-existing" $true "Stopped existing process before restart."
}

if (-not (Test-Path -LiteralPath $ScriptPath -PathType Leaf) -or $null -eq $nodeCommand) {
    $results | Format-Table -AutoSize
    exit 1
}

$process = Start-Process `
    -FilePath $nodeCommand.Source `
    -ArgumentList @($ScriptPath, "--host", $HostName, "--port", [string]$Port) `
    -PassThru `
    -WindowStyle Hidden `
    -RedirectStandardOutput $OutLog `
    -RedirectStandardError $ErrLog

Set-Content -LiteralPath $PidFile -Encoding UTF8 -Value ([string]$process.Id)
Add-Check "mock-openai:started" (Test-ProcessAlive $process.Id) "Started process $($process.Id); logs are written beside the script."

$healthy = Wait-Healthy
Add-Check "mock-openai:health" $healthy "Probe $HealthUrl returned healthy=$healthy."

$results | Format-Table -AutoSize
$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    exit 1
}

Write-Host ""
Write-Host "Local mock OpenAI upstream is healthy at $HealthUrl."
exit 0
