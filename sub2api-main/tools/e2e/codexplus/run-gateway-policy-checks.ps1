param(
    [string]$Root,
    [string]$EvidenceDir,
    [string]$EnvPrefix = "CODEXPLUS_07_E2E_",
    [string]$EnvFile,
    [string]$GatewayPath = "/v1/responses",
    [switch]$FixtureMode,
    [switch]$AllowGatewayRequests,
    [switch]$AllowProduction
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")
} else {
    $Root = Resolve-Path $Root
}

$PlanRoot = Join-Path $Root "codex-plus-dev-plan"
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
        [string]$Scenario,
        [string]$Model,
        [int]$HttpStatus,
        [string]$Expected,
        [string]$Observed,
        [string]$RequestId,
        [string]$ErrorCode,
        [string]$ServiceStatus,
        [string]$Reason,
        [string]$BodyParse,
        [string]$AuditCorrelation,
        [string]$Result,
        [string]$Note
    )
    $script:observations.Add([pscustomobject]@{
        Scenario = $Scenario
        Model = $Model
        HttpStatus = $HttpStatus
        Expected = $Expected
        Observed = $Observed
        RequestId = $RequestId
        ErrorCode = $ErrorCode
        ServiceStatus = $ServiceStatus
        Reason = $Reason
        BodyParse = $BodyParse
        AuditCorrelation = $AuditCorrelation
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
    $redacted = $redacted -replace "(?i)(api_key|upstream_key|access_token|refresh_token|poll_token|session_token)(""|')?\s*:\s*(""|')[^""']+(""|')", '$1: "[redacted]"'
    return $redacted
}

function Join-Url {
    param([string]$Base, [string]$Path)
    return $Base.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Get-HeaderValue {
    param(
        [object]$Headers,
        [string[]]$Names
    )
    if ($null -eq $Headers) {
        return ""
    }
    foreach ($name in $Names) {
        try {
            $value = $Headers[$name]
            if ($null -ne $value -and [string]$value -ne "") {
                if ($value -is [System.Array]) {
                    return [string]$value[0]
                }
                return [string]$value
            }
        } catch {
            # Try property lookup below.
        }
        $prop = $Headers.PSObject.Properties | Where-Object { $_.Name -ieq $name } | Select-Object -First 1
        if ($null -ne $prop -and $null -ne $prop.Value -and [string]$prop.Value -ne "") {
            return [string]$prop.Value
        }
    }
    return ""
}

function Get-Field {
    param(
        [object]$Value,
        [string[]]$Names
    )
    if ($null -eq $Value) {
        return $null
    }
    foreach ($name in $Names) {
        $prop = $Value.PSObject.Properties | Where-Object { $_.Name -ieq $name } | Select-Object -First 1
        if ($null -ne $prop) {
            return $prop.Value
        }
    }
    return $null
}

function Get-ServiceStatusFromErrorCode {
    param([string]$ErrorCode)
    switch -Regex ($ErrorCode) {
        "GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED" { return "not_purchased" }
        "GATEWAY_POLICY_ENTITLEMENT_EXPIRED" { return "expired" }
        "GATEWAY_POLICY_BALANCE_INSUFFICIENT|GATEWAY_POLICY_QUOTA_EXCEEDED" { return "low_balance" }
        "GATEWAY_POLICY_DEVICE_REVOKED|GATEWAY_POLICY_DEVICE_BLOCKED" { return "device_revoked" }
        "GATEWAY_POLICY_MODEL_NOT_ALLOWED" { return "model_unavailable" }
        "GATEWAY_POLICY_RATE_LIMITED|GATEWAY_POLICY_TPM_LIMITED|GATEWAY_POLICY_CONCURRENCY_LIMITED" { return "rate_limited" }
        "GATEWAY_POLICY_CONFIG_UNAVAILABLE" { return "gateway_unhealthy" }
        default { return "" }
    }
}

function Get-GatewayResponseSummary {
    param(
        [object]$Json,
        [string]$BodyParseStatus
    )
    $errorObj = Get-Field $Json @("error")
    $errorCode = Get-Field $errorObj @("code", "type", "status", "error_code")
    if ($null -eq $errorCode) {
        $errorCode = Get-Field $Json @("error_code", "code", "type", "status")
    }
    $serviceStatus = Get-Field $errorObj @("service_status", "serviceStatus")
    if ($null -eq $serviceStatus) {
        $serviceStatus = Get-Field $Json @("service_status", "serviceStatus")
    }
    $reason = Get-Field $errorObj @("message", "reason")
    if ($null -eq $reason) {
        $reason = Get-Field $Json @("message", "reason", "detail")
    }

    $safeErrorCode = Redact-Text ([string]$errorCode)
    $safeServiceStatus = Redact-Text ([string]$serviceStatus)
    if ([string]::IsNullOrWhiteSpace($safeServiceStatus)) {
        $safeServiceStatus = Get-ServiceStatusFromErrorCode $safeErrorCode
    }
    $safeReason = Redact-Text ([string]$reason)
    if ($safeReason.Length -gt 80) {
        $safeReason = $safeReason.Substring(0, 80)
    }

    return [pscustomobject]@{
        ErrorCode = $safeErrorCode
        ServiceStatus = $safeServiceStatus
        Reason = $safeReason
        BodyParse = $BodyParseStatus
    }
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

function Invoke-GatewayRequest {
    param(
        [string]$GatewayBaseUrl,
        [string]$ApiKey,
        [string]$Model
    )
    $url = Join-Url $GatewayBaseUrl $GatewayPath
    $headers = @{
        Authorization = "Bearer $ApiKey"
        "X-CodexPlus-Device-Id" = Get-EnvValue "TEST_DEVICE_ID"
        "Content-Type" = "application/json"
    }
    $body = @{
        model = $Model
        input = "Codex++ release gate ping. Reply with pong."
        max_output_tokens = 16
        stream = $false
    } | ConvertTo-Json -Depth 8

    try {
        $response = Invoke-WebRequest -Method Post -Uri $url -Headers $headers -Body $body -ContentType "application/json" -TimeoutSec 45 -UseBasicParsing
        $json = $null
        $parseStatus = "json"
        try {
            $json = ([string]$response.Content) | ConvertFrom-Json
        } catch {
            $parseStatus = "non-json"
        }
        return @{
            StatusCode = [int]$response.StatusCode
            RequestId = Get-HeaderValue $response.Headers @("X-Request-ID", "X-Request-Id", "x-request-id")
            ClientRequestId = Get-HeaderValue $response.Headers @("X-Client-Request-ID", "X-Client-Request-Id", "x-client-request-id")
            Json = $json
            BodyParse = $parseStatus
            Text = Redact-Text ([string]$response.Content)
        }
    } catch {
        $statusCode = 0
        $text = $_.Exception.Message
        $headers = $null
        if ($null -ne $_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
            $headers = $_.Exception.Response.Headers
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $text = $reader.ReadToEnd()
        }
        $json = $null
        $parseStatus = "json"
        try {
            $json = $text | ConvertFrom-Json
        } catch {
            $parseStatus = "non-json"
        }
        return @{
            StatusCode = $statusCode
            RequestId = Get-HeaderValue $headers @("X-Request-ID", "X-Request-Id", "x-request-id")
            ClientRequestId = Get-HeaderValue $headers @("X-Client-Request-ID", "X-Client-Request-Id", "x-client-request-id")
            Json = $json
            BodyParse = $parseStatus
            Text = Redact-Text $text
        }
    }
}

function Write-MarkdownTable {
    param([System.Collections.IEnumerable]$Rows)
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("| Scenario | Model | HTTP | Expected | Observed | RequestId | ErrorCode | ServiceStatus | Reason | BodyParse | AuditCorrelation | Result | Note |")
    $lines.Add("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
    foreach ($row in $Rows) {
        $lines.Add("| $($row.Scenario) | $($row.Model) | $($row.HttpStatus) | $($row.Expected) | $($row.Observed) | $($row.RequestId) | $($row.ErrorCode) | $($row.ServiceStatus) | $($row.Reason) | $($row.BodyParse) | $($row.AuditCorrelation) | $($row.Result) | $($row.Note) |")
    }
    return ($lines -join [Environment]::NewLine)
}

function Write-Evidence {
    param([bool]$Passed)
    $result = if ($Passed) { "pass" } else { "fail" }
    $fixtureNote = if ($FixtureMode) { "Fixture mode: true. This is tool self-test evidence only, not release evidence." } else { "Fixture mode: false. Gateway requests were made only if -AllowGatewayRequests was supplied." }
    $table = Write-MarkdownTable $observations

    $gatewayContent = @"
# 05 Gateway Policy E2E

Run folder: $(Split-Path -Leaf $EvidencePath)
Status: executed
Result: $result

## Scope

$fixtureNote

This runner covers one low-cost active-user gateway request and rejection probes for no entitlement, expired entitlement, insufficient balance, revoked device and unauthorized model. It does not validate desktop launch, package install, compatibility migration, payment, admin screenshots or full audit-event visibility.

## Gateway Policy Observations

$table

## Redaction

User-side gateway API Keys, upstream provider Keys, Authorization headers, JWTs and response bodies are redacted or summarized. Token values are intentionally not printed.
"@

    $auditContent = @"
# 09 Usage Events Audit

Run folder: $(Split-Path -Leaf $EvidencePath)
Status: executed
Result: fail

## Scope

$fixtureNote

Gateway request/rejection observations are recorded in `05-gateway-policy-e2e.md`. Admin-visible usage/event/audit rows still require separate evidence before final release verification can pass.

## Redaction

Token values, gateway API Keys, Authorization headers and upstream provider Keys are intentionally not printed.
"@

    $defectContent = @"
# 11 Defects

## Gateway Runner Findings

- P0: review any gateway policy row marked fail before release.
- P1: admin usage/audit row evidence remains required outside this runner.
- P2: response-body content is intentionally summarized to reduce secret leakage risk.
- P3: none recorded by this runner.
"@

    Set-Content -LiteralPath (Join-Path $EvidencePath "05-gateway-policy-e2e.md") -Encoding UTF8 -Value $gatewayContent
    Set-Content -LiteralPath (Join-Path $EvidencePath "09-usage-events-audit.md") -Encoding UTF8 -Value $auditContent
    Set-Content -LiteralPath (Join-Path $EvidencePath "11-defects.md") -Encoding UTF8 -Value $defectContent
}

Import-EnvFile $EnvFile

$gatewayBaseUrl = Get-EnvValue "GATEWAY_BASE_URL"
$allowedModel = Get-EnvValue "ALLOWED_TEST_MODEL"
$deniedModel = Get-EnvValue "DENIED_TEST_MODEL"
$deviceId = Get-EnvValue "TEST_DEVICE_ID"

if ($FixtureMode) {
    $fixtureRows = @(
        @{ Scenario = "user_active"; Model = "allowed-model-fixture"; Http = 200; Expected = "2xx success"; Observed = "success"; RequestId = "fixture-user-active-request"; ErrorCode = ""; ServiceStatus = "available"; Reason = "success"; BodyParse = "json" },
        @{ Scenario = "user_not_purchased"; Model = "allowed-model-fixture"; Http = 403; Expected = "GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED"; Observed = "rejection"; RequestId = "fixture-user-not-purchased-request"; ErrorCode = "GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED"; ServiceStatus = "not_purchased"; Reason = "not_purchased"; BodyParse = "json" },
        @{ Scenario = "user_expired"; Model = "allowed-model-fixture"; Http = 403; Expected = "GATEWAY_POLICY_ENTITLEMENT_EXPIRED"; Observed = "rejection"; RequestId = "fixture-user-expired-request"; ErrorCode = "GATEWAY_POLICY_ENTITLEMENT_EXPIRED"; ServiceStatus = "expired"; Reason = "expired"; BodyParse = "json" },
        @{ Scenario = "user_low_balance"; Model = "allowed-model-fixture"; Http = 402; Expected = "GATEWAY_POLICY_BALANCE_INSUFFICIENT"; Observed = "rejection"; RequestId = "fixture-user-low-balance-request"; ErrorCode = "GATEWAY_POLICY_BALANCE_INSUFFICIENT"; ServiceStatus = "low_balance"; Reason = "low_balance"; BodyParse = "json" },
        @{ Scenario = "user_device_revoked"; Model = "allowed-model-fixture"; Http = 403; Expected = "GATEWAY_POLICY_DEVICE_REVOKED"; Observed = "rejection"; RequestId = "fixture-user-device-revoked-request"; ErrorCode = "GATEWAY_POLICY_DEVICE_REVOKED"; ServiceStatus = "device_revoked"; Reason = "device_revoked"; BodyParse = "json" },
        @{ Scenario = "user_model_denied"; Model = "denied-model-fixture"; Http = 403; Expected = "GATEWAY_POLICY_MODEL_NOT_ALLOWED"; Observed = "rejection"; RequestId = "fixture-user-model-denied-request"; ErrorCode = "GATEWAY_POLICY_MODEL_NOT_ALLOWED"; ServiceStatus = "model_unavailable"; Reason = "model_unavailable"; BodyParse = "json" }
    )
    foreach ($row in $fixtureRows) {
        Add-Result "gateway:$($row.Scenario)" $true "fixture http=$($row.Http); request_id present; error_code=$($row.ErrorCode)"
        Add-Observation $row.Scenario $row.Model $row.Http $row.Expected $row.Observed $row.RequestId $row.ErrorCode $row.ServiceStatus $row.Reason $row.BodyParse "ready-for-admin-audit" "pass" "Fixture mode only; not release evidence."
    }
} elseif (-not $AllowGatewayRequests) {
    Add-Result "gateway-requests-opt-in" $false "Supply -AllowGatewayRequests only in an approved low-cost test environment."
    $summary = "not-run"
    Add-Observation "gateway-policy" "" 0 "explicit opt-in" $summary "fail" "Gateway requests were not sent because -AllowGatewayRequests was not supplied."
} else {
    Add-Result "env:GATEWAY_BASE_URL" (-not [string]::IsNullOrWhiteSpace($gatewayBaseUrl)) "$($EnvPrefix)GATEWAY_BASE_URL must be set."
    Add-Result "env:GATEWAY_BASE_URL-safe" ($AllowProduction -or (Test-SafeUrl $gatewayBaseUrl)) "$($EnvPrefix)GATEWAY_BASE_URL must be local/dev/test/staging/sandbox/qa unless -AllowProduction is supplied."
    Add-Result "env:TEST_DEVICE_ID" (-not [string]::IsNullOrWhiteSpace($deviceId)) "$($EnvPrefix)TEST_DEVICE_ID must be set."
    Add-Result "env:ALLOWED_TEST_MODEL" (-not [string]::IsNullOrWhiteSpace($allowedModel)) "$($EnvPrefix)ALLOWED_TEST_MODEL must be set."
    Add-Result "env:DENIED_TEST_MODEL" (-not [string]::IsNullOrWhiteSpace($deniedModel)) "$($EnvPrefix)DENIED_TEST_MODEL must be set."

    $scenarios = @(
        @{ Name = "user_active"; KeySuffix = "USER_ACTIVE_GATEWAY_KEY"; Model = $allowedModel; ExpectSuccess = $true; ExpectedError = ""; ExpectedService = "available|success" },
        @{ Name = "user_not_purchased"; KeySuffix = "USER_NOT_PURCHASED_GATEWAY_KEY"; Model = $allowedModel; ExpectSuccess = $false; ExpectedError = "GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED"; ExpectedService = "not_purchased" },
        @{ Name = "user_expired"; KeySuffix = "USER_EXPIRED_GATEWAY_KEY"; Model = $allowedModel; ExpectSuccess = $false; ExpectedError = "GATEWAY_POLICY_ENTITLEMENT_EXPIRED"; ExpectedService = "expired" },
        @{ Name = "user_low_balance"; KeySuffix = "USER_LOW_BALANCE_GATEWAY_KEY"; Model = $allowedModel; ExpectSuccess = $false; ExpectedError = "GATEWAY_POLICY_BALANCE_INSUFFICIENT|GATEWAY_POLICY_QUOTA_EXCEEDED"; ExpectedService = "low_balance|rate_limited" },
        @{ Name = "user_device_revoked"; KeySuffix = "USER_DEVICE_REVOKED_GATEWAY_KEY"; Model = $allowedModel; ExpectSuccess = $false; ExpectedError = "GATEWAY_POLICY_DEVICE_REVOKED|GATEWAY_POLICY_DEVICE_BLOCKED"; ExpectedService = "device_revoked" },
        @{ Name = "user_model_denied"; KeySuffix = "USER_MODEL_DENIED_GATEWAY_KEY"; Model = $deniedModel; ExpectSuccess = $false; ExpectedError = "GATEWAY_POLICY_MODEL_NOT_ALLOWED"; ExpectedService = "model_unavailable" }
    )

    foreach ($scenario in $scenarios) {
        $apiKey = Get-EnvValue $scenario.KeySuffix
        if ([string]::IsNullOrWhiteSpace($apiKey)) {
            $expectedWhenMissing = if ($scenario.ExpectSuccess) { "2xx success" } else { "rejection" }
            Add-Result "gateway-key:$($scenario.Name)" $false "$($EnvPrefix)$($scenario.KeySuffix) must be set; value intentionally not printed."
            Add-Observation $scenario.Name $scenario.Model 0 $expectedWhenMissing "missing gateway key" "" "" "" "" "not-run" "missing-key" "fail" "Gateway key value intentionally not printed."
            continue
        }

        $response = Invoke-GatewayRequest $gatewayBaseUrl $apiKey $scenario.Model
        $summary = Get-GatewayResponseSummary $response.Json $response.BodyParse
        $is2xx = ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300)
        $requestId = if (-not [string]::IsNullOrWhiteSpace($response.RequestId)) { $response.RequestId } else { $response.ClientRequestId }
        $requestIdPresent = -not [string]::IsNullOrWhiteSpace($requestId)
        $expectedErrorMatched = if ($scenario.ExpectSuccess) { $true } else { $summary.ErrorCode -match $scenario.ExpectedError }
        $expectedServiceMatched = if ($scenario.ExpectSuccess) { $true } else { $summary.ServiceStatus -match $scenario.ExpectedService }
        $passed = if ($scenario.ExpectSuccess) {
            $is2xx -and $requestIdPresent
        } else {
            (-not $is2xx) -and $requestIdPresent -and $expectedErrorMatched -and $expectedServiceMatched
        }
        $expected = if ($scenario.ExpectSuccess) { "2xx success" } else { "rejection" }
        $observed = if ($is2xx) { "2xx success" } else { "structured rejection" }
        $result = if ($passed) { "pass" } else { "fail" }
        $auditCorrelation = if ($requestIdPresent) { "ready-for-admin-audit" } else { "missing-request-id" }
        Add-Result "gateway:$($scenario.Name)" $passed "http=$($response.StatusCode); expected=$expected; request_id present=$requestIdPresent; error_code=$($summary.ErrorCode); response body redacted."
        Add-Observation $scenario.Name $scenario.Model $response.StatusCode $expected $observed $requestId $summary.ErrorCode $summary.ServiceStatus $summary.Reason $summary.BodyParse $auditCorrelation $result "Response body redacted; safe fields retained."
    }
}

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
Write-Evidence ($failed.Count -eq 0)

$results | Format-Table -AutoSize

if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 gateway policy checks failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    Write-Host "Evidence written to: $EvidencePath" -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Host "07 gateway policy checks passed."
Write-Host "Evidence written to: $EvidencePath"
if ($FixtureMode) {
    Write-Host "Fixture mode output is for tooling self-test only, not release evidence." -ForegroundColor Yellow
}
exit 0
