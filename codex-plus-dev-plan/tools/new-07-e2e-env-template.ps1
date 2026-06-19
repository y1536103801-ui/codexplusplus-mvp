param(
    [string]$Root,
    [string]$OutputRoot,
    [string]$Timestamp,
    [string]$BackendBaseUrl = "http://localhost:8080",
    [string]$AdminBaseUrl = "http://localhost:8080",
    [string]$GatewayBaseUrl = "http://localhost:8080",
    [string]$ManagerBuildPath,
    [string]$EnvPrefix = "CODEXPLUS_07_E2E_",
    [switch]$ProbeHttp,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $Root "codex-plus-dev-plan\test-runs"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot = Join-Path $Root $OutputRoot
}

if ([string]::IsNullOrWhiteSpace($Timestamp)) {
    $Timestamp = Get-Date -Format "yyyyMMdd-HHmm"
}

if ($Timestamp -match "^\d{8}-\d{4}-e2e-env$") {
    $runName = $Timestamp
} elseif ($Timestamp -match "^\d{8}-\d{4}$") {
    $runName = "$Timestamp-e2e-env"
} else {
    throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-e2e-env."
}

if ([string]::IsNullOrWhiteSpace($ManagerBuildPath)) {
    $candidateManagerBuildPaths = @(
        (Join-Path $Root "CodexPlusPlus-main\dist\windows\app\codex-plus-plus-manager.exe"),
        (Join-Path $Root "CodexPlusPlus-main\dist\windows\app"),
        (Join-Path $Root "CodexPlusPlus-main\apps\codex-plus-manager\src-tauri\target\release\codex-plus-plus-manager.exe"),
        (Join-Path $Root "CodexPlusPlus-main\apps\codex-plus-manager\dist")
    )
    $ManagerBuildPath = @($candidateManagerBuildPaths | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1)
    if ([string]::IsNullOrWhiteSpace($ManagerBuildPath)) {
        $ManagerBuildPath = $candidateManagerBuildPaths[0]
    }
} elseif (-not [System.IO.Path]::IsPathRooted($ManagerBuildPath)) {
    $ManagerBuildPath = Join-Path $Root $ManagerBuildPath
}

$runPath = Join-Path $OutputRoot $runName
if ((Test-Path -LiteralPath $runPath) -and -not $Force) {
    throw "E2E env template already exists: $runPath. Use -Force only when intentionally regenerating placeholders."
}

New-Item -ItemType Directory -Force -Path $runPath | Out-Null

function Test-UrlProbe {
    param([string]$Url)
    if (-not $ProbeHttp) {
        return "not run"
    }

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $Url -Method Get -TimeoutSec 8 -MaximumRedirection 0 -ErrorAction Stop
        return "HTTP $($response.StatusCode)"
    } catch {
        return "failed"
    }
}

function ConvertTo-EnvLine {
    param(
        [string]$Name,
        [string]$Value
    )
    $escaped = $Value -replace "'", "''"
    return "`$env:$Name = '$escaped'"
}

$backendHealthUrl = $BackendBaseUrl.TrimEnd("/") + "/health"
$backendProbe = Test-UrlProbe $backendHealthUrl
$adminProbe = Test-UrlProbe $AdminBaseUrl
$gatewayProbe = Test-UrlProbe $GatewayBaseUrl
$managerExists = Test-Path -LiteralPath $ManagerBuildPath

$envValues = [ordered]@{
    "BACKEND_BASE_URL" = $BackendBaseUrl
    "ADMIN_BASE_URL" = $AdminBaseUrl
    "MANAGER_BUILD_PATH" = $ManagerBuildPath
    "ADMIN_TOKEN" = "<fill-admin-jwt-or-test-admin-token>"
    "USER_ACTIVE_TOKEN" = "<fill-active-user-client-token>"
    "USER_NOT_PURCHASED_TOKEN" = "<fill-not-purchased-user-client-token>"
    "USER_EXPIRED_TOKEN" = "<fill-expired-user-client-token>"
    "USER_LOW_BALANCE_TOKEN" = "<fill-low-balance-user-client-token>"
    "USER_DEVICE_REVOKED_TOKEN" = "<fill-device-revoked-user-client-token>"
    "USER_MODEL_DENIED_TOKEN" = "<fill-model-denied-user-client-token>"
    "USER_ACTIVE_ID" = "<fill-active-user-numeric-id>"
    "USER_NOT_PURCHASED_ID" = "<fill-not-purchased-user-numeric-id>"
    "USER_EXPIRED_ID" = "<fill-expired-user-numeric-id>"
    "USER_LOW_BALANCE_ID" = "<fill-low-balance-user-numeric-id>"
    "USER_DEVICE_REVOKED_ID" = "<fill-device-revoked-user-numeric-id>"
    "USER_MODEL_DENIED_ID" = "<fill-model-denied-user-numeric-id>"
    "TEST_DEVICE_ID" = "codexplus-07-e2e-device"
    "ALLOWED_TEST_MODEL" = "<fill-allowed-model>"
    "DENIED_TEST_MODEL" = "<fill-denied-model>"
    "GATEWAY_BASE_URL" = $GatewayBaseUrl
    "USER_ACTIVE_GATEWAY_KEY" = "<fill-active-user-gateway-key>"
    "USER_NOT_PURCHASED_GATEWAY_KEY" = "<fill-not-purchased-user-gateway-key>"
    "USER_EXPIRED_GATEWAY_KEY" = "<fill-expired-user-gateway-key>"
    "USER_LOW_BALANCE_GATEWAY_KEY" = "<fill-low-balance-user-gateway-key>"
    "USER_DEVICE_REVOKED_GATEWAY_KEY" = "<fill-device-revoked-user-gateway-key>"
    "USER_MODEL_DENIED_GATEWAY_KEY" = "<fill-model-denied-user-gateway-key>"
    "TEST_REDEEM_CODE" = "<optional-test-redeem-code>"
}

