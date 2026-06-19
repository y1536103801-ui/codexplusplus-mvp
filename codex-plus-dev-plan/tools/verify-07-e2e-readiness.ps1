param(
    [string]$Root,
    [string]$EnvPrefix = "CODEXPLUS_07_E2E_",
    [string]$EnvFile,
    [string]$OutputPath,
    [switch]$AllowProduction,
    [switch]$ProbeHttp,
    [switch]$EndpointPreflight,
    [switch]$EndpointPreflightOnly
)

$ErrorActionPreference = "Stop"

# Default required inputs use the CODEXPLUS_07_E2E_ prefix, for example:
# CODEXPLUS_07_E2E_BACKEND_BASE_URL, CODEXPLUS_07_E2E_ADMIN_BASE_URL,
# CODEXPLUS_07_E2E_MANAGER_BUILD_PATH, CODEXPLUS_07_E2E_ADMIN_TOKEN,
# CODEXPLUS_07_E2E_USER_ACTIVE_TOKEN, CODEXPLUS_07_E2E_USER_NOT_PURCHASED_TOKEN,
# CODEXPLUS_07_E2E_USER_EXPIRED_TOKEN, CODEXPLUS_07_E2E_USER_LOW_BALANCE_TOKEN,
# CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_TOKEN, CODEXPLUS_07_E2E_USER_MODEL_DENIED_TOKEN,
# CODEXPLUS_07_E2E_USER_ACTIVE_ID, CODEXPLUS_07_E2E_USER_NOT_PURCHASED_ID,
# CODEXPLUS_07_E2E_USER_EXPIRED_ID, CODEXPLUS_07_E2E_USER_LOW_BALANCE_ID,
# CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_ID, CODEXPLUS_07_E2E_USER_MODEL_DENIED_ID,
# CODEXPLUS_07_E2E_TEST_DEVICE_ID, CODEXPLUS_07_E2E_ALLOWED_TEST_MODEL,
# and CODEXPLUS_07_E2E_DENIED_TEST_MODEL.

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ($EndpointPreflightOnly) {
    $EndpointPreflight = $true
    $ProbeHttp = $true
}

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

function Get-EnvValue {
    param([string]$Suffix)
    return [Environment]::GetEnvironmentVariable($EnvPrefix + $Suffix, "Process")
}

function Test-RepoFile {
    param([string]$RelativePath)
    $path = Join-Path $Root $RelativePath
    Add-Check "repo-file:$RelativePath" (Test-Path -LiteralPath $path -PathType Leaf) $RelativePath
}

function Test-RepoDir {
    param([string]$RelativePath)
    $path = Join-Path $Root $RelativePath
    Add-Check "repo-dir:$RelativePath" (Test-Path -LiteralPath $path -PathType Container) $RelativePath
}

function Test-Placeholder {
    param([string]$Value)
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return $false
    }
    return ($Value -notmatch "(?i)^\s*(TODO|TBD|FILL_ME|PLACEHOLDER|PENDING|NOT_EXECUTED|EXAMPLE|<.*>)\s*$")
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

function Test-EnvRequired {
    param(
        [string]$Suffix,
        [string]$Kind
    )
    $name = $EnvPrefix + $Suffix
    $value = Get-EnvValue $Suffix
    $hasValue = -not [string]::IsNullOrWhiteSpace($value)
    Add-Check "env-present:$name" $hasValue "$name must be set; value intentionally not printed."
    if (-not $hasValue) {
        return $null
    }

    $notPlaceholder = Test-Placeholder $value
    Add-Check "env-not-placeholder:$name" $notPlaceholder "$name must not be TODO/FILL_ME/pending/example placeholder; value intentionally not printed."

    if ($Kind -eq "url") {
        $safe = Test-SafeUrl $value
        Add-Check "env-safe-nonproduction-url:$name" ($AllowProduction -or $safe) "$name must target local/dev/test/staging/sandbox/qa unless -AllowProduction is supplied."
    }

    if ($Kind -eq "path") {
        Add-Check "env-path-exists:$name" (Test-Path -LiteralPath $value) "$name path must exist."
    }

    return $value
}

