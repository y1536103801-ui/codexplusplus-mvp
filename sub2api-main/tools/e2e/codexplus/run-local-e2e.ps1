param(
    [string]$Root,
    [string]$OutputRoot,
    [string]$Timestamp,
    [string]$EnvPrefix = "CODEXPLUS_07_E2E_",
    [string]$EnvFile,
    [switch]$FixtureMode,
    [switch]$AllowProduction,
    [switch]$ProbeHttp,
    [switch]$EndpointPreflight,
    [switch]$SkipDeviceRegistration,
    [switch]$RunBrowserHandoff,
    [switch]$AllowSessionStart,
    [switch]$AllowBrowserComplete,
    [switch]$AllowPartialBrowserHandoff,
    [switch]$RunGatewayPolicy,
    [switch]$AllowGatewayRequests,
    [switch]$RunAdminAudit,
    [switch]$AllowAdminAuditReads,
    [switch]$StartMockOpenAI,
    [switch]$ReplaceMockOpenAI,
    [int]$MockOpenAIPort = 18081,
    [switch]$AllowRedeem,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..\..\..")
} else {
    $Root = Resolve-Path $Root
}

$PlanRoot = Join-Path $Root "codex-plus-dev-plan"
$Generator = Join-Path $PlanRoot "tools\new-07-evidence-run.ps1"
$ClientRunner = Join-Path $PSScriptRoot "run-client-api-checks.ps1"
$BrowserHandoffRunner = Join-Path $PSScriptRoot "run-browser-handoff-checks.ps1"
$GatewayRunner = Join-Path $PSScriptRoot "run-gateway-policy-checks.ps1"
$AdminAuditRunner = Join-Path $PSScriptRoot "run-admin-audit-checks.ps1"
$MockOpenAIRunner = Join-Path $PSScriptRoot "start-local-mock-openai-upstream.ps1"

if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $PlanRoot "test-runs"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputRoot)) {
    $OutputRoot = Join-Path $Root $OutputRoot
}

if ([string]::IsNullOrWhiteSpace($Timestamp)) {
    $Timestamp = Get-Date -Format "yyyyMMdd-HHmm"
}

if ($Timestamp -match "^\d{8}-\d{4}-e2e$") {
    $runName = $Timestamp
} elseif ($Timestamp -match "^\d{8}-\d{4}$") {
    $runName = "$Timestamp-e2e"
} else {
    throw "Timestamp must match YYYYMMDD-HHMM or YYYYMMDD-HHMM-e2e."
}

$runPath = Join-Path $OutputRoot $runName

if ($StartMockOpenAI) {
    $mockArgs = @("-Port", [string]$MockOpenAIPort)
    if ($ReplaceMockOpenAI) { $mockArgs += "-ReplaceExisting" }
    & powershell -NoProfile -ExecutionPolicy Bypass -File $MockOpenAIRunner @mockArgs
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
}

$generatorArgs = @("-Root", $Root, "-OutputRoot", $OutputRoot, "-Timestamp", $Timestamp)
if ($Force) { $generatorArgs += "-Force" }
& powershell -NoProfile -ExecutionPolicy Bypass -File $Generator @generatorArgs
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

$clientArgs = @("-Root", $Root, "-EvidenceDir", $runPath, "-EnvPrefix", $EnvPrefix)
if ($EnvFile) { $clientArgs += @("-EnvFile", $EnvFile) }
if ($FixtureMode) { $clientArgs += "-FixtureMode" }
if ($AllowProduction) { $clientArgs += "-AllowProduction" }
if ($ProbeHttp) { $clientArgs += "-ProbeHttp" }
if ($EndpointPreflight) { $clientArgs += "-EndpointPreflight" }
if ($SkipDeviceRegistration) { $clientArgs += "-SkipDeviceRegistration" }
if ($AllowRedeem) { $clientArgs += "-AllowRedeem" }

& powershell -NoProfile -ExecutionPolicy Bypass -File $ClientRunner @clientArgs
$exitCode = $LASTEXITCODE
if ($exitCode -ne 0) {
    Write-Host ""
    Write-Host "07 local E2E scaffold path: $runPath"
    Write-Host "Client API subset failed; browser handoff and gateway policy runners were not started."
    exit $exitCode
}

if ($RunBrowserHandoff) {
    $handoffArgs = @("-Root", $Root, "-EvidenceDir", $runPath, "-EnvPrefix", $EnvPrefix)
    if ($EnvFile) { $handoffArgs += @("-EnvFile", $EnvFile) }
    if ($FixtureMode) { $handoffArgs += "-FixtureMode" }
    if ($AllowProduction) { $handoffArgs += "-AllowProduction" }
    if ($AllowSessionStart) { $handoffArgs += "-AllowSessionStart" }
    if ($AllowBrowserComplete) { $handoffArgs += "-AllowBrowserComplete" }
    if ($AllowPartialBrowserHandoff) { $handoffArgs += "-AllowPartial" }
    & powershell -NoProfile -ExecutionPolicy Bypass -File $BrowserHandoffRunner @handoffArgs
    $exitCode = $LASTEXITCODE
}

if ($exitCode -ne 0) {
    Write-Host ""
    Write-Host "07 local E2E scaffold path: $runPath"
    Write-Host "Browser handoff subset failed; gateway policy runner was not started."
    exit $exitCode
}

if ($RunGatewayPolicy) {
    $gatewayArgs = @("-Root", $Root, "-EvidenceDir", $runPath, "-EnvPrefix", $EnvPrefix)
    if ($EnvFile) { $gatewayArgs += @("-EnvFile", $EnvFile) }
    if ($FixtureMode) { $gatewayArgs += "-FixtureMode" }
    if ($AllowProduction) { $gatewayArgs += "-AllowProduction" }
    if ($AllowGatewayRequests) { $gatewayArgs += "-AllowGatewayRequests" }
    & powershell -NoProfile -ExecutionPolicy Bypass -File $GatewayRunner @gatewayArgs
    $exitCode = $LASTEXITCODE
}

if ($exitCode -ne 0) {
    Write-Host ""
    Write-Host "07 local E2E scaffold path: $runPath"
    Write-Host "Gateway policy subset failed; admin audit runner was not started."
    exit $exitCode
}

if ($RunAdminAudit) {
    $adminAuditArgs = @("-Root", $Root, "-EvidenceDir", $runPath, "-EnvPrefix", $EnvPrefix)
    if ($EnvFile) { $adminAuditArgs += @("-EnvFile", $EnvFile) }
    if ($FixtureMode) { $adminAuditArgs += "-FixtureMode" }
    if ($AllowProduction) { $adminAuditArgs += "-AllowProduction" }
    if ($AllowAdminAuditReads) { $adminAuditArgs += "-AllowAdminAuditReads" }
    & powershell -NoProfile -ExecutionPolicy Bypass -File $AdminAuditRunner @adminAuditArgs
    $exitCode = $LASTEXITCODE
}

Write-Host ""
Write-Host "07 local E2E scaffold path: $runPath"
Write-Host "This runner fills scaffolded evidence for selected subsets only. Desktop launch, package install, compatibility migration and payment still require separate execution."

exit $exitCode