$envLines = New-Object System.Collections.Generic.List[string]
$envLines.Add("# 07 E2E environment template generated $runName.")
$envLines.Add("# Fill placeholder values before running release E2E. Do not commit real token values.")
foreach ($key in $envValues.Keys) {
    $envLines.Add((ConvertTo-EnvLine ($EnvPrefix + $key) $envValues[$key]))
}

$envFile = Join-Path $runPath "e2e-env.template.ps1"
Set-Content -LiteralPath $envFile -Encoding UTF8 -Value ($envLines -join [Environment]::NewLine)

$checklist = @"
# 07 E2E Environment Template

Run folder: $runName
Status: generated

## Detected Inputs

- Backend base URL: $BackendBaseUrl
- Backend health probe: $backendProbe
- Admin base URL: $AdminBaseUrl
- Admin probe: $adminProbe
- Gateway base URL: $GatewayBaseUrl
- Gateway probe: $gatewayProbe
- Manager build path: $ManagerBuildPath
- Manager build path exists: $managerExists

## Required Manual Inputs

- `$($EnvPrefix)ADMIN_TOKEN`
- `$($EnvPrefix)USER_ACTIVE_TOKEN`
- `$($EnvPrefix)USER_NOT_PURCHASED_TOKEN`
- `$($EnvPrefix)USER_EXPIRED_TOKEN`
- `$($EnvPrefix)USER_LOW_BALANCE_TOKEN`
- `$($EnvPrefix)USER_DEVICE_REVOKED_TOKEN`
- `$($EnvPrefix)USER_MODEL_DENIED_TOKEN`
- `$($EnvPrefix)USER_ACTIVE_ID`
- `$($EnvPrefix)USER_NOT_PURCHASED_ID`
- `$($EnvPrefix)USER_EXPIRED_ID`
- `$($EnvPrefix)USER_LOW_BALANCE_ID`
- `$($EnvPrefix)USER_DEVICE_REVOKED_ID`
- `$($EnvPrefix)USER_MODEL_DENIED_ID`
- `$($EnvPrefix)ALLOWED_TEST_MODEL`
- `$($EnvPrefix)DENIED_TEST_MODEL`
- `$($EnvPrefix)USER_ACTIVE_GATEWAY_KEY`
- `$($EnvPrefix)USER_NOT_PURCHASED_GATEWAY_KEY`
- `$($EnvPrefix)USER_EXPIRED_GATEWAY_KEY`
- `$($EnvPrefix)USER_LOW_BALANCE_GATEWAY_KEY`
- `$($EnvPrefix)USER_DEVICE_REVOKED_GATEWAY_KEY`
- `$($EnvPrefix)USER_MODEL_DENIED_GATEWAY_KEY`

## Commands

```powershell
. "$envFile"
powershell -NoProfile -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1 -Root (Resolve-Path .) -EnvPrefix $EnvPrefix -ProbeHttp
powershell -NoProfile -ExecutionPolicy Bypass -File sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1 -Root (Resolve-Path .) -OutputRoot codex-plus-dev-plan/test-runs -EnvPrefix $EnvPrefix -ProbeHttp -RunGatewayPolicy -AllowGatewayRequests -RunAdminAudit -AllowAdminAuditReads
```

## Release Boundary

This generated template is not release evidence. It only records the local/default values and placeholders needed before the 07 E2E runners can produce sanitized execution evidence.
"@

$checklistFile = Join-Path $runPath "e2e-env-checklist.md"
Set-Content -LiteralPath $checklistFile -Encoding UTF8 -Value $checklist

Write-Host "Created 07 E2E env template: $runPath"
Write-Host "Template: $envFile"
Write-Host "Checklist: $checklistFile"
if ($ProbeHttp) {
    Write-Host "Backend health probe: $backendProbe"
    Write-Host "Admin probe: $adminProbe"
    Write-Host "Gateway probe: $gatewayProbe"
}