function Test-ManagerBuildExecutable {
    param([string]$Path)

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return $false
    }

    $expectedName = "codex-plus-plus-manager.exe"
    if (Test-Path -LiteralPath $Path -PathType Leaf) {
        return ([System.IO.Path]::GetFileName($Path) -ieq $expectedName)
    }

    if (Test-Path -LiteralPath $Path -PathType Container) {
        return (Test-Path -LiteralPath (Join-Path $Path $expectedName) -PathType Leaf)
    }

    return $false
}

function Test-EnvNumericID {
    param([string]$Suffix)
    $value = Test-EnvRequired $Suffix "id"
    if (-not [string]::IsNullOrWhiteSpace($value)) {
        Add-Check "env-numeric-id:$($EnvPrefix)$Suffix" ($value -match "^\d+$") "$($EnvPrefix)$Suffix must be a numeric test user ID; value intentionally not printed."
    }
    return $value
}

function Import-EnvFile {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }

    $resolved = if ([System.IO.Path]::IsPathRooted($Path)) { $Path } else { Join-Path $Root $Path }
    $exists = Test-Path -LiteralPath $resolved -PathType Leaf
    Add-Check "env-file:exists" $exists "Env file path is validated; values are intentionally not printed."
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
            Add-Check "env-file:line-$lineNumber" $false "Env file line $lineNumber is not a supported `$env:NAME = 'value' assignment."
            continue
        }

        $name = $match.Groups[1].Value
        $value = $match.Groups[2].Value -replace "''", "'"
        [Environment]::SetEnvironmentVariable($name, $value, "Process")
        $loaded++
    }
    Add-Check "env-file:loaded-values" ($loaded -gt 0) "$loaded env value(s) loaded; values intentionally not printed."
}

function Join-Url {
    param(
        [string]$Base,
        [string]$Path
    )
    return $Base.TrimEnd("/") + "/" + $Path.TrimStart("/")
}

