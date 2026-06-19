param(
    [string]$Root,
    [string]$EvidenceDir,
    [string]$EnvPrefix = "CODEXPLUS_07_E2E_",
    [string]$EnvFile,
    [switch]$FixtureMode,
    [switch]$AllowProduction,
    [switch]$AllowSessionStart,
    [switch]$AllowBrowserComplete,
    [switch]$AllowPartial,
    [string]$DeviceName = "Codex++ E2E desktop"
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")
} else {
    $Root = Resolve-Path $Root
}

$PlanRoot = Join-Path $Root "codex-plus-dev-plan"
$ContractRoot = Join-Path $Root "codex-plus-contracts"
$FixtureRoot = Join-Path $ContractRoot "test-fixtures\client"

if ([string]::IsNullOrWhiteSpace($EvidenceDir)) {
    $EvidenceDir = Join-Path $PlanRoot ("test-runs\" + (Get-Date -Format "yyyyMMdd-HHmm") + "-e2e")
} elseif (-not [System.IO.Path]::IsPathRooted($EvidenceDir)) {
    $EvidenceDir = Join-Path $Root $EvidenceDir
}

$EvidencePath = [System.IO.Path]::GetFullPath($EvidenceDir)
New-Item -ItemType Directory -Force -Path $EvidencePath | Out-Null

$results = New-Object System.Collections.Generic.List[object]
$observations = New-Object System.Collections.Generic.List[object]

function Add-Result {
    param([string]$Name, [bool]$Passed, [string]$Detail)
    $script:results.Add([pscustomobject]@{
        Check = $Name
        Result = if ($Passed) { "PASS" } else { "FAIL" }
        Detail = $Detail
    })
}

function Add-Observation {
    param(
        [string]$Step,
        [int]$HttpStatus,
        [string]$Status,
        [string]$Result,
        [string]$Note
    )
    $script:observations.Add([pscustomobject]@{
        Step = $Step
        HttpStatus = $HttpStatus
        Status = $Status
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
    $redacted = $redacted -replace "(?i)(access_token|refresh_token|poll_token|session_token)=((?!redacted)[A-Za-z0-9._~%+-]+)", '$1=[redacted]'
    return $redacted
}

function Join-Url {
    param([string]$Base, [string]$Path)
    return $Base.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Test-SafeUrl {
    param([string]$Value)
    try {
        $uri = [System.Uri]$Value
    } catch {
        return $false
    }
    if ($uri.Scheme -notin @("http", "https")) {
        return $false
    }
    $uriHost = $uri.Host.ToLowerInvariant()
    if ($uriHost -in @("localhost", "127.0.0.1", "::1")) {
        return $true
    }
    return ($uriHost -match "(^|[.-])(dev|test|staging|stage|sandbox|qa|local)([.-]|$)")
}

function Read-Fixture {
    param([string]$FileName)
    $path = Join-Path $FixtureRoot $FileName
    $json = Get-Content -Raw -LiteralPath $path | ConvertFrom-Json
    return @{
        StatusCode = 200
        Json = $json
        Text = Redact-Text (ConvertTo-Json $json -Depth 20)
    }
}

function Invoke-JsonRequest {
    param(
        [string]$Method,
        [string]$Url,
        [object]$Body = $null,
        [string]$BearerToken = ""
    )

    $headers = @{
        "Content-Type" = "application/json"
    }
    if (-not [string]::IsNullOrWhiteSpace($BearerToken)) {
        $headers.Authorization = "Bearer $BearerToken"
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
    $lines.Add("| Step | HTTP | Status | Result | Note |")
    $lines.Add("| --- | --- | --- | --- | --- |")
    foreach ($row in $Rows) {
        $lines.Add("| $($row.Step) | $($row.HttpStatus) | $($row.Status) | $($row.Result) | $($row.Note) |")
    }
    return ($lines -join [Environment]::NewLine)
}

function Append-Or-CreateEvidence {
    param(
        [string]$Name,
        [string]$Title,
        [string]$Content
    )
    $path = Join-Path $EvidencePath $Name
    if (Test-Path -LiteralPath $path -PathType Leaf) {
        Add-Content -LiteralPath $path -Encoding UTF8 -Value ([Environment]::NewLine + $Content)
    } else {
        $full = @"
# $Title

Run folder: $(Split-Path -Leaf $EvidencePath)
Status: executed
$Content
"@
        Set-Content -LiteralPath $path -Encoding UTF8 -Value $full
    }
}

function Write-Evidence {
    param([bool]$Passed)
    $result = if ($Passed) { "pass" } else { "fail" }
    $fixtureNote = if ($FixtureMode) { "Fixture mode: true. This is tool self-test evidence only, not release evidence." } else { "Fixture mode: false. Real desktop handoff requests are gated by -AllowSessionStart and -AllowBrowserComplete." }
    $table = Write-MarkdownTable $observations

    $contractContent = @"

## Browser Handoff Contract Checks

Result: $result

$fixtureNote

Paths checked:
- `/api/v1/auth/desktop/start`
- `/api/v1/auth/desktop/complete`
- `/api/v1/auth/desktop/poll`

Safety checks:
- poll_token not in authorize_url
- verification_code is 6 digit code
- complete response never returns a desktop access token
- completed desktop poll token values are redacted from evidence
"@

    $clientContent = @"

## Browser Handoff E2E Subset

Browser handoff subset result: $result

$fixtureNote

This runner covers desktop start, pre-complete desktop poll, authenticated browser complete, and completed desktop poll. It does not validate Manager UI rendering, Turnstile UI completion, provider write, Codex launch, package install, compatibility migration or payment.

Paths exercised:
- `/api/v1/auth/desktop/start`
- `/api/v1/auth/desktop/complete`
- `/api/v1/auth/desktop/poll`

Safety checks:
- poll_token not in authorize_url
- verification_code is 6 digit code
- complete response never returns a desktop access token
- completed desktop poll token values are redacted from evidence

## Browser Handoff Observations

$table

## Redaction

Session token, poll token, browser JWT, desktop access token, refresh token and Authorization headers are intentionally not printed. The authorize URL is recorded only after query-secret redaction.
"@

    $defectContent = @"

## Browser Handoff Runner Findings

- P0: review any browser handoff row marked fail before release.
- P1: real browser Turnstile/2FA confirmation and Manager UI evidence remain required outside fixture mode.
- P2: this runner records token presence and flow status only; token values are intentionally omitted.
- P3: none recorded by this runner.
"@

    Append-Or-CreateEvidence "02-contract-checks.md" "02 Contract Checks" $contractContent
    Append-Or-CreateEvidence "04-client-api-e2e.md" "04 Client API E2E" $clientContent
    Append-Or-CreateEvidence "11-defects.md" "11 Defects" $defectContent
}

Import-EnvFile $EnvFile

foreach ($file in @(
    "api\client-openapi.yaml",
    "test-fixtures\client\desktop-handoff.start.json",
    "test-fixtures\client\desktop-handoff.complete.json",
    "test-fixtures\client\desktop-handoff.poll.pending.json",
    "test-fixtures\client\desktop-handoff.poll.completed.json"
)) {
    $path = Join-Path $ContractRoot $file
    Add-Result "contract-file:$file" (Test-Path -LiteralPath $path -PathType Leaf) $file
}

$openApiPath = Join-Path $ContractRoot "api\client-openapi.yaml"
if (Test-Path -LiteralPath $openApiPath -PathType Leaf) {
    $openApiText = Get-Content -Raw -LiteralPath $openApiPath
    foreach ($path in @("/api/v1/auth/desktop/start", "/api/v1/auth/desktop/complete", "/api/v1/auth/desktop/poll")) {
        Add-Result "contract-path:$path" ($openApiText -match [regex]::Escape($path)) $path
    }
}

$backendBase = Get-EnvValue "BACKEND_BASE_URL"
$deviceId = Get-EnvValue "TEST_DEVICE_ID"
$browserToken = Get-EnvValue "BROWSER_AUTH_TOKEN"
$sessionToken = ""
$pollToken = ""

if ($FixtureMode) {
    $startResponse = Read-Fixture "desktop-handoff.start.json"
} elseif (-not $AllowSessionStart) {
    Add-Result "browser-handoff-session-start-opt-in" $false "Supply -AllowSessionStart only in an approved test environment; POST /desktop/start creates a pending browser session."
    Add-Observation "desktop-start" 0 "not-run" "fail" "Session start was not run because -AllowSessionStart was not supplied."
} else {
    Add-Result "env:BACKEND_BASE_URL" (-not [string]::IsNullOrWhiteSpace($backendBase)) "$($EnvPrefix)BACKEND_BASE_URL must be set."
    Add-Result "env:BACKEND_BASE_URL-safe" ($AllowProduction -or (Test-SafeUrl $backendBase)) "$($EnvPrefix)BACKEND_BASE_URL must be local/dev/test/staging/sandbox/qa unless -AllowProduction is supplied."
    Add-Result "env:TEST_DEVICE_ID" (-not [string]::IsNullOrWhiteSpace($deviceId)) "$($EnvPrefix)TEST_DEVICE_ID must be set."
    if (-not [string]::IsNullOrWhiteSpace($backendBase)) {
        $startBody = @{
            device_id = $deviceId
            device_name = $DeviceName
        }
        $startResponse = Invoke-JsonRequest "POST" (Join-Url $backendBase "/api/v1/auth/desktop/start") $startBody
    }
}

if ($null -ne $startResponse) {
    $startData = $startResponse.Json.data
    if ($null -ne $startData) {
        $sessionToken = [string]$startData.session_token
        $pollToken = [string]$startData.poll_token
        $authorizeUrl = [string]$startData.authorize_url
        $verificationCode = [string]$startData.verification_code
        $authorizeUrlHasPollToken = ($authorizeUrl -match "(?i)(^|[?&])poll_token=")
        $authorizeUrlHasSessionToken = ($authorizeUrl -match "(?i)(^|[?&])session_token=")
        $verificationCodeOk = ($verificationCode -match "^\d{6}$")
        $passed = ($startResponse.StatusCode -eq 200 -and -not [string]::IsNullOrWhiteSpace($sessionToken) -and -not [string]::IsNullOrWhiteSpace($pollToken) -and -not $authorizeUrlHasPollToken -and $authorizeUrlHasSessionToken -and $verificationCodeOk)
        $observationResult = if ($passed) { "pass" } else { "fail" }
        Add-Result "browser-handoff:start" $passed "http=$($startResponse.StatusCode); authorize_url redacted=$(Redact-Text $authorizeUrl); poll_token not in authorize_url=$(-not $authorizeUrlHasPollToken); verification_code_6_digit=$verificationCodeOk"
        Add-Observation "desktop-start" $startResponse.StatusCode "session-created" $observationResult "authorize_url query secrets redacted; poll_token not in authorize_url=$(-not $authorizeUrlHasPollToken)."
    } else {
        Add-Result "browser-handoff:start" $false "No data object returned by desktop start."
        Add-Observation "desktop-start" $startResponse.StatusCode "missing-data" "fail" "No token values printed."
    }
}

if (-not [string]::IsNullOrWhiteSpace($sessionToken) -and -not [string]::IsNullOrWhiteSpace($pollToken)) {
    if ($FixtureMode) {
        $pendingResponse = Read-Fixture "desktop-handoff.poll.pending.json"
    } else {
        $pendingResponse = Invoke-JsonRequest "POST" (Join-Url $backendBase "/api/v1/auth/desktop/poll") @{
            session_token = $sessionToken
            poll_token = $pollToken
        }
    }
    $pendingStatus = [string]$pendingResponse.Json.data.status
    $pendingPassed = ($pendingResponse.StatusCode -eq 200 -and $pendingStatus -eq "pending")
    $pendingObservationResult = if ($pendingPassed) { "pass" } else { "fail" }
    Add-Result "browser-handoff:pending-poll" $pendingPassed "http=$($pendingResponse.StatusCode); status=$pendingStatus"
    $pendingObservationStatus = if ($pendingStatus -eq "pending") { "pre-complete" } else { $pendingStatus }
    Add-Observation "desktop-poll-before-complete" $pendingResponse.StatusCode $pendingObservationStatus $pendingObservationResult "Pre-complete poll returned no desktop tokens."
}

if ($FixtureMode) {
    $completeResponse = Read-Fixture "desktop-handoff.complete.json"
} elseif ($AllowBrowserComplete) {
    if ([string]::IsNullOrWhiteSpace($browserToken)) {
        Add-Result "env:BROWSER_AUTH_TOKEN" $false "$($EnvPrefix)BROWSER_AUTH_TOKEN must be set when -AllowBrowserComplete is supplied; value intentionally not printed."
        Add-Observation "browser-complete" 0 "missing-browser-token" "fail" "Browser JWT value intentionally not printed."
    } elseif ([string]::IsNullOrWhiteSpace($sessionToken)) {
        Add-Result "browser-handoff:complete" $false "Session token was not available from desktop start."
        Add-Observation "browser-complete" 0 "missing-session" "fail" "No token values printed."
    } else {
        $completeResponse = Invoke-JsonRequest "POST" (Join-Url $backendBase "/api/v1/auth/desktop/complete") @{
            session_token = $sessionToken
        } $browserToken
    }
} else {
    Add-Result "browser-handoff-complete-opt-in" $false "Supply -AllowBrowserComplete with $($EnvPrefix)BROWSER_AUTH_TOKEN after a real browser login to approve the pending session."
    Add-Observation "browser-complete" 0 "not-run" "fail" "Browser complete was not run because -AllowBrowserComplete was not supplied."
}

if ($null -ne $completeResponse) {
    $completeStatus = [string]$completeResponse.Json.data.status
    $completeHasToken = ($completeResponse.Text -match "(?i)access_token|refresh_token")
    $completePassed = ($completeResponse.StatusCode -eq 200 -and $completeStatus -eq "completed" -and -not $completeHasToken)
    $completeObservationResult = if ($completePassed) { "pass" } else { "fail" }
    Add-Result "browser-handoff:complete" $completePassed "http=$($completeResponse.StatusCode); status=$completeStatus; complete response token fields present=$completeHasToken"
    Add-Observation "browser-complete" $completeResponse.StatusCode $completeStatus $completeObservationResult "Complete response must not return desktop tokens."
}

if (($FixtureMode -or $AllowBrowserComplete) -and -not [string]::IsNullOrWhiteSpace($sessionToken) -and -not [string]::IsNullOrWhiteSpace($pollToken)) {
    if ($FixtureMode) {
        $completedPollResponse = Read-Fixture "desktop-handoff.poll.completed.json"
    } else {
        $completedPollResponse = Invoke-JsonRequest "POST" (Join-Url $backendBase "/api/v1/auth/desktop/poll") @{
            session_token = $sessionToken
            poll_token = $pollToken
        }
    }
    $completedStatus = [string]$completedPollResponse.Json.data.status
    $hasAccessToken = $null -ne $completedPollResponse.Json.data.access_token -and -not [string]::IsNullOrWhiteSpace([string]$completedPollResponse.Json.data.access_token)
    $hasRefreshToken = $null -ne $completedPollResponse.Json.data.refresh_token -and -not [string]::IsNullOrWhiteSpace([string]$completedPollResponse.Json.data.refresh_token)
    $completedPassed = ($completedPollResponse.StatusCode -eq 200 -and $completedStatus -eq "completed" -and $hasAccessToken -and $hasRefreshToken)
    $completedObservationResult = if ($completedPassed) { "pass" } else { "fail" }
    Add-Result "browser-handoff:completed-poll" $completedPassed "http=$($completedPollResponse.StatusCode); status=$completedStatus; access token present=$hasAccessToken; refresh token present=$hasRefreshToken; values redacted."
    Add-Observation "desktop-poll-after-complete" $completedPollResponse.StatusCode $completedStatus $completedObservationResult "Desktop tokens were present only in completed poll and values were redacted."
}

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
$evidencePassed = ($failed.Count -eq 0)
$passedForExit = if ($AllowPartial) {
    $criticalFailures = @($failed | Where-Object { $_.Check -notin @("browser-handoff-complete-opt-in") })
    $criticalFailures.Count -eq 0
} else {
    $evidencePassed
}

Write-Evidence $evidencePassed
$results | Format-Table -AutoSize

if (-not $passedForExit) {
    Write-Host ""
    Write-Host "07 browser handoff checks failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    Write-Host "Evidence written to: $EvidencePath" -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Host "07 browser handoff checks passed."
Write-Host "Evidence written to: $EvidencePath"
if ($FixtureMode) {
    Write-Host "Fixture mode output is for tooling self-test only, not release evidence." -ForegroundColor Yellow
} elseif ($AllowPartial) {
    Write-Host "Partial browser handoff output is prep evidence only; full release evidence still requires browser complete and completed desktop poll." -ForegroundColor Yellow
}
exit 0
