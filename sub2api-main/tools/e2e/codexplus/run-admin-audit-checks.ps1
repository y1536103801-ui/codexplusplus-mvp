param(
    [string]$Root,
    [string]$EvidenceDir,
    [string]$EnvPrefix = "CODEXPLUS_07_E2E_",
    [string]$EnvFile,
    [int]$EventLimit = 50,
    [switch]$FixtureMode,
    [switch]$AllowAdminAuditReads,
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
        [int]$HttpStatus,
        [int]$EventCount,
        [string]$EventTypes,
        [string]$ExpectedSignal,
        [string]$ErrorCode,
        [string]$ServiceStatus,
        [bool]$HasRequestId,
        [bool]$HasConfigVersion,
        [bool]$DeviceMatched,
        [bool]$RedactionApplied,
        [string]$Result,
        [string]$Note,
        [string]$ExpectedGatewayRequestId = "",
        [bool]$GatewayRequestIdMatched = $false
    )
    $script:observations.Add([pscustomobject]@{
        Scenario = $Scenario
        HttpStatus = $HttpStatus
        EventCount = $EventCount
        EventTypes = $EventTypes
        ExpectedSignal = $ExpectedSignal
        ErrorCode = $ErrorCode
        ServiceStatus = $ServiceStatus
        RequestId = if ($HasRequestId) { "present" } else { "missing" }
        GatewayRequestId = if ([string]::IsNullOrWhiteSpace($ExpectedGatewayRequestId)) { "missing" } else { Redact-Text $ExpectedGatewayRequestId }
        RequestIdCorrelation = if ($GatewayRequestIdMatched) { "matched" } else { "missing-or-mismatched" }
        ConfigVersion = if ($HasConfigVersion) { "present" } else { "missing" }
        DeviceMatch = if ($DeviceMatched) { "yes" } else { "no" }
        Redaction = if ($RedactionApplied) { "redaction_applied" } else { "missing" }
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

function Join-Url {
    param([string]$Base, [string]$Path)
    return $Base.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Test-SafeUrl {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $false
    }
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

function Get-Field {
    param(
        [object]$Value,
        [string[]]$Names
    )
    if ($null -eq $Value) {
        return $null
    }
    foreach ($name in $Names) {
        if ($Value -is [hashtable] -and $Value.ContainsKey($name)) {
            return $Value[$name]
        }
        $prop = $Value.PSObject.Properties | Where-Object { $_.Name -ieq $name } | Select-Object -First 1
        if ($null -ne $prop) {
            return $prop.Value
        }
    }
    return $null
}

function Get-EnvelopeData {
    param([object]$Json)
    if ($null -eq $Json) {
        return @()
    }
    if ($Json -is [System.Array]) {
        return @($Json)
    }
    $data = Get-Field $Json @("data", "items", "events")
    if ($null -eq $data) {
        return @($Json)
    }
    return @($data)
}

function Convert-Payload {
    param([object]$Payload)
    if ($null -eq $Payload) {
        return $null
    }
    if ($Payload -is [string]) {
        try {
            return $Payload | ConvertFrom-Json
        } catch {
            return $null
        }
    }
    return $Payload
}

function Get-SafeScalar {
    param([object]$Value)
    if ($null -eq $Value) {
        return ""
    }
    if ($Value -is [System.Array]) {
        return ((@($Value) | ForEach-Object { Get-SafeScalar $_ }) -join ",")
    }
    return (Redact-Text ([string]$Value)).Trim()
}

function Test-EventHasValue {
    param(
        [object[]]$Events,
        [string[]]$Names,
        [string]$Pattern = ".+"
    )
    foreach ($event in $Events) {
        $value = Get-Field $event $Names
        if ($null -ne $value -and (Get-SafeScalar $value) -match $Pattern) {
            return $true
        }
        $payload = Convert-Payload (Get-Field $event @("payload"))
        $payloadValue = Get-Field $payload $Names
        if ($null -ne $payloadValue -and (Get-SafeScalar $payloadValue) -match $Pattern) {
            return $true
        }
        $metadata = Get-Field $payload @("metadata")
        $metadataValue = Get-Field $metadata $Names
        if ($null -ne $metadataValue -and (Get-SafeScalar $metadataValue) -match $Pattern) {
            return $true
        }
    }
    return $false
}

function Get-FirstEventValue {
    param(
        [object[]]$Events,
        [string[]]$Names
    )
    foreach ($event in $Events) {
        $value = Get-Field $event $Names
        if ($null -ne $value -and (Get-SafeScalar $value) -ne "") {
            return Get-SafeScalar $value
        }
        $payload = Convert-Payload (Get-Field $event @("payload"))
        $payloadValue = Get-Field $payload $Names
        if ($null -ne $payloadValue -and (Get-SafeScalar $payloadValue) -ne "") {
            return Get-SafeScalar $payloadValue
        }
        $metadata = Get-Field $payload @("metadata")
        $metadataValue = Get-Field $metadata $Names
        if ($null -ne $metadataValue -and (Get-SafeScalar $metadataValue) -ne "") {
            return Get-SafeScalar $metadataValue
        }
    }
    return ""
}

function Get-EventTypes {
    param([object[]]$Events)
    $types = New-Object System.Collections.Generic.List[string]
    foreach ($event in $Events) {
        $eventType = Get-FirstEventValue @($event) @("event_type", "eventType", "type")
        if ($eventType -ne "") {
            $types.Add($eventType)
        }
    }
    if ($types.Count -eq 0) {
        return ""
    }
    return (($types | Sort-Object -Unique) -join ",")
}

function Get-FixtureGatewayRequestId {
    param([string]$Scenario)
    return ("fixture-{0}-request" -f ($Scenario -replace "_", "-"))
}

function Read-GatewayRequestIds {
    $requestIds = @{}
    $path = Join-Path $EvidencePath "05-gateway-policy-e2e.md"
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        return $requestIds
    }

    $scenarioIndex = -1
    $requestIdIndex = -1
    foreach ($line in Get-Content -LiteralPath $path) {
        $trimmed = $line.Trim()
        if (-not $trimmed.StartsWith("|")) {
            continue
        }
        $cells = @($trimmed.Trim([char]"|").Split("|") | ForEach-Object { $_.Trim() })
        if ($cells.Count -eq 0) {
            continue
        }
        if ($scenarioIndex -lt 0 -or $requestIdIndex -lt 0) {
            for ($index = 0; $index -lt $cells.Count; $index++) {
                if ($cells[$index] -ieq "Scenario") {
                    $scenarioIndex = $index
                }
                if ($cells[$index] -imatch "^Request\s*Id$") {
                    $requestIdIndex = $index
                }
            }
            continue
        }
        if ($cells[0] -match "^-+$") {
            continue
        }
        if ($cells.Count -le $scenarioIndex -or $cells.Count -le $requestIdIndex) {
            continue
        }

        $scenario = $cells[$scenarioIndex]
        $requestId = $cells[$requestIdIndex]
        if ($scenario -match "^user_" -and -not [string]::IsNullOrWhiteSpace($requestId) -and $requestId -notmatch "^(missing|not-run|fail)$") {
            $requestIds[$scenario] = $requestId
        }
    }
    return $requestIds
}

function Get-ExpectedGatewayRequestId {
    param(
        [string]$Scenario,
        [hashtable]$GatewayRequestIds,
        [bool]$AllowFixtureFallback
    )
    if ($GatewayRequestIds.ContainsKey($Scenario)) {
        return $GatewayRequestIds[$Scenario]
    }
    if ($AllowFixtureFallback) {
        return Get-FixtureGatewayRequestId $Scenario
    }
    return ""
}

function Test-EventRequestIdMatched {
    param(
        [object[]]$Events,
        [string]$ExpectedRequestId
    )
    if ([string]::IsNullOrWhiteSpace($ExpectedRequestId)) {
        return $false
    }
    return (Test-EventHasValue $Events @("request_id", "requestId") ("^" + [regex]::Escape($ExpectedRequestId) + "$"))
}

function Invoke-AdminEventsRequest {
    param(
        [string]$AdminBaseUrl,
        [string]$AdminToken,
        [string]$UserID
    )
    $url = Join-Url $AdminBaseUrl ("/api/v1/admin/codex-plus/users/{0}/events?limit={1}" -f $UserID, $EventLimit)
    $headers = @{ Authorization = "Bearer $AdminToken" }

    try {
        $response = Invoke-WebRequest -Method Get -Uri $url -Headers $headers -TimeoutSec 30 -UseBasicParsing
        $text = [string]$response.Content
        $json = $null
        try { $json = $text | ConvertFrom-Json } catch { $json = $null }
        return @{
            StatusCode = [int]$response.StatusCode
            Json = $json
            Text = Redact-Text $text
        }
    } catch {
        $statusCode = 0
        $text = $_.Exception.Message
        if ($null -ne $_.Exception.Response) {
            $statusCode = [int]$_.Exception.Response.StatusCode
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $text = $reader.ReadToEnd()
        }
        $json = $null
        try { $json = $text | ConvertFrom-Json } catch { $json = $null }
        return @{
            StatusCode = $statusCode
            Json = $json
            Text = Redact-Text $text
        }
    }
}

function New-FixtureEvents {
    param(
        [string]$Scenario,
        [string]$UserID,
        [string]$DeviceID,
        [string]$EventType,
        [string]$ErrorCode,
        [string]$ServiceStatus,
        [string]$RequestId
    )
    $requestId = if ([string]::IsNullOrWhiteSpace($RequestId)) { Get-FixtureGatewayRequestId $Scenario } else { $RequestId }
    $payload = [pscustomobject]@{
        event_type = $EventType
        user_id = $UserID
        device_id = $DeviceID
        request_id = $requestId
        config_version = "fixture-config-v1"
        error_code = $ErrorCode
        risk_tags = if ($EventType -eq "gateway_policy_rejected") { @("gateway_policy_rejected") } else { @("usage_reconciliation") }
        redaction_applied = $true
        metadata = [pscustomobject]@{
            service_status = $ServiceStatus
            reason = $ServiceStatus
        }
    }
    return @([pscustomobject]@{
        id = "event-$Scenario"
        user_id = $UserID
        device_id = $DeviceID
        event_type = $EventType
        severity = if ($EventType -eq "gateway_policy_rejected") { "warning" } else { "info" }
        request_id = $requestId
        config_version = "fixture-config-v1"
        payload = $payload
    })
}

function Write-MarkdownTable {
    param([System.Collections.IEnumerable]$Rows)
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("| Scenario | HTTP | Events | Event types | Expected signal | Error code | Service status | Request ID | Gateway request_id | Request ID correlation | Config version | Device match | Redaction | Result | Note |")
    $lines.Add("| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |")
    foreach ($row in $Rows) {
        $line = "| $($row.Scenario) | $($row.HttpStatus) | $($row.EventCount) | $($row.EventTypes) | $($row.ExpectedSignal) | $($row.ErrorCode) | $($row.ServiceStatus) | $($row.RequestId) | $($row.GatewayRequestId) | $($row.RequestIdCorrelation) | $($row.ConfigVersion) | $($row.DeviceMatch) | $($row.Redaction) | $($row.Result) | $($row.Note) |"
        $safeLine = $line -replace "`r?`n", " "
        $lines.Add($safeLine)
    }
    return ($lines -join [Environment]::NewLine)
}

function Write-Evidence {
    param([bool]$Passed)
    $result = if ($Passed) { "pass" } else { "fail" }
    $fixtureNote = if ($FixtureMode) { "Fixture mode: true. This is tool self-test evidence only, not release evidence." } else { "Fixture mode: false. Admin audit/event rows were read only if -AllowAdminAuditReads was supplied." }
    $coverage = if ($Passed) { "pass" } else { "fail" }
    $table = Write-MarkdownTable $observations

    $content = @"
# 09 Usage Events Audit

Run folder: $(Split-Path -Leaf $EvidencePath)
Status: executed
Result: $result

## Scope

$fixtureNote

This runner reads admin-visible Codex++ events for the active success path and policy rejection paths. It does not mutate entitlements, devices, plans, model policy, balances, package state, compatibility snapshots or provider settings.

## Usage And Admin Audit Correlation

Usage rows and admin audit events for success and rejection paths: $coverage
Gateway request_id correlation: $coverage

$table

## Structured Signals Required

- Success path: `usage_recorded`, matching gateway `request_id` from `05-gateway-policy-e2e.md`, `config_version`, matching test device ID when supplied, and `redaction_applied`.
- Rejection paths: `gateway_policy_rejected`, `GATEWAY_POLICY_*` error code, service status, matching gateway `request_id` from `05-gateway-policy-e2e.md`, `config_version`, matching test device ID when supplied, and `redaction_applied`.

## Redaction

Redacted evidence: true; token values, gateway API Keys, Authorization headers, upstream provider Keys and raw event payloads are intentionally not printed.
"@

    Set-Content -LiteralPath (Join-Path $EvidencePath "09-usage-events-audit.md") -Encoding UTF8 -Value $content
}

Import-EnvFile $EnvFile

$adminBaseUrl = Get-EnvValue "ADMIN_BASE_URL"
$adminToken = Get-EnvValue "ADMIN_TOKEN"
$deviceId = Get-EnvValue "TEST_DEVICE_ID"

$scenarios = @(
    @{ Name = "user_active"; UserIDSuffix = "USER_ACTIVE_ID"; ExpectedEvent = "usage_recorded"; ExpectedErrorPattern = ""; ExpectedServicePattern = "available|success|usage"; FixtureUserID = "1001" },
    @{ Name = "user_not_purchased"; UserIDSuffix = "USER_NOT_PURCHASED_ID"; ExpectedEvent = "gateway_policy_rejected"; ExpectedErrorPattern = "GATEWAY_POLICY_ENTITLEMENT_NOT_PURCHASED"; ExpectedServicePattern = "not_purchased"; FixtureUserID = "1002" },
    @{ Name = "user_expired"; UserIDSuffix = "USER_EXPIRED_ID"; ExpectedEvent = "gateway_policy_rejected"; ExpectedErrorPattern = "GATEWAY_POLICY_ENTITLEMENT_EXPIRED"; ExpectedServicePattern = "expired"; FixtureUserID = "1003" },
    @{ Name = "user_low_balance"; UserIDSuffix = "USER_LOW_BALANCE_ID"; ExpectedEvent = "gateway_policy_rejected"; ExpectedErrorPattern = "GATEWAY_POLICY_BALANCE_INSUFFICIENT|GATEWAY_POLICY_QUOTA_EXCEEDED"; ExpectedServicePattern = "low_balance|rate_limited"; FixtureUserID = "1004" },
    @{ Name = "user_device_revoked"; UserIDSuffix = "USER_DEVICE_REVOKED_ID"; ExpectedEvent = "gateway_policy_rejected"; ExpectedErrorPattern = "GATEWAY_POLICY_DEVICE_REVOKED|GATEWAY_POLICY_DEVICE_BLOCKED"; ExpectedServicePattern = "device_revoked"; FixtureUserID = "1005" },
    @{ Name = "user_model_denied"; UserIDSuffix = "USER_MODEL_DENIED_ID"; ExpectedEvent = "gateway_policy_rejected"; ExpectedErrorPattern = "GATEWAY_POLICY_MODEL_NOT_ALLOWED"; ExpectedServicePattern = "model_unavailable"; FixtureUserID = "1006" }
)

$gatewayRequestIds = Read-GatewayRequestIds
$missingGatewayRequestIds = @($scenarios | Where-Object { -not $gatewayRequestIds.ContainsKey($_.Name) } | ForEach-Object { $_.Name })
if ($FixtureMode -and $gatewayRequestIds.Count -eq 0) {
    Add-Result "gateway-evidence:request-id-map" $true "Fixture fallback uses gateway fixture request_id values for correlation self-test."
} else {
    Add-Result "gateway-evidence:request-id-map" ($missingGatewayRequestIds.Count -eq 0) ("Gateway request_id values from 05-gateway-policy-e2e.md found for {0}/{1} scenario(s)." -f ($scenarios.Count - $missingGatewayRequestIds.Count), $scenarios.Count)
}

if ($EventLimit -lt 1 -or $EventLimit -gt 200) {
    Add-Result "event-limit:range" $false "EventLimit must be between 1 and 200."
} else {
    Add-Result "event-limit:range" $true "EventLimit=$EventLimit."
}

if ($FixtureMode) {
    if ([string]::IsNullOrWhiteSpace($deviceId)) {
        $deviceId = "device-fixture-001"
    }
    foreach ($scenario in $scenarios) {
        $errorCode = if ($scenario.ExpectedErrorPattern -eq "") { "" } else { ($scenario.ExpectedErrorPattern -split "\|")[0] }
        $serviceStatus = if ($scenario.Name -eq "user_active") { "available" } else { ($scenario.ExpectedServicePattern -split "\|")[0] }
        $expectedGatewayRequestId = Get-ExpectedGatewayRequestId $scenario.Name $gatewayRequestIds $true
        $events = New-FixtureEvents $scenario.Name $scenario.FixtureUserID $deviceId $scenario.ExpectedEvent $errorCode $serviceStatus $expectedGatewayRequestId
        $eventTypes = Get-EventTypes $events
        $hasRequestId = Test-EventHasValue $events @("request_id", "requestId")
        $gatewayRequestIdMatched = Test-EventRequestIdMatched $events $expectedGatewayRequestId
        $hasConfigVersion = Test-EventHasValue $events @("config_version", "configVersion")
        $deviceMatched = Test-EventHasValue $events @("device_id", "deviceId") ([regex]::Escape($deviceId))
        $redactionApplied = Test-EventHasValue $events @("redaction_applied", "redactionApplied") "true"
        $actualErrorCode = Get-FirstEventValue $events @("error_code", "errorCode")
        $actualServiceStatus = Get-FirstEventValue $events @("service_status", "serviceStatus")
        $eventMatched = $eventTypes -match [regex]::Escape($scenario.ExpectedEvent)
        $errorMatched = if ($scenario.ExpectedErrorPattern -eq "") { $true } else { $actualErrorCode -match $scenario.ExpectedErrorPattern }
        $serviceMatched = if ($scenario.ExpectedServicePattern -eq "") { $true } else { $actualServiceStatus -match $scenario.ExpectedServicePattern }
        $passed = $eventMatched -and $errorMatched -and $serviceMatched -and $hasRequestId -and $gatewayRequestIdMatched -and $hasConfigVersion -and $deviceMatched -and $redactionApplied
        $observationResult = if ($passed) { "pass" } else { "fail" }
        Add-Result "admin-audit:$($scenario.Name)" $passed "fixture events include safe correlation fields and matched gateway request_id."
        Add-Observation $scenario.Name 200 @($events).Count $eventTypes $scenario.ExpectedEvent $actualErrorCode $actualServiceStatus $hasRequestId $hasConfigVersion $deviceMatched $redactionApplied $observationResult "Fixture mode only; raw payload not printed." $expectedGatewayRequestId $gatewayRequestIdMatched
    }
} elseif (-not $AllowAdminAuditReads) {
    Add-Result "admin-audit-reads-opt-in" $false "Supply -AllowAdminAuditReads only in an approved test environment."
    Add-Observation "admin-audit" 0 0 "" "explicit opt-in" "" "" $false $false $false $true "fail" "Admin audit rows were not read because -AllowAdminAuditReads was not supplied." "" $false
} else {
    Add-Result "env:ADMIN_BASE_URL" (-not [string]::IsNullOrWhiteSpace($adminBaseUrl)) "$($EnvPrefix)ADMIN_BASE_URL must be set."
    Add-Result "env:ADMIN_BASE_URL-safe" ($AllowProduction -or (Test-SafeUrl $adminBaseUrl)) "$($EnvPrefix)ADMIN_BASE_URL must be local/dev/test/staging/sandbox/qa unless -AllowProduction is supplied."
    Add-Result "env:ADMIN_TOKEN" (-not [string]::IsNullOrWhiteSpace($adminToken)) "$($EnvPrefix)ADMIN_TOKEN must be set; value intentionally not printed."
    Add-Result "env:TEST_DEVICE_ID" (-not [string]::IsNullOrWhiteSpace($deviceId)) "$($EnvPrefix)TEST_DEVICE_ID must be set."

    foreach ($scenario in $scenarios) {
        $userId = Get-EnvValue $scenario.UserIDSuffix
        if ([string]::IsNullOrWhiteSpace($userId) -or $userId -notmatch "^\d+$") {
            Add-Result "env:$($scenario.UserIDSuffix)" $false "$($EnvPrefix)$($scenario.UserIDSuffix) must be a numeric test user ID."
            Add-Observation $scenario.Name 0 0 "" $scenario.ExpectedEvent "" "" $false $false $false $true "fail" "Missing numeric user ID; raw values not printed." "" $false
            continue
        }

        $expectedGatewayRequestId = Get-ExpectedGatewayRequestId $scenario.Name $gatewayRequestIds $false
        $response = Invoke-AdminEventsRequest $adminBaseUrl $adminToken $userId
        $events = Get-EnvelopeData $response.Json
        $eventTypes = Get-EventTypes $events
        $hasRequestId = Test-EventHasValue $events @("request_id", "requestId")
        $gatewayRequestIdMatched = Test-EventRequestIdMatched $events $expectedGatewayRequestId
        $hasConfigVersion = Test-EventHasValue $events @("config_version", "configVersion")
        $deviceMatched = if ([string]::IsNullOrWhiteSpace($deviceId)) { $true } else { Test-EventHasValue $events @("device_id", "deviceId") ([regex]::Escape($deviceId)) }
        $redactionApplied = Test-EventHasValue $events @("redaction_applied", "redactionApplied") "true"
        $actualErrorCode = Get-FirstEventValue $events @("error_code", "errorCode")
        $actualServiceStatus = Get-FirstEventValue $events @("service_status", "serviceStatus")
        $eventMatched = $eventTypes -match [regex]::Escape($scenario.ExpectedEvent)
        $errorMatched = if ($scenario.ExpectedErrorPattern -eq "") { $true } else { $actualErrorCode -match $scenario.ExpectedErrorPattern }
        $serviceMatched = if ($scenario.ExpectedServicePattern -eq "") { $true } else { $actualServiceStatus -match $scenario.ExpectedServicePattern }
        $httpOk = ($response.StatusCode -eq 200)
        $hasEvents = @($events).Count -gt 0
        $passed = $httpOk -and $hasEvents -and $eventMatched -and $errorMatched -and $serviceMatched -and $hasRequestId -and $gatewayRequestIdMatched -and $hasConfigVersion -and $deviceMatched -and $redactionApplied
        $observationResult = if ($passed) { "pass" } else { "fail" }
        Add-Result "admin-audit:$($scenario.Name)" $passed "http=$($response.StatusCode); events=$(@($events).Count); gateway request_id matched=$gatewayRequestIdMatched; safe fields only."
        Add-Observation $scenario.Name $response.StatusCode @($events).Count $eventTypes $scenario.ExpectedEvent $actualErrorCode $actualServiceStatus $hasRequestId $hasConfigVersion $deviceMatched $redactionApplied $observationResult "Raw admin event payload omitted." $expectedGatewayRequestId $gatewayRequestIdMatched
    }
}

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
Write-Evidence ($failed.Count -eq 0)

$results | Format-Table -AutoSize

if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 admin audit checks failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    Write-Host "Evidence written to: $EvidencePath" -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Host "07 admin audit checks passed."
Write-Host "Evidence written to: $EvidencePath"
if ($FixtureMode) {
    Write-Host "Fixture mode output is for tooling self-test only, not release evidence." -ForegroundColor Yellow
}
exit 0
