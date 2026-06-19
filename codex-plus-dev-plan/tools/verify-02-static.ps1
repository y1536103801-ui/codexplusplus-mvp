param(
    [string]$Root
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
}

$checks = New-Object System.Collections.Generic.List[object]

function Add-StaticCheck {
    param(
        [string]$Name,
        [bool]$Passed,
        [string]$Detail
    )
    $script:checks.Add([pscustomobject]@{
        Check = $Name
        Result = if ($Passed) { "PASS" } else { "FAIL" }
        Detail = $Detail
    })
}

function Read-RepoText {
    param([string]$Path)
    return Get-Content -Raw -LiteralPath (Join-Path $Root $Path)
}

function Test-RepoText {
    param(
        [string]$Path,
        [string]$Pattern,
        [string]$Name,
        [string]$Detail
    )
    $text = Read-RepoText $Path
    Add-StaticCheck $Name ($text -match $Pattern) $Detail
}

function Test-RepoTextNot {
    param(
        [string]$Path,
        [string]$Pattern,
        [string]$Name,
        [string]$Detail
    )
    $text = Read-RepoText $Path
    Add-StaticCheck $Name ($text -notmatch $Pattern) $Detail
}

$servicePath = "sub2api-main/backend/internal/service/codexplus_client.go"
$serviceTestPath = "sub2api-main/backend/internal/service/codexplus_client_test.go"
$handlerPath = "sub2api-main/backend/internal/handler/client/client_handler.go"
$dtoPath = "sub2api-main/backend/internal/handler/dto/codexplus_client.go"
$routePath = "sub2api-main/backend/internal/server/routes/client.go"
$wirePath = "sub2api-main/backend/internal/service/wire.go"
$foundationPath = "sub2api-main/backend/internal/service/codexplus_foundation.go"
$ledgerPath = "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
$statusPath = "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md"
$tracePath = "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md"

foreach ($path in @(
    $servicePath,
    $serviceTestPath,
    $handlerPath,
    $dtoPath,
    $routePath,
    $wirePath,
    $foundationPath,
    $ledgerPath,
    $statusPath,
    $tracePath,
    "codex-plus-contracts/api/client-openapi.yaml",
    "codex-plus-contracts/test-fixtures/client/bootstrap.available.json",
    "codex-plus-contracts/test-fixtures/client/usage.available.json",
    "codex-plus-contracts/test-fixtures/client/devices.registered.json",
    "codex-plus-contracts/test-fixtures/client/redeem.applied.json"
)) {
    Add-StaticCheck "file-exists:$path" (Test-Path -LiteralPath (Join-Path $Root $path) -PathType Leaf) $path
}

foreach ($route in @('client.GET\("/bootstrap"', 'client.GET\("/usage"', 'client.POST\("/devices"', 'client.POST\("/redeem"')) {
    Test-RepoText $routePath $route "route:$route" "Client route must be registered."
}

foreach ($symbol in @(
    "func \(s \*CodexPlusClientService\) Bootstrap",
    "func \(s \*CodexPlusClientService\) ClientUsage",
    "func \(s \*CodexPlusClientService\) UpsertDevice",
    "func \(s \*CodexPlusClientService\) Redeem",
    "type CodexPlusClientRedeemer interface",
    "CodexPlusClientPlanForSubscription"
)) {
    if ($symbol -eq "CodexPlusClientPlanForSubscription") {
        Test-RepoText $servicePath "codexPlusClientPlanForSubscription" "service-symbol:$symbol" "Client entitlement must resolve plans from configured entitlement sources."
    } else {
        Test-RepoText $servicePath $symbol "service-symbol:$symbol" "$symbol must exist."
    }
}

foreach ($symbol in @(
    "codexPlusClientUsagePolicyForPlan",
    "codexPlusClientDefaultModelForPlan",
    "codexPlusClientModelAllowedByPlan",
    "containsCodexPlusInt64",
    "containsCodexPlusStringFold",
    "isCodexPlusPlanEnabled"
)) {
    Test-RepoText $servicePath $symbol "policy-source:$symbol" "$symbol must be used by 02 so usage status and model visibility share the 01 config source."
}