function Invoke-EndpointProbe {
    param(
        [string]$HttpMethod,
        [string]$TargetUrl,
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

        if ($HttpMethod -eq "GET") {
            $response = $client.GetAsync($TargetUrl).GetAwaiter().GetResult()
        } elseif ($HttpMethod -eq "POST") {
            if ($null -eq $Body) {
                $Body = ""
            }
            $content = New-Object System.Net.Http.StringContent -ArgumentList @($Body, [System.Text.Encoding]::UTF8, "application/json")
            $response = $client.PostAsync($TargetUrl, $content).GetAwaiter().GetResult()
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

function Add-EndpointPreflightCheck {
    param(
        [string]$Name,
        [int]$Status,
        [int[]]$AllowedStatuses
    )

    $allowedText = ($AllowedStatuses | ForEach-Object { [string]$_ }) -join "/"
    Add-Check $Name ($AllowedStatuses -contains $Status) "HTTP $Status. Expected one of $allowedText; 404, 5xx or connection failure means the selected service is not a valid 07 route target."
}

function ConvertTo-MarkdownCell {
    param([string]$Value)
    if ($null -eq $Value) {
        return ""
    }
    return (($Value -replace "\|", "\|" -replace "`r?`n", " ") -replace "\s+", " ").Trim()
}

function Write-ReadinessReport {
    param(
        [string]$Path,
        [bool]$Passed,
        [int]$FailedCount,
        [bool]$PreflightOnly
    )

    if ([string]::IsNullOrWhiteSpace($Path)) {
        return
    }

    $resolved = if ([System.IO.Path]::IsPathRooted($Path)) {
        [System.IO.Path]::GetFullPath($Path)
    } else {
        [System.IO.Path]::GetFullPath((Join-Path $Root $Path))
    }

    $parent = Split-Path -Parent $resolved
    if (-not [string]::IsNullOrWhiteSpace($parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }

    $mode = if ($PreflightOnly) { "endpoint-preflight-only" } else { "full-readiness" }
    $lines = New-Object System.Collections.Generic.List[string]
    $lines.Add("# 07 E2E Readiness Diagnostic")
    $lines.Add("")
    $lines.Add("Mode: $mode")
    $lines.Add("Result: $(if ($Passed) { "pass" } else { "fail" })")
    $lines.Add("Failing checks: $FailedCount")
    $lines.Add("Release evidence: no")
    $lines.Add("Env prefix: $EnvPrefix")
    if (-not [string]::IsNullOrWhiteSpace($EnvFile)) {
        $lines.Add("Env file: $EnvFile")
    }
    $lines.Add("")
    $lines.Add("## Checks")
    $lines.Add("")
    $lines.Add("| Check | Result | Detail |")
    $lines.Add("| --- | --- | --- |")
    foreach ($item in $results) {
        $lines.Add("| $(ConvertTo-MarkdownCell $item.Check) | $(ConvertTo-MarkdownCell $item.Result) | $(ConvertTo-MarkdownCell $item.Detail) |")
    }
    $lines.Add("")
    $lines.Add("## Release Boundary")
    $lines.Add("")
    if ($PreflightOnly) {
        $lines.Add("This diagnostic only proves that the selected local/dev service URLs and 07 route preflight checks are plausible. Token, model, persona, browser handoff, desktop launch, gateway request, package install, compatibility and Module J release evidence remain separate requirements.")
    } else {
        $lines.Add("This readiness check verifies execution inputs before E2E. A pass here is still not an E2E execution result and does not replace browser handoff, desktop launch, gateway request, package install, compatibility or Module J release evidence.")
    }

    Set-Content -LiteralPath $resolved -Encoding UTF8 -Value ($lines -join [Environment]::NewLine)
}

function Test-EndpointPreflight {
    param(
        [string]$BackendUrl,
        [string]$AdminUrl,
        [string]$GatewayUrl
    )

    if ([string]::IsNullOrWhiteSpace($BackendUrl)) {
        Add-Check "endpoint-preflight:client-bootstrap" $false "Cannot probe missing backend URL."
        Add-Check "endpoint-preflight:desktop-poll-route" $false "Cannot probe missing backend URL."
    } else {
        $clientStatus = Invoke-EndpointProbe -HttpMethod "GET" -TargetUrl (Join-Url $BackendUrl "/api/v1/client/bootstrap")
        Add-EndpointPreflightCheck "endpoint-preflight:client-bootstrap" $clientStatus @(401, 403)

        $desktopPollStatus = Invoke-EndpointProbe -HttpMethod "POST" -TargetUrl (Join-Url $BackendUrl "/api/v1/auth/desktop/poll") -Body "{}"
        Add-EndpointPreflightCheck "endpoint-preflight:desktop-poll-route" $desktopPollStatus @(400, 401, 403)
    }

    if ([string]::IsNullOrWhiteSpace($AdminUrl)) {
        Add-Check "endpoint-preflight:admin-codex-plus-config" $false "Cannot probe missing admin URL."
    } else {
        $adminStatus = Invoke-EndpointProbe -HttpMethod "GET" -TargetUrl (Join-Url $AdminUrl "/api/v1/admin/codex-plus/config")
        Add-EndpointPreflightCheck "endpoint-preflight:admin-codex-plus-config" $adminStatus @(401, 403, 423)
    }

    if ([string]::IsNullOrWhiteSpace($GatewayUrl)) {
        Add-Check "endpoint-preflight:gateway-responses" $false "Cannot probe missing gateway URL."
    } else {
        $gatewayStatus = Invoke-EndpointProbe -HttpMethod "POST" -TargetUrl (Join-Url $GatewayUrl "/v1/responses") -Body "{}"
        Add-EndpointPreflightCheck "endpoint-preflight:gateway-responses" $gatewayStatus @(400, 401, 403)
    }

    Add-Check "endpoint-preflight:desktop-start-not-mutated" $true "POST /api/v1/auth/desktop/start is not called because it creates a pending browser session; full handoff remains separate evidence."
}

Import-EnvFile $EnvFile

Test-RepoDir "sub2api-main"
Test-RepoDir "CodexPlusPlus-main"
Test-RepoDir "codex-plus-dev-plan"
Test-RepoFile "codex-plus-contracts/api/client-openapi.yaml"
Test-RepoFile "codex-plus-contracts/status-error/client-status-errors.md"
Test-RepoFile "codex-plus-contracts/events/client-events.schema.json"
Test-RepoFile "codex-plus-dev-plan/INTEGRATION-VERIFICATION-CHECKLIST.md"
Test-RepoFile "codex-plus-dev-plan/CODEX-AUTONOMOUS-TEST-RUNBOOK.md"

$backendUrl = Test-EnvRequired "BACKEND_BASE_URL" "url"
$adminUrl = Test-EnvRequired "ADMIN_BASE_URL" "url"
$managerPath = Test-EnvRequired "MANAGER_BUILD_PATH" "path"
if (-not [string]::IsNullOrWhiteSpace($managerPath)) {
    Add-Check "env-manager-build-windows-exe:$($EnvPrefix)MANAGER_BUILD_PATH" (Test-ManagerBuildExecutable $managerPath) "$($EnvPrefix)MANAGER_BUILD_PATH must be codex-plus-plus-manager.exe or a directory containing it on Windows."
}
$gatewayUrl = Get-EnvValue "GATEWAY_BASE_URL"
if ([string]::IsNullOrWhiteSpace($gatewayUrl)) {
    $gatewayUrl = $backendUrl
} elseif ($EndpointPreflight -or $EndpointPreflightOnly) {
    Add-Check "env-safe-nonproduction-url:$($EnvPrefix)GATEWAY_BASE_URL" ($AllowProduction -or (Test-SafeUrl $gatewayUrl)) "$($EnvPrefix)GATEWAY_BASE_URL must target local/dev/test/staging/sandbox/qa unless -AllowProduction is supplied."
}

if ($EndpointPreflightOnly) {
    Add-Check "mode:endpoint-preflight-only" $true "Token, model and persona checks are intentionally skipped for local route diagnostics; this mode is not release evidence."
} else {
    $adminToken = Test-EnvRequired "ADMIN_TOKEN" "secret"
    $activeToken = Test-EnvRequired "USER_ACTIVE_TOKEN" "secret"
    $notPurchasedToken = Test-EnvRequired "USER_NOT_PURCHASED_TOKEN" "secret"
    $expiredToken = Test-EnvRequired "USER_EXPIRED_TOKEN" "secret"
    $lowBalanceToken = Test-EnvRequired "USER_LOW_BALANCE_TOKEN" "secret"
    $revokedToken = Test-EnvRequired "USER_DEVICE_REVOKED_TOKEN" "secret"
    $modelDeniedToken = Test-EnvRequired "USER_MODEL_DENIED_TOKEN" "secret"
    $activeUserID = Test-EnvNumericID "USER_ACTIVE_ID"
    $notPurchasedUserID = Test-EnvNumericID "USER_NOT_PURCHASED_ID"
    $expiredUserID = Test-EnvNumericID "USER_EXPIRED_ID"
    $lowBalanceUserID = Test-EnvNumericID "USER_LOW_BALANCE_ID"
    $revokedUserID = Test-EnvNumericID "USER_DEVICE_REVOKED_ID"
    $modelDeniedUserID = Test-EnvNumericID "USER_MODEL_DENIED_ID"
    $deviceId = Test-EnvRequired "TEST_DEVICE_ID" "id"
    $allowedModel = Test-EnvRequired "ALLOWED_TEST_MODEL" "id"
    $deniedModel = Test-EnvRequired "DENIED_TEST_MODEL" "id"

    if (-not [string]::IsNullOrWhiteSpace($allowedModel) -and -not [string]::IsNullOrWhiteSpace($deniedModel)) {
        Add-Check "model-matrix:allowed-and-denied-differ" ($allowedModel -ne $deniedModel) "Allowed and denied model names must be different."
    }

    if (-not [string]::IsNullOrWhiteSpace($deviceId)) {
        Add-Check "device-id:shape" ($deviceId.Length -ge 6 -and $deviceId -notmatch "\s") "TEST_DEVICE_ID must be a stable non-whitespace test identifier."
    }
}

if ($ProbeHttp) {
    $probeTargets = @(
        @{ Name = "backend"; Url = $backendUrl },
        @{ Name = "admin"; Url = $adminUrl }
    )
    if (-not [string]::IsNullOrWhiteSpace($gatewayUrl)) {
        $probeTargets += @{ Name = "gateway"; Url = $gatewayUrl }
    }

    foreach ($target in $probeTargets) {
        if ([string]::IsNullOrWhiteSpace($target.Url)) {
            Add-Check "probe:$($target.Name)" $false "Cannot probe missing URL."
            continue
        }

        try {
            $response = Invoke-WebRequest -Uri $target.Url -Method Get -TimeoutSec 8 -MaximumRedirection 0 -ErrorAction Stop
            Add-Check "probe:$($target.Name)" $true "HTTP $($response.StatusCode) from $($target.Name) target."
        } catch {
            Add-Check "probe:$($target.Name)" $false "HTTP probe failed for $($target.Name); response body and credentials intentionally not printed."
        }
    }
}

if ($EndpointPreflight) {
    Test-EndpointPreflight $backendUrl $adminUrl $gatewayUrl
}

if (-not $EndpointPreflightOnly) {
    foreach ($tokenItem in @(
        @{ Name = "ADMIN_TOKEN"; Value = $adminToken },
        @{ Name = "USER_ACTIVE_TOKEN"; Value = $activeToken },
        @{ Name = "USER_NOT_PURCHASED_TOKEN"; Value = $notPurchasedToken },
        @{ Name = "USER_EXPIRED_TOKEN"; Value = $expiredToken },
        @{ Name = "USER_LOW_BALANCE_TOKEN"; Value = $lowBalanceToken },
        @{ Name = "USER_DEVICE_REVOKED_TOKEN"; Value = $revokedToken },
        @{ Name = "USER_MODEL_DENIED_TOKEN"; Value = $modelDeniedToken }
    )) {
        if (-not [string]::IsNullOrWhiteSpace($tokenItem.Value)) {
            Add-Check "secret-output-redaction:$($EnvPrefix)$($tokenItem.Name)" $true "Token presence checked; value intentionally not printed."
        }
    }

    foreach ($userIDItem in @(
        @{ Name = "USER_ACTIVE_ID"; Value = $activeUserID },
        @{ Name = "USER_NOT_PURCHASED_ID"; Value = $notPurchasedUserID },
        @{ Name = "USER_EXPIRED_ID"; Value = $expiredUserID },
        @{ Name = "USER_LOW_BALANCE_ID"; Value = $lowBalanceUserID },
        @{ Name = "USER_DEVICE_REVOKED_ID"; Value = $revokedUserID },
        @{ Name = "USER_MODEL_DENIED_ID"; Value = $modelDeniedUserID }
    )) {
        if (-not [string]::IsNullOrWhiteSpace($userIDItem.Value)) {
            Add-Check "admin-audit-user-id-ready:$($EnvPrefix)$($userIDItem.Name)" $true "Numeric user ID input is ready for admin audit correlation; value intentionally not printed."
        }
    }
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
Write-ReadinessReport -Path $OutputPath -Passed ($failed.Count -eq 0) -FailedCount $failed.Count -PreflightOnly ([bool]$EndpointPreflightOnly)
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 E2E readiness failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    Write-Host "This is a test-environment readiness failure, not an E2E execution result." -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Host "07 E2E readiness passed."
exit 0
