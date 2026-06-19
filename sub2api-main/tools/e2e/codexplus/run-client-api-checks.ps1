param(
    [string]$Root,
    [string]$EvidenceDir,
    [string]$EnvPrefix = "CODEXPLUS_07_E2E_",
    [string]$EnvFile,
    [switch]$FixtureMode,
    [switch]$SkipReadiness,
    [switch]$AllowProduction,
    [switch]$ProbeHttp,
    [switch]$EndpointPreflight,
    [switch]$SkipDeviceRegistration,
    [switch]$AllowRedeem
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")
} else {
    $Root = Resolve-Path $Root
}

$PlanRoot = Join-Path $Root "codex-plus-dev-plan"
$ContractRoot = Join-Path $Root "codex-plus-contracts"
$ReadinessScript = Join-Path $PlanRoot "tools\verify-07-e2e-readiness.ps1"
$FixtureRoot = Join-Path $ContractRoot "test-fixtures\client"

if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
    $EvidenceDir = Join-Path $PlanRoot ("test-runs\" + (Get-Date -Format "yyyyMMdd-HHmm") + "-e2e")
} elseif (-not [System.IO.Path]::IsPathRooted($EvidenceDir)) {
    $EvidenceDir = Join-Path $Root $EvidenceDir
}

$EvidencePath = [System.IO.Path]::GetFullPath($EvidenceDir)
New-Item -ItemType Directory -Force -Path $EvidencePath | Out-Null
$ScratchLogPath = Join-Path (Split-Path -Parent $EvidencePath) ("_scratch-logs\" + (Split-Path -Leaf $EvidencePath))
New-Item -ItemType Directory -Force -Path $ScratchLogPath | Out-Null

$results = New-Object System.Collections.Generic.List[object]
$observations = New-Object System.Collections.Generic.List[object]

function Add-Result {
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

function Add-Observation {
    param(
        [string]$Scenario,
        [string]$Method,
        [string]$Path,
        [int]$HttpStatus,
        [object]$Summary,
        [string]$Result,
        [string]$Note
    )
    $script:observations.Add([pscustomobject]@{
        Scenario = $Scenario
        Method = $Method
        Path = $Path
        HttpStatus = $HttpStatus
        ServiceStatus = $Summary.service_status
        EnvelopeStatus = $Summary.envelope_status
        Reason = $Summary.reason
        ErrorCode = $Summary.error_code
        SnapshotVersion = $Summary.snapshot_version
        Result = $Result
        Note = $Note
    })
}

function Get-EnvValue {
    param([string]$Suffix)
    return [Environment]::GetEnvironmentVariable($EnvPrefix + $Suffix, "Process")
}

function Import-EnvFile {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }

    $resolved = if ([System.IO.Path]::IsPathRooted($Path)) { $Path } else { Join-Path $Root $Path }
    $exists = Test-Path -LiteralPath $resolved -PathType Leaf
    Add-Result "env-file:exists" $exists "Env file path is validated; values are intentionally not printed."
    if (-not $exists) {
        return
    }

    $loaded = 0
    $lineNumber = 0
    foreach ($line in Get-Content -LiteralPath $resolved) {
        $lineNumber++
        if ($line -match "^\s*$" -or $line -match "^\s*#") {
            continue
        }

        $match = [regex]::Match($line, "^\s*\`$env:([A-Za-z_][A-Za-z0-9_]*)\s*=\s*'(.*)'\s*$")
        if (-not $match.Success) {
            Add-Result "env-file:line-$lineNumber" $false "Env file line $lineNumber is not a supported `$env:NAME = 'value' assignment."
            continue
        }

        $name = $match.Groups[1].Value
        $value = $match.Groups[2].Value -replace "''", "'"
        [Environment]::SetEnvironmentVariable($name, $value, "Process")
        $loaded++
    }
    Add-Result "env-file:loaded-values" ($loaded -gt 0) "$loaded env value(s) loaded; values intentionally not printed."
}

function Redact-Text {
    param([string]$Text)
    if ($null -eq $Text) {
        return ""
    }
    $redacted = $Text
    $redacted = $redacted -replace "(?i)\b(sk-[a-z0-9][a-z0-9_-]{10,}|sk-proj-[a-z0-9][a-z0-9_-]{10,}|sk-ant-[a-z0-9][a-z0-9_-]{10,})\b", "[redacted-api-key]"
    $redacted = $redacted -replace "(?i)\bAuthorization\s*:\s*(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]+", "Authorization: [redacted]"
    $redacted = $redacted -replace "\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b", "[redacted-jwt]"
    $redacted = $redacted -replace "(?i)(access_token|refresh_token|id_token|poll_token|session_token|api_key|upstream_key)(""|')?\s*:\s*(""|')[^""']+(""|')", '$1: "[redacted]"'
    return $redacted
}

function ConvertTo-SafeJson {
    param([object]$Value)
    return Redact-Text (ConvertTo-Json $Value -Depth 20)
}

function Join-Url {
    param(
        [string]$Base,
        [string]$Path
    )
    return $Base.TrimEnd("/") + $Path
}

function Get-ResponseSummary {
    param([object]$Json)
    $data = $Json.data
    $summary = [ordered]@{
        envelope_status = $Json.status
        reason = $Json.reason
        error_code = $Json.error_code
        service_status = $null
        snapshot_version = $null
        model_count = $null
        device_status = $null
    }

    if ($null -ne $data) {
        if ($null -ne $data.service -and $null -ne $data.service.status) {
            $summary.service_status = [string]$data.service.status
        } elseif ($null -ne $data.service_status) {
            $summary.service_status = [string]$data.service_status
        }

        if ($null -ne $data.snapshot_version) {
            $summary.snapshot_version = [string]$data.snapshot_version
        } elseif ($null -ne $data.service -and $null -ne $data.service.snapshot_version) {
            $summary.snapshot_version = [string]$data.service.snapshot_version
        }

        if ($null -ne $data.models) {
            $summary.model_count = @($data.models).Count
        }

        if ($null -ne $data.device -and $null -ne $data.device.status) {
            $summary.device_status = [string]$data.device.status
        } elseif ($null -ne $data.status -and $null -ne $data.device_id) {
            $summary.device_status = [string]$data.status
        }
    }

    return [pscustomobject]$summary
}

function Read-Fixture {
    param([string]$FileName)
    $path = Join-Path $FixtureRoot $FileName
    $json = Get-Content -Raw -LiteralPath $path | ConvertFrom-Json
    return @{
        StatusCode = 200
        Json = $json
        Text = ConvertTo-SafeJson $json
    }
}

function Invoke-JsonRequest {
    param(
        [string]$Method,
        [string]$Url,
        [string]$Token,
        [object]$Body = $null
    )

    $headers = @{
        Authorization = "Bearer $Token"
        "X-CodexPlus-Device-Id" = Get-EnvValue "TEST_DEVICE_ID"
        "X-CodexPlus-Client-Version" = "07-e2e-runner"
    }

    $parameters = @{
        Method = $Method
        Uri = $Url
        Headers = $headers
        TimeoutSec = 30
        UseBasicParsing = $true
    }

    if ($null -ne $Body) {
        $parameters.ContentType = "application/json"
        $parameters.Body = ConvertTo-Json $Body -Depth 12
    }

    try {
        $response = Invoke-WebRequest @parameters
        $statusCode = [int]$response.StatusCode
        $text = [string]$response.Content
    } catch {
        $statusCode = 0
        $text = $_.Exception.Message
        if ($null -ne $_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $text = $reader.ReadToEnd()
        }
    }

    $json = $null
    try {
        $json = $text | ConvertFrom-Json
    } catch {
        $json = [pscustomobject]@{
            status = "error"
            reason = "NON_JSON_RESPONSE"
            error_code = "NON_JSON_RESPONSE"
            data = $null
        }
    }

    return @{
        StatusCode = $statusCode
        Json = $json
        Text = Redact-Text $text
    }
}

function Write-MarkdownTable {
    param([System.Collections.IEnumerable]$Rows)
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("| Scenario | Method | Path | HTTP | Service status | Envelope | Reason | Error code | Snapshot | Result | Note |")
    $lines.Add("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
    foreach ($row in $Rows) {
        $lines.Add("| $($row.Scenario) | $($row.Method) | $($row.Path) | $($row.HttpStatus) | $($row.ServiceStatus) | $($row.EnvelopeStatus) | $($row.Reason) | $($row.ErrorCode) | $($row.SnapshotVersion) | $($row.Result) | $($row.Note) |")
    }
    return ($lines -join [Environment]::NewLine)
}

function Write-Evidence {
    param([bool]$Passed)

    $contractResult = if ($Passed) { "pass" } else { "fail" }
    $scopeResult = if ($Passed) { "pass" } else { "fail" }
    $fixtureNote = if ($FixtureMode) { "Fixture mode: true. This is tool self-test evidence only, not release evidence." } else { "Fixture mode: false. Calls were made against the configured test backend." }
    $table = Write-MarkdownTable $observations

    $contractContent = @"
# 02 Contract Checks

Run folder: $(Split-Path -Leaf $EvidencePath)
Status: executed
Result: $contractResult

## Scope

$fixtureNote

Client API contract checks covered bootstrap, usage, devices, and redeem path presence from `codex-plus-contracts/api/client-openapi.yaml`.

## Paths Checked

- `/api/v1/client/bootstrap`
- `/api/v1/client/usage`
- `/api/v1/client/devices`
- `/api/v1/client/redeem`

## Result Summary

$table
"@

    $clientContent = @"
# 04 Client API E2E

Run folder: $(Split-Path -Leaf $EvidencePath)
Status: executed
Result: $scopeResult

## Scope

$fixtureNote

Scope result for this client API subset: $scopeResult

This runner covers bootstrap, usage, and idempotent test-device refresh. Browser handoff start, complete, desktop poll completion, desktop Manager login, Codex launch, gateway model request and package install are outside this runner and must be supplied by the broader Module I evidence flow.

## Client API Observations

$table
"@

    $auditContent = @"
# 09 Usage Events Audit

Run folder: $(Split-Path -Leaf $EvidencePath)
Status: executed
Result: $scopeResult

## Scope

$fixtureNote

Scope result for usage endpoint observation: $scopeResult

This runner records usage endpoint response shape and redaction behavior only. Admin audit/event rows for success and rejection paths are not covered by this runner and must be captured before final release evidence can pass.

## Usage And Redaction Observations

$table

## Redaction

Token values, Authorization headers, user-side API Keys, upstream provider Keys, poll tokens and session tokens are intentionally not printed.
"@

    $defectContent = @"
# 11 Defects

## Client API Runner Findings

- P0: none recorded by this runner.
- P1: broader E2E evidence still must cover browser handoff completion, gateway enforcement, desktop launch, package install, compatibility migration and admin audit/event visibility.
- P2: review any failed client API subset rows above before using this evidence in Module J.
- P3: none recorded by this runner.
"@

    Set-Content -LiteralPath (Join-Path $EvidencePath "02-contract-checks.md") -Encoding UTF8 -Value $contractContent
    Set-Content -LiteralPath (Join-Path $EvidencePath "04-client-api-e2e.md") -Encoding UTF8 -Value $clientContent
    Set-Content -LiteralPath (Join-Path $EvidencePath "09-usage-events-audit.md") -Encoding UTF8 -Value $auditContent
    Set-Content -LiteralPath (Join-Path $EvidencePath "11-defects.md") -Encoding UTF8 -Value $defectContent
}

Import-EnvFile $EnvFile

foreach ($file in @(
    "api\client-openapi.yaml",
    "test-fixtures\client\bootstrap.available.json",
    "test-fixtures\client\bootstrap.not_purchased.json",
    "test-fixtures\client\bootstrap.expired.json",
    "test-fixtures\client\bootstrap.low_balance.json",
    "test-fixtures\client\bootstrap.device_revoked.json",
    "test-fixtures\client\usage.available.json",
    "test-fixtures\client\devices.registered.json"
)) {
    $path = Join-Path $ContractRoot $file
    Add-Result "contract-file:$file" (Test-Path -LiteralPath $path -PathType Leaf) $file
}

$openApiPath = Join-Path $ContractRoot "api\client-openapi.yaml"
if (Test-Path -LiteralPath $openApiPath -PathType Leaf) {
    $openApiText = Get-Content -Raw -LiteralPath $openApiPath
    foreach ($path in @("/api/v1/client/bootstrap", "/api/v1/client/usage", "/api/v1/client/devices", "/api/v1/client/redeem")) {
        Add-Result "contract-path:$path" ($openApiText -match [regex]::Escape($path)) $path
    }
}

$readinessPassed = $true
if (-not $FixtureMode -and -not $SkipReadiness) {
    $readinessArgs = @("-Root", $Root, "-EnvPrefix", $EnvPrefix)
    if ($EnvFile) { $readinessArgs += @("-EnvFile", $EnvFile) }
    if ($AllowProduction) { $readinessArgs += "-AllowProduction" }
    if ($ProbeHttp) { $readinessArgs += "-ProbeHttp" }
    if ($EndpointPreflight) { $readinessArgs += "-EndpointPreflight" }
    $readinessLogPath = Join-Path $ScratchLogPath "client-api-readiness.log"
    & powershell -NoProfile -ExecutionPolicy Bypass -File $ReadinessScript @readinessArgs *> $readinessLogPath
    $readinessPassed = ($LASTEXITCODE -eq 0)
    Add-Result "readiness-verifier" $readinessPassed "verify-07-e2e-readiness.ps1 exit=$LASTEXITCODE; log stored outside release evidence at _scratch-logs."
}

$personas = @(
    @{ Name = "user_active"; Suffix = "USER_ACTIVE_TOKEN"; Expected = "available"; Fixture = "bootstrap.available.json" },
    @{ Name = "user_not_purchased"; Suffix = "USER_NOT_PURCHASED_TOKEN"; Expected = "not_purchased"; Fixture = "bootstrap.not_purchased.json" },
    @{ Name = "user_expired"; Suffix = "USER_EXPIRED_TOKEN"; Expected = "expired"; Fixture = "bootstrap.expired.json" },
    @{ Name = "user_low_balance"; Suffix = "USER_LOW_BALANCE_TOKEN"; Expected = "low_balance"; Fixture = "bootstrap.low_balance.json" },
    @{ Name = "user_device_revoked"; Suffix = "USER_DEVICE_REVOKED_TOKEN"; Expected = "device_revoked"; Fixture = "bootstrap.device_revoked.json" },
    @{ Name = "user_model_denied"; Suffix = "USER_MODEL_DENIED_TOKEN"; Expected = ""; Fixture = "bootstrap.available.json" }
)

$backendBase = Get-EnvValue "BACKEND_BASE_URL"

if (-not $FixtureMode -and -not $readinessPassed) {
    $summary = [pscustomobject]@{
        envelope_status = "not-run"
        reason = "READINESS_FAILED"
        error_code = "READINESS_FAILED"
        service_status = "not-run"
        snapshot_version = ""
    }
    Add-Observation "readiness" "PRECHECK" "verify-07-e2e-readiness.ps1" 0 $summary "fail" "Client API HTTP checks were not run because readiness failed."
    Write-Evidence $false
    $results | Format-Table -AutoSize
    Write-Host ""
    Write-Host "07 client API checks failed: readiness failed before HTTP execution." -ForegroundColor Red
    Write-Host "Evidence written to: $EvidencePath" -ForegroundColor Yellow
    exit 1
}

if (-not $FixtureMode -and [string]::IsNullOrWhiteSpace($backendBase)) {
    Add-Result "env:BACKEND_BASE_URL" $false "$($EnvPrefix)BACKEND_BASE_URL is required unless -FixtureMode is used."
    $summary = [pscustomobject]@{
        envelope_status = "not-run"
        reason = "BACKEND_BASE_URL_MISSING"
        error_code = "BACKEND_BASE_URL_MISSING"
        service_status = "not-run"
        snapshot_version = ""
    }
    Add-Observation "readiness" "PRECHECK" "BACKEND_BASE_URL" 0 $summary "fail" "Client API HTTP checks were not run because backend URL is missing."
    Write-Evidence $false
    $results | Format-Table -AutoSize
    Write-Host ""
    Write-Host "07 client API checks failed: backend base URL missing." -ForegroundColor Red
    Write-Host "Evidence written to: $EvidencePath" -ForegroundColor Yellow
    exit 1
}

foreach ($persona in $personas) {
    if ($FixtureMode) {
        $response = Read-Fixture $persona.Fixture
    } else {
        $token = Get-EnvValue $persona.Suffix
        $response = Invoke-JsonRequest "GET" (Join-Url $backendBase "/api/v1/client/bootstrap") $token
    }
    $summary = Get-ResponseSummary $response.Json
    $expectedOk = [string]::IsNullOrWhiteSpace($persona.Expected) -or ($summary.service_status -eq $persona.Expected)
    $passed = ($response.StatusCode -eq 200 -and $expectedOk)
    $observationResult = if ($passed) { "pass" } else { "fail" }
    Add-Result "bootstrap:$($persona.Name)" $passed "http=$($response.StatusCode); service_status=$($summary.service_status); expected=$($persona.Expected)"
    Add-Observation $persona.Name "GET" "/api/v1/client/bootstrap" $response.StatusCode $summary $observationResult "Token value intentionally not printed."
}

if ($FixtureMode) {
    $usageResponse = Read-Fixture "usage.available.json"
} else {
    $usageResponse = Invoke-JsonRequest "GET" (Join-Url $backendBase "/api/v1/client/usage") (Get-EnvValue "USER_ACTIVE_TOKEN")
}
$usageSummary = Get-ResponseSummary $usageResponse.Json
$usagePassed = ($usageResponse.StatusCode -eq 200 -and -not [string]::IsNullOrWhiteSpace($usageSummary.service_status))
$usageObservationResult = if ($usagePassed) { "pass" } else { "fail" }
Add-Result "usage:user_active" $usagePassed "http=$($usageResponse.StatusCode); service_status=$($usageSummary.service_status)"
Add-Observation "user_active_usage" "GET" "/api/v1/client/usage" $usageResponse.StatusCode $usageSummary $usageObservationResult "Usage response only; admin audit/event rows are separate evidence."

if (-not $SkipDeviceRegistration) {
    if ($FixtureMode) {
        $deviceResponse = Read-Fixture "devices.registered.json"
    } else {
        $body = @{
            device_id = Get-EnvValue "TEST_DEVICE_ID"
            platform = "windows"
            app_version = "07-e2e-runner"
            codex_version = "test-codex-version"
            last_seen_at = (Get-Date).ToUniversalTime().ToString("o")
        }
        $deviceResponse = Invoke-JsonRequest "POST" (Join-Url $backendBase "/api/v1/client/devices") (Get-EnvValue "USER_ACTIVE_TOKEN") $body
    }
    $deviceSummary = Get-ResponseSummary $deviceResponse.Json
    $devicePassed = ($deviceResponse.StatusCode -eq 200 -and -not [string]::IsNullOrWhiteSpace($deviceSummary.device_status))
    $deviceObservationResult = if ($devicePassed) { "pass" } else { "fail" }
    Add-Result "device-register:user_active" $devicePassed "http=$($deviceResponse.StatusCode); device_status=$($deviceSummary.device_status)"
    Add-Observation "user_active_device" "POST" "/api/v1/client/devices" $deviceResponse.StatusCode $deviceSummary $deviceObservationResult "Idempotent test-device refresh."
}

if ($AllowRedeem) {
    $redeemCode = Get-EnvValue "REDEEM_CODE"
    if ([string]::IsNullOrWhiteSpace($redeemCode)) {
        Add-Result "redeem:code-present" $false "Set $($EnvPrefix)REDEEM_CODE before using -AllowRedeem."
    } else {
        if ($FixtureMode) {
            $redeemResponse = Read-Fixture "redeem.applied.json"
        } else {
            $body = @{
                code = $redeemCode
                device_id = Get-EnvValue "TEST_DEVICE_ID"
            }
            $redeemResponse = Invoke-JsonRequest "POST" (Join-Url $backendBase "/api/v1/client/redeem") (Get-EnvValue "USER_ACTIVE_TOKEN") $body
        }
        $redeemSummary = Get-ResponseSummary $redeemResponse.Json
        $redeemPassed = ($redeemResponse.StatusCode -eq 200)
        $redeemObservationResult = if ($redeemPassed) { "pass" } else { "fail" }
        Add-Result "redeem:user_active" $redeemPassed "http=$($redeemResponse.StatusCode); redeem result redacted."
        Add-Observation "user_active_redeem" "POST" "/api/v1/client/redeem" $redeemResponse.StatusCode $redeemSummary $redeemObservationResult "Redeem is opt-in; code value intentionally not printed."
    }
} else {
    Add-Result "redeem:skipped-by-default" $true "Redeem is skipped unless -AllowRedeem is supplied."
}

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
Write-Evidence ($failed.Count -eq 0)

$results | Format-Table -AutoSize

if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 client API checks failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    Write-Host "Evidence written to: $EvidencePath" -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Host "07 client API checks passed."
Write-Host "Evidence written to: $EvidencePath"
if ($FixtureMode) {
    Write-Host "Fixture mode output is for tooling self-test only, not release evidence." -ForegroundColor Yellow
}
exit 0