Test-RepoTextNot $servicePath "\bfirstPlan\s*\(" "no-first-plan-fallback" "Client API must not select the first configured plan by position."
Test-RepoTextNot $servicePath "\bfirstUsagePolicy\s*\(" "no-first-usage-policy-fallback" "Client API must not select the first configured usage policy by position."

foreach ($field in @(
    "message_key",
    "commerce_action",
    "action_copy_key",
    "balance_summary",
    "period_usage",
    "strict_device_enforcement",
    "force_update_prompt",
    "announcements"
)) {
    Test-RepoText $dtoPath $field "dto-field:$field" "$field must be exposed in client DTOs."
}

foreach ($symbol in @(
    "successEnvelope",
    "clientRequestID",
    "ClientUsage",
    "RequestID",
    "clientBootstrapMessage",
    "clientRedeemMessage"
)) {
    Test-RepoText $handlerPath $symbol "handler-contract:$symbol" "$symbol must be present in the client handler contract boundary."
}

foreach ($eventSymbol in @(
    "bootstrap_requested",
    "usage_requested",
    "device_registered",
    "redeem_attempted",
    "RequestID",
    "ConfigVersion"
)) {
    Test-RepoText $servicePath $eventSymbol "event-context:$eventSymbol" "$eventSymbol must be represented in structured client events."
}
Test-RepoText $wirePath "ConfigVersion:\s*stringPtrIfNotBlank\(event\.ConfigVersion\)" "event-store-config-version" "Event persistence must preserve config version when present."
Test-RepoText $wirePath "RequestID:\s*stringPtrIfNotBlank\(event\.RequestID\)" "event-store-request-id" "Event persistence must preserve request id when present."

foreach ($testName in @(
    "TestCodexPlusClientBootstrapCreatesAndReusesManagedKey",
    "TestCodexPlusClientBootstrapIncludesContractFieldsAndEventContext",
    "TestCodexPlusClientUsageReturnsContractShapeAndEvent",
    "TestCodexPlusClientBootstrapSelectsPlanPolicyAndModelsFromEntitlement",
    "TestCodexPlusClientUpsertDeviceIsIdempotentAndEmitsEvent",
    "TestCodexPlusClientRedeemMapsStatusesAndEmitsEvent",
    "TestCodexPlusClientBootstrapNotPurchasedDoesNotCreateKey",
    "TestCodexPlusClientBootstrapRevokedDeviceSuppressesKey"
)) {
    Test-RepoText $serviceTestPath $testName "service-test:$testName" "$testName must exist."
}

Test-RepoText $ledgerPath "\| ``00-contract`` \|[^\r\n]*\| passed \|" "00-passed-before-02" "00-contract must be passed before 02 completion."
Test-RepoText $ledgerPath "\| ``01-backend-config-center`` \|[^\r\n]*\| passed \|" "01-passed-before-02" "01-backend-config-center must be passed before 02 completion."
Test-RepoText $ledgerPath "\| ``02-backend-client-api`` \|[^\r\n]*\| (active|passed) \|" "02-active-or-passed" "02 must be active during verification or passed after completion."
foreach ($stageNumber in 3..7) {
    $stagePrefix = "{0:D2}" -f $stageNumber
    $allowed = if ($stageNumber -eq 3) { "(blocked|active)" } else { "blocked" }
    Test-RepoText $ledgerPath "\| ``$stagePrefix-[^``]+`` \|[^\r\n]*\| $allowed \|" "stage-$stagePrefix-sequential" "$stagePrefix must respect sequential gate state."
}
Test-RepoText $tracePath "02-Backend Client API" "trace-02-present" "Trace must include the 02 stage boundary or completion evidence."

$checks | Format-Table -AutoSize

$failed = @($checks | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "02 static audit failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "02 static audit passed." -ForegroundColor Green
exit 0
