param(
    [string]$Root,
    [string]$Stage = "current",
    [switch]$Strict
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Root)) {
    $Root = Resolve-Path (Join-Path $PSScriptRoot "..\..")
} else {
    $Root = Resolve-Path $Root
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

function Read-Text {
    param([string]$Path)
    return Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $Root $Path)
}

function Test-RequiredFiles {
    param([string[]]$Files)
    foreach ($file in $Files) {
        $fullPath = Join-Path $Root $file
        Add-Check "required-file:$file" (Test-Path -LiteralPath $fullPath -PathType Leaf) $file
    }
}

function Test-TextContains {
    param(
        [string]$Path,
        [string]$Pattern,
        [string]$CheckName,
        [string]$Detail
    )
    $text = Read-Text $Path
    Add-Check $CheckName ($text -match $Pattern) $Detail
}

function Test-JsonContracts {
    $jsonFiles = Get-ChildItem -LiteralPath (Join-Path $Root "codex-plus-contracts") -Recurse -Filter "*.json"
    foreach ($jsonFile in $jsonFiles) {
        try {
            Get-Content -Raw -Encoding UTF8 -LiteralPath $jsonFile.FullName | ConvertFrom-Json | Out-Null
            Add-Check "json-parse:$($jsonFile.Name)" $true $jsonFile.FullName
        } catch {
            Add-Check "json-parse:$($jsonFile.Name)" $false $_.Exception.Message
        }
    }
}

function Test-FinalReport {
    param(
        [string]$Lane,
        [string]$File,
        [string[]]$RequiredSections,
        [string[]]$DecisionItems = @()
    )
    $fullPath = Join-Path $Root $File
    $exists = Test-Path -LiteralPath $fullPath -PathType Leaf
    Add-Check "worker-report-exists:$File" $exists $File
    if (-not $exists) {
        return
    }

    $reportText = Get-Content -Raw -Encoding UTF8 -LiteralPath $fullPath
    Add-Check "worker-report-final:$Lane" ($reportText -match "(?im)^Report status:\s*final\s*$") "Report must contain 'Report status: final'."
    Add-Check "worker-report-lane:$Lane" ($reportText -match "(?im)^Worker lane:\s*$Lane\s*$") "Report lane must match assigned worker."
    Add-Check "worker-report-forbidden-edits:$Lane" ($reportText -match "(?im)^Forbidden edits:\s*none\s*$") "Report must state forbidden edits: none."
    Add-Check "worker-report-no-placeholders:$Lane" ($reportText -notmatch "(?i)\b(TODO|PLACEHOLDER|NOT FINAL)\b") "Final report must not contain placeholder or non-final markers."
    foreach ($section in $RequiredSections) {
        Add-Check "worker-report-section:${Lane}:$section" ($reportText -match "(?im)^##\s+$([regex]::Escape($section))\s*$") "$section must be present."
    }
    foreach ($item in $DecisionItems) {
        $itemPattern = "(?im)(^\s*-\s*$item\s*:\s*(fixed|deferred|rejected)\b|^\s*\|\s*$item\s*\|\s*(fixed|deferred|rejected)\s*\|)"
        Add-Check "worker-report-item:$item" ($reportText -match $itemPattern) "$item must have fixed, deferred or rejected decision."
    }
}

function Test-NoResidue {
    param(
        [string[]]$Files,
        [string[]]$Patterns
    )
    foreach ($pattern in $Patterns) {
        $hits = @()
        foreach ($file in $Files) {
            $text = Read-Text $file
            if ($text -match $pattern) {
                $hits += $file
            }
        }
        Add-Check "no-current-residue:$pattern" ($hits.Count -eq 0) ($hits -join ", ")
    }
}

function Resolve-CurrentStage {
    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    if ($ledger -match "\| ``07-integration-release`` \|[^\r\n]*\| active \|") {
        return "07-integration-release"
    }
    if ($ledger -match "\| ``06-commerce-and-enforcement`` \|[^\r\n]*\| active \|") {
        return "06-commerce-and-enforcement"
    }
    if ($ledger -match "\| ``05-admin-operations`` \|[^\r\n]*\| active \|") {
        return "05-admin-operations"
    }
    if ($ledger -match "\| ``04-client-user-experience`` \|[^\r\n]*\| active \|") {
        return "04-client-user-experience"
    }
    if ($ledger -match "\| ``03-client-cloud-core`` \|[^\r\n]*\| active \|") {
        return "03-client-cloud-core"
    }
    if ($ledger -match "\| ``02-backend-client-api`` \|[^\r\n]*\| active \|") {
        return "02-backend-client-api"
    }
    if ($ledger -match "\| ``01-backend-config-center`` \|[^\r\n]*\| active \|") {
        return "01-backend-config-center"
    }
    if ($ledger -match "\| ``00-contract`` \|[^\r\n]*\| active \|") {
        return "00-contract"
    }
    return "00-contract"
}

if ($Stage -eq "current") {
    $Stage = Resolve-CurrentStage
}

$supportedStages = @("00-contract", "01-backend-config-center", "02-backend-client-api", "03-client-cloud-core", "04-client-user-experience", "05-admin-operations", "06-commerce-and-enforcement", "07-integration-release")
Add-Check "stage-supported" ($supportedStages -contains $Stage) $Stage

if ($Stage -eq "00-contract") {
    Test-RequiredFiles @(
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/CONTRACT-GATE.md",
        "codex-plus-dev-plan/00-contract/README.md",
        "codex-plus-dev-plan/00-contract/PARALLEL-RESTART-PACK.md",
        "codex-plus-dev-plan/00-contract/COORDINATOR-PREAUDIT.md",
        "codex-plus-contracts/api/client-openapi.yaml",
        "codex-plus-contracts/config/plan-catalog.schema.json",
        "codex-plus-contracts/config/model-catalog.schema.json",
        "codex-plus-contracts/config/usage-policy.schema.json",
        "codex-plus-contracts/config/feature-flags.schema.json",
        "codex-plus-contracts/status-error/client-status-errors.md",
        "codex-plus-contracts/events/client-events.schema.json",
        "codex-plus-contracts/compatibility-matrix.md",
        "codex-plus-contracts/change-review-policy.md",
        "codex-plus-contracts/storage-decision.md"
    )

    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    if ($Strict) {
        Add-Check "00-status" ($ledger -match "\| ``00-contract`` \|[^\r\n]*\| active \|") "00-contract must be active in strict pre-pass mode."
    } else {
        Add-Check "00-status" ($ledger -match "\| ``00-contract`` \|[^\r\n]*\| (active|passed) \|") "00-contract must be active or passed."
    }
    foreach ($stageNumber in 2..7) {
        $stagePrefix = "{0:D2}" -f $stageNumber
        Add-Check "stage-$stagePrefix-blocked" ($ledger -match "\| ``$stagePrefix-[^``]+`` \|[^\r\n]*\| blocked \|") "$stagePrefix must remain blocked before later gates."
    }

    $restartPack = Read-Text "codex-plus-dev-plan/00-contract/PARALLEL-RESTART-PACK.md"
    Add-Check "restart-pack-preaudit-reference" ($restartPack -match "COORDINATOR-PREAUDIT\.md") "Restart pack must require workers to read the preaudit."
    foreach ($itemGroup in @("A1-A5", "B1-B4", "C1-C4")) {
        Add-Check "restart-pack-items:$itemGroup" ($restartPack -match $itemGroup) "$itemGroup must be referenced."
    }

    $preaudit = Read-Text "codex-plus-dev-plan/00-contract/COORDINATOR-PREAUDIT.md"
    foreach ($item in @("A1","A2","A3","A4","A5","B1","B2","B3","B4","C1","C2","C3","C4")) {
        Add-Check "preaudit-item:$item" ($preaudit -match "### $item\.") "$item must be present."
    }

    $openapi = Read-Text "codex-plus-contracts/api/client-openapi.yaml"
    foreach ($path in @("/api/v1/auth/desktop/start", "/api/v1/auth/desktop/complete", "/api/v1/auth/desktop/poll", "/api/v1/client/bootstrap", "/api/v1/client/usage", "/api/v1/client/devices", "/api/v1/client/redeem")) {
        Add-Check "openapi-path:$path" ($openapi.Contains($path)) $path
    }

    Test-JsonContracts
    Test-FinalReport "A" "codex-plus-dev-plan/00-contract/reports/worker-a-client-api-final.md" @("Changed Files", "Verification", "Preaudit Answers", "Downstream Assumptions") @("A1","A2","A3","A4","A5")
    Test-FinalReport "B" "codex-plus-dev-plan/00-contract/reports/worker-b-admin-config-final.md" @("Changed Files", "Verification", "Preaudit Answers", "Downstream Assumptions") @("B1","B2","B3","B4")
    Test-FinalReport "C" "codex-plus-dev-plan/00-contract/reports/worker-c-status-error-event-final.md" @("Changed Files", "Verification", "Preaudit Answers", "Downstream Assumptions") @("C1","C2","C3","C4")
}

if ($Stage -eq "01-backend-config-center") {
    Test-RequiredFiles @(
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
        "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md",
        "codex-plus-dev-plan/tools/verify-01-static.ps1",
        "codex-plus-dev-plan/tools/verify-01-go.ps1",
        "codex-plus-dev-plan/01-backend-config-center/README.md",
        "codex-plus-dev-plan/01-backend-config-center/task-plan-catalog.md",
        "codex-plus-dev-plan/01-backend-config-center/task-model-catalog.md",
        "codex-plus-dev-plan/01-backend-config-center/task-usage-policy.md",
        "codex-plus-dev-plan/01-backend-config-center/task-feature-flags.md",
        "codex-plus-dev-plan/01-backend-config-center/reports/worker-plan-catalog-final.md",
        "codex-plus-dev-plan/01-backend-config-center/reports/worker-model-catalog-final.md",
        "codex-plus-dev-plan/01-backend-config-center/reports/worker-usage-policy-final.md",
        "codex-plus-dev-plan/01-backend-config-center/reports/worker-feature-flags-final.md",
        "sub2api-main/backend/internal/codexplus/configregistry/common.go",
        "sub2api-main/backend/internal/codexplus/configregistry/plan_catalog.go",
        "sub2api-main/backend/internal/codexplus/configregistry/model_catalog.go",
        "sub2api-main/backend/internal/codexplus/configregistry/usage_policy.go",
        "sub2api-main/backend/internal/codexplus/configregistry/feature_flags.go",
        "sub2api-main/backend/internal/service/codexplus_config_service.go",
        "sub2api-main/backend/internal/service/codexplus_config_service_test.go"
    )

    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    Add-Check "00-passed-before-01" ($ledger -match "\| ``00-contract`` \|[^\r\n]*\| passed \|") "00-contract must be passed before 01."
    Add-Check "01-active-or-passed" (($ledger -match "\| ``01-backend-config-center`` \|[^\r\n]*\| active \|") -or ($ledger -match "\| ``01-backend-config-center`` \|[^\r\n]*\| passed \|")) "01-backend-config-center must be active during integration or passed after verification."
    Add-Check "stage-02-sequential" (($ledger -match "\| ``02-backend-client-api`` \|[^\r\n]*\| blocked \|") -or ($ledger -match "\| ``02-backend-client-api`` \|[^\r\n]*\| active \|")) "02 may be blocked during 01 or active after 01 passes."
    foreach ($stageNumber in 3..7) {
        $stagePrefix = "{0:D2}" -f $stageNumber
        Add-Check "stage-$stagePrefix-blocked" ($ledger -match "\| ``$stagePrefix-[^``]+`` \|[^\r\n]*\| blocked \|") "$stagePrefix must remain blocked until its prior gate passes."
    }

    Test-FinalReport "P" "codex-plus-dev-plan/01-backend-config-center/reports/worker-plan-catalog-final.md" @("Changed files", "Verification", "Downstream assumptions", "Remaining risks")
    Test-FinalReport "M" "codex-plus-dev-plan/01-backend-config-center/reports/worker-model-catalog-final.md" @("Changed files", "Verification", "Downstream assumptions", "Remaining risks")
    Test-FinalReport "U" "codex-plus-dev-plan/01-backend-config-center/reports/worker-usage-policy-final.md" @("Changed files", "Verification", "Downstream assumptions", "Remaining risks")
    Test-FinalReport "F" "codex-plus-dev-plan/01-backend-config-center/reports/worker-feature-flags-final.md" @("Changed files", "Verification", "Downstream assumptions", "Remaining risks")

    Test-TextContains "sub2api-main/backend/internal/service/codexplus_config_service.go" "configregistry" "service-imports-configregistry" "Config service must consume the 01 registry package."
    foreach ($symbol in @("DefaultPlanCatalog", "DefaultModelCatalog", "DefaultUsagePolicyCatalog", "DefaultFeatureFlags", "ValidatePlanCatalog", "ValidateModelCatalog", "ValidateUsagePolicyCatalog", "ValidateFeatureFlags", "codexPlusAlignDefaultConfigReferences")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_config_service.go" $symbol "service-symbol:$symbol" "$symbol must be referenced by config service integration."
    }
    foreach ($symbol in @("PriceAmountMinor", "UsagePolicyID", "CopyKeys", "Exposure", "StrictDeviceEnforcement", "EntitlementSources")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_config_service.go" $symbol "service-field:$symbol" "$symbol must be represented at service boundary."
    }
    foreach ($symbol in @("TestCodexPlusConfigDefaultUsesRegistryCatalogs", "TestCodexPlusConfigValidationUsesRegistryPlanRules", "TestCodexPlusConfigValidationUsesRegistryFeatureExposureRules")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_config_service_test.go" $symbol "service-test:$symbol" "$symbol must exist in targeted service integration tests."
    }
    foreach ($symbol in @("gofmt -l", "go test ./internal/codexplus/configregistry ./internal/service -run CodexPlus", "01-backend-config-center Go compile gate passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-01-go.ps1" $symbol "go-verify-script:$symbol" "$symbol must be present in the Go verification script."
    }
    foreach ($symbol in @("display-price-no-fallback", "draft-status-error-message", "downstream-risk", "01 static audit passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-01-static.ps1" $symbol "static-verify-script:$symbol" "$symbol must be present in the static verification script."
    }
    foreach ($symbol in @("02-backend-client-api", "05-admin-operations", "06-commerce-and-enforcement", "backend copy keys")) {
        Test-TextContains "codex-plus-dev-plan/01-backend-config-center/reports/coordinator-integration-static-gate.md" $symbol "downstream-risk:$symbol" "$symbol must be represented in the downstream risk register."
    }
    foreach ($fileAndSymbols in @(
        @{ File = "sub2api-main/backend/internal/codexplus/configregistry/plan_catalog.go"; Symbols = @("PlanCatalog", "ValidatePlanCatalog", "DefaultPlanCatalog") },
        @{ File = "sub2api-main/backend/internal/codexplus/configregistry/model_catalog.go"; Symbols = @("ModelCatalog", "ValidateModelCatalog", "DefaultModelCatalog") },
        @{ File = "sub2api-main/backend/internal/codexplus/configregistry/usage_policy.go"; Symbols = @("UsagePolicyCatalog", "ValidateUsagePolicyCatalog", "DefaultUsagePolicyCatalog") },
        @{ File = "sub2api-main/backend/internal/codexplus/configregistry/feature_flags.go"; Symbols = @("FeatureFlags", "ValidateFeatureFlags", "DefaultFeatureFlags") }
    )) {
        foreach ($symbol in $fileAndSymbols.Symbols) {
            Test-TextContains $fileAndSymbols.File $symbol "registry-symbol:$symbol" "$symbol must exist."
        }
    }

    Test-JsonContracts
    Test-NoResidue @(
        "sub2api-main/backend/internal/codexplus/configregistry/common.go",
        "sub2api-main/backend/internal/codexplus/configregistry/plan_catalog.go",
        "sub2api-main/backend/internal/codexplus/configregistry/model_catalog.go",
        "sub2api-main/backend/internal/codexplus/configregistry/usage_policy.go",
        "sub2api-main/backend/internal/codexplus/configregistry/feature_flags.go",
        "sub2api-main/backend/internal/service/codexplus_config_service.go",
        "sub2api-main/backend/internal/service/codexplus_config_service_test.go",
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
        "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md"
    ) @("(?i)\b(TODO|PLACEHOLDER|not implemented)\b", "panic\(")
}

if ($Stage -eq "02-backend-client-api") {
    Test-RequiredFiles @(
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
        "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md",
        "codex-plus-dev-plan/tools/verify-02-static.ps1",
        "codex-plus-dev-plan/tools/verify-02-go.ps1",
        "codex-plus-dev-plan/02-backend-client-api/README.md",
        "codex-plus-dev-plan/02-backend-client-api/task-client-bootstrap-api.md",
        "codex-plus-dev-plan/02-backend-client-api/task-client-usage-api.md",
        "codex-plus-dev-plan/02-backend-client-api/task-client-device-api.md",
        "codex-plus-dev-plan/02-backend-client-api/task-client-redeem-api.md",
        "codex-plus-dev-plan/02-backend-client-api/reports/coordinator-integration-static-gate.md",
        "codex-plus-contracts/api/client-openapi.yaml",
        "codex-plus-contracts/test-fixtures/client/bootstrap.available.json",
        "codex-plus-contracts/test-fixtures/client/usage.available.json",
        "codex-plus-contracts/test-fixtures/client/devices.registered.json",
        "codex-plus-contracts/test-fixtures/client/redeem.applied.json",
        "sub2api-main/backend/internal/service/codexplus_config_service.go",
        "sub2api-main/backend/internal/codexplus/configregistry/common.go",
        "sub2api-main/backend/internal/service/codexplus_client.go",
        "sub2api-main/backend/internal/service/codexplus_client_test.go",
        "sub2api-main/backend/internal/handler/client/client_handler.go",
        "sub2api-main/backend/internal/handler/dto/codexplus_client.go",
        "sub2api-main/backend/internal/server/routes/client.go"
    )

    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    Add-Check "00-passed-before-02" ($ledger -match "\| ``00-contract`` \|[^\r\n]*\| passed \|") "00-contract must be passed before 02."
    Add-Check "01-passed-before-02" ($ledger -match "\| ``01-backend-config-center`` \|[^\r\n]*\| passed \|") "01-backend-config-center must be passed before 02."
    Add-Check "02-active-or-passed" ($ledger -match "\| ``02-backend-client-api`` \|[^\r\n]*\| (active|passed) \|") "02-backend-client-api must be active during verification or passed after completion."
    foreach ($stageNumber in 3..7) {
        $stagePrefix = "{0:D2}" -f $stageNumber
        $allowed = if ($stageNumber -eq 3) { "(blocked|active)" } else { "blocked" }
        Add-Check "stage-$stagePrefix-sequential" ($ledger -match "\| ``$stagePrefix-[^``]+`` \|[^\r\n]*\| $allowed \|") "$stagePrefix must respect the sequential gate state after 02 verification."
    }

    foreach ($taskFile in @(
        "codex-plus-dev-plan/02-backend-client-api/task-client-bootstrap-api.md",
        "codex-plus-dev-plan/02-backend-client-api/task-client-usage-api.md",
        "codex-plus-dev-plan/02-backend-client-api/task-client-device-api.md",
        "codex-plus-dev-plan/02-backend-client-api/task-client-redeem-api.md"
    )) {
        Test-TextContains $taskFile "## 解耦要求" "02-task-decoupling:$taskFile" "$taskFile must state decoupling requirements."
        Test-TextContains $taskFile "## 禁止改动范围" "02-task-forbidden-scope:$taskFile" "$taskFile must keep worker scopes bounded."
        Test-TextContains $taskFile "## 测试要求" "02-task-tests:$taskFile" "$taskFile must define verification requirements."
    }

    Test-TextContains "codex-plus-dev-plan/02-backend-client-api/README.md" "配置中心" "02-readme-consumes-01" "02 must explicitly consume 01 config center output."
    Test-TextContains "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md" "02-Backend Client API Dispatch Ready" "02-dispatch-recorded" "Trace must record the 02 parallel dispatch boundary."
    foreach ($symbol in @("Bootstrap", "ClientUsage", "UpsertDevice", "Redeem", "codexPlusClientPlanForSubscription", "codexPlusClientUsagePolicyForPlan", "codexPlusClientModelAllowedByPlan")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_client.go" $symbol "02-service-symbol:$symbol" "$symbol must exist in the client API service."
    }
    foreach ($symbol in @("firstPlan", "firstUsagePolicy")) {
        $text = Read-Text "sub2api-main/backend/internal/service/codexplus_client.go"
        Add-Check "02-no-$symbol" ($text -notmatch "\b$symbol\s*\(") "$symbol must not be used by 02."
    }
    foreach ($testName in @("TestCodexPlusClientBootstrapIncludesContractFieldsAndEventContext", "TestCodexPlusClientUsageReturnsContractShapeAndEvent", "TestCodexPlusClientBootstrapSelectsPlanPolicyAndModelsFromEntitlement", "TestCodexPlusClientUpsertDeviceIsIdempotentAndEmitsEvent", "TestCodexPlusClientRedeemMapsStatusesAndEmitsEvent")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_client_test.go" $testName "02-service-test:$testName" "$testName must exist."
    }
    foreach ($scriptSymbol in @("02 static audit passed", "no-first-plan-fallback", "event-store-config-version")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-02-static.ps1" $scriptSymbol "02-static-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($scriptSymbol in @("gofmt -l", "go test ./internal/service ./internal/handler/client ./internal/handler/dto ./internal/server/routes -run", "02-backend-client-api Go compile gate passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-02-go.ps1" $scriptSymbol "02-go-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($reportSymbol in @("Report status: final", "Worker lane: Coordinator", "verify-02-static.ps1", "verify-02-go.ps1", "firstPlan", "03-client-cloud-core")) {
        Test-TextContains "codex-plus-dev-plan/02-backend-client-api/reports/coordinator-integration-static-gate.md" $reportSymbol "02-report:$reportSymbol" "$reportSymbol must be recorded."
    }
    Test-JsonContracts
}

if ($Stage -eq "03-client-cloud-core") {
    Test-RequiredFiles @(
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
        "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md",
        "codex-plus-dev-plan/tools/verify-03-static.ps1",
        "codex-plus-dev-plan/tools/verify-03-node.ps1",
        "codex-plus-dev-plan/tools/verify-03-rust.ps1",
        "codex-plus-dev-plan/03-client-cloud-core/README.md",
        "codex-plus-dev-plan/03-client-cloud-core/task-bootstrap-consumer.md",
        "codex-plus-dev-plan/03-client-cloud-core/task-managed-provider-writer.md",
        "codex-plus-dev-plan/03-client-cloud-core/task-local-session-store.md",
        "codex-plus-dev-plan/03-client-cloud-core/task-error-and-log-redaction.md",
        "codex-plus-dev-plan/03-client-cloud-core/reports/coordinator-static-verification.md",
        "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/api.rs",
        "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/bootstrap.rs",
        "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/local_state.rs",
        "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/provider_writer.rs",
        "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/redaction.rs",
        "CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/src/cloud_commands.rs",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/cloudCommands.ts",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/types.ts"
    )

    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    Add-Check "00-passed-before-03" ($ledger -match "\| ``00-contract`` \|[^\r\n]*\| passed \|") "00-contract must be passed before 03."
    Add-Check "01-passed-before-03" ($ledger -match "\| ``01-backend-config-center`` \|[^\r\n]*\| passed \|") "01-backend-config-center must be passed before 03."
    Add-Check "02-passed-before-03" ($ledger -match "\| ``02-backend-client-api`` \|[^\r\n]*\| passed \|") "02-backend-client-api must be passed before 03."
    Add-Check "03-active-or-passed" ($ledger -match "\| ``03-client-cloud-core`` \|[^\r\n]*\| (active|passed) \|") "03-client-cloud-core must be active during verification or passed after completion."
    Add-Check "stage-04-sequential" ($ledger -match "\| ``04-client-user-experience`` \|[^\r\n]*\| (blocked|active) \|") "04 must remain blocked during 03 or become active after 03 passes."
    foreach ($stageNumber in 5..7) {
        $stagePrefix = "{0:D2}" -f $stageNumber
        Add-Check "stage-$stagePrefix-blocked" ($ledger -match "\| ``$stagePrefix-[^``]+`` \|[^\r\n]*\| blocked \|") "$stagePrefix must remain blocked during 03."
    }

    foreach ($taskFile in @(
        "codex-plus-dev-plan/03-client-cloud-core/task-bootstrap-consumer.md",
        "codex-plus-dev-plan/03-client-cloud-core/task-managed-provider-writer.md",
        "codex-plus-dev-plan/03-client-cloud-core/task-local-session-store.md",
        "codex-plus-dev-plan/03-client-cloud-core/task-error-and-log-redaction.md"
    )) {
        Test-TextContains $taskFile "## 解耦要求" "03-task-decoupling:$taskFile" "$taskFile must state decoupling requirements."
        Test-TextContains $taskFile "## 禁止改动范围" "03-task-forbidden-scope:$taskFile" "$taskFile must keep worker scopes bounded."
        Test-TextContains $taskFile "## 测试要求" "03-task-tests:$taskFile" "$taskFile must define verification requirements."
    }

    foreach ($symbol in @("message_key", "commerce_action", "action_copy_key", "strict_device_enforcement", "force_update_prompt", "X-CodexPlus-Device-Id")) {
        Test-TextContains "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/api.rs" $symbol "03-api-symbol:$symbol" "$symbol must be consumed by the desktop API layer."
    }
    foreach ($symbol in @("CloudLocalStore", "state_with_usage", "state_projects_backend_action_fields_without_provider_key", "json_string_from_keys")) {
        Test-TextContains "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/local_state.rs" $symbol "03-state-symbol:$symbol" "$symbol must exist in local state handling."
    }
    foreach ($symbol in @("OPENAI_API_KEY", "upstream_base_url", "local_responses_proxy_base_url", "managed_profile_routes_codex_through_local_proxy")) {
        Test-TextContains "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/provider_writer.rs" $symbol "03-provider-symbol:$symbol" "$symbol must exist in managed provider writing."
    }
    foreach ($symbol in @("polltoken", "sessiontoken", "redacts_token_patterns_and_url_query_values")) {
        Test-TextContains "CodexPlusPlus-main/crates/codex-plus-core/src/codexplus_cloud/redaction.rs" $symbol "03-redaction-symbol:$symbol" "$symbol must be covered by redaction utilities."
    }
    foreach ($scriptSymbol in @("03 static audit passed", "strict_device_enforcement", "managed_profile_routes_codex_through_local_proxy")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-03-static.ps1" $scriptSymbol "03-static-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($scriptSymbol in @("npm run check", "03-client-cloud-core Node/TypeScript gate passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-03-node.ps1" $scriptSymbol "03-node-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($scriptSymbol in @("cargo fmt --check", "cargo test -p codex-plus-core codexplus_cloud", '"relay_config"', '"protocol_proxy"', "03-client-cloud-core Rust gate passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-03-rust.ps1" $scriptSymbol "03-rust-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($reportSymbol in @("Report status: final", "verify-03-static.ps1", "verify-03-node.ps1", "verify-03-rust.ps1", "cargo", "04-client-user-experience")) {
        Test-TextContains "codex-plus-dev-plan/03-client-cloud-core/reports/coordinator-static-verification.md" $reportSymbol "03-report:$reportSymbol" "$reportSymbol must be recorded."
    }
}

if ($Stage -eq "04-client-user-experience") {
    Test-RequiredFiles @(
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
        "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md",
        "codex-plus-dev-plan/tools/verify-04-static.ps1",
        "codex-plus-dev-plan/tools/verify-04-node.ps1",
        "codex-plus-dev-plan/04-client-user-experience/README.md",
        "codex-plus-dev-plan/04-client-user-experience/task-home-dashboard-ui.md",
        "codex-plus-dev-plan/04-client-user-experience/task-login-binding-ui.md",
        "codex-plus-dev-plan/04-client-user-experience/task-install-assistant-ui.md",
        "codex-plus-dev-plan/04-client-user-experience/task-new-user-tutorial-ui.md",
        "codex-plus-dev-plan/04-client-user-experience/reports/coordinator-ui-verification.md",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/App.tsx",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/styles.css",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/cloud.css",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudHomeScreen.tsx",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudStatusPanel.tsx",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudLoginPanel.tsx",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudUsagePanel.tsx",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudInstallAssistant.tsx",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudTutorialPanel.tsx",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/cloudCommands.ts",
        "CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/types.ts"
    )

    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    Add-Check "00-passed-before-04" ($ledger -match "\| ``00-contract`` \|[^\r\n]*\| passed \|") "00-contract must be passed before 04."
    Add-Check "01-passed-before-04" ($ledger -match "\| ``01-backend-config-center`` \|[^\r\n]*\| passed \|") "01-backend-config-center must be passed before 04."
    Add-Check "02-passed-before-04" ($ledger -match "\| ``02-backend-client-api`` \|[^\r\n]*\| passed \|") "02-backend-client-api must be passed before 04."
    Add-Check "03-passed-before-04" ($ledger -match "\| ``03-client-cloud-core`` \|[^\r\n]*\| passed \|") "03-client-cloud-core must be passed before 04."
    Add-Check "04-active-or-passed" ($ledger -match "\| ``04-client-user-experience`` \|[^\r\n]*\| (active|passed) \|") "04-client-user-experience must be active during verification or passed after completion."
    foreach ($stageNumber in 5..7) {
        $stagePrefix = "{0:D2}" -f $stageNumber
        $allowed = if ($stageNumber -eq 5) { "(blocked|active)" } else { "blocked" }
        Add-Check "stage-$stagePrefix-sequential" ($ledger -match "\| ``$stagePrefix-[^``]+`` \|[^\r\n]*\| $allowed \|") "$stagePrefix must respect the sequential gate state after 04 verification."
    }

    foreach ($taskFile in @(
        "codex-plus-dev-plan/04-client-user-experience/task-home-dashboard-ui.md",
        "codex-plus-dev-plan/04-client-user-experience/task-login-binding-ui.md",
        "codex-plus-dev-plan/04-client-user-experience/task-install-assistant-ui.md",
        "codex-plus-dev-plan/04-client-user-experience/task-new-user-tutorial-ui.md"
    )) {
        Test-TextContains $taskFile "## 解耦要求" "04-task-decoupling:$taskFile" "$taskFile must state decoupling requirements."
        Test-TextContains $taskFile "## 禁止改动范围" "04-task-forbidden-scope:$taskFile" "$taskFile must keep worker scopes bounded."
        Test-TextContains $taskFile "## 测试要求" "04-task-tests:$taskFile" "$taskFile must define verification requirements."
    }

    foreach ($symbol in @("Status:\s*(active|passed)", "03-client-cloud-core", "05-admin-operations")) {
        Test-TextContains "codex-plus-dev-plan/04-client-user-experience/README.md" $symbol "04-readme:$symbol" "$symbol must be recorded in the 04 README."
    }
    foreach ($scriptSymbol in @("04 static audit passed", "CloudLoginPanel", "CloudTutorialPanel", "no-secret-example")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-04-static.ps1" $scriptSymbol "04-static-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($scriptSymbol in @("npm run check", "04-client-user-experience Node/TypeScript gate passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-04-node.ps1" $scriptSymbol "04-node-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($reportSymbol in @("Report status:", "verify-04-static.ps1", "verify-04-node.ps1", "05-admin-operations")) {
        Test-TextContains "codex-plus-dev-plan/04-client-user-experience/reports/coordinator-ui-verification.md" $reportSymbol "04-report:$reportSymbol" "$reportSymbol must be recorded."
    }
}

if ($Stage -eq "05-admin-operations") {
    Test-RequiredFiles @(
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
        "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md",
        "codex-plus-dev-plan/tools/verify-05-static.ps1",
        "codex-plus-dev-plan/tools/verify-05-node.ps1",
        "codex-plus-dev-plan/tools/verify-05-go.ps1",
        "codex-plus-dev-plan/05-admin-operations/README.md",
        "codex-plus-dev-plan/05-admin-operations/task-admin-plan-management.md",
        "codex-plus-dev-plan/05-admin-operations/task-admin-model-management.md",
        "codex-plus-dev-plan/05-admin-operations/task-admin-usage-policy-management.md",
        "codex-plus-dev-plan/05-admin-operations/task-admin-user-entitlement-view.md",
        "codex-plus-dev-plan/05-admin-operations/reports/coordinator-admin-ops-verification.md",
        "codex-plus-dev-plan/05-admin-operations/reports/worker-plan-management-final.md",
        "codex-plus-dev-plan/05-admin-operations/reports/worker-model-management-final.md",
        "codex-plus-dev-plan/05-admin-operations/reports/worker-usage-policy-final.md",
        "codex-plus-dev-plan/05-admin-operations/reports/worker-user-entitlement-final.md",
        "sub2api-main/backend/internal/service/codexplus_admin_service.go",
        "sub2api-main/backend/internal/handler/admin/codexplus_handler.go",
        "sub2api-main/backend/internal/server/routes/codexplus_admin.go",
        "sub2api-main/frontend/src/api/admin/codexPlus.ts",
        "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusView.vue",
        "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusPlanCatalogPanel.vue",
        "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusModelCatalogPanel.vue",
        "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusUsagePolicyPanel.vue",
        "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusFeatureFlagsPanel.vue",
        "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusUserEntitlementPanel.vue"
    )

    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    foreach ($stage in @("00-contract", "01-backend-config-center", "02-backend-client-api", "03-client-cloud-core", "04-client-user-experience")) {
        Add-Check "previous-stage-passed:$stage" ($ledger -match "\| ``$stage`` \|[^\r\n]*\| passed \|") "$stage must be passed before 05."
    }
    Add-Check "05-active-or-passed" ($ledger -match "\| ``05-admin-operations`` \|[^\r\n]*\| (active|passed) \|") "05-admin-operations must be active during verification or passed after completion."
    Add-Check "stage-06-sequential" ($ledger -match "\| ``06-commerce-and-enforcement`` \|[^\r\n]*\| (blocked|active) \|") "06 must be blocked during 05 or active after 05 passes."
    Add-Check "stage-07-blocked" ($ledger -match "\| ``07-integration-release`` \|[^\r\n]*\| blocked \|") "07 must remain blocked until 06 passes."

    foreach ($taskFile in @(
        "codex-plus-dev-plan/05-admin-operations/task-admin-plan-management.md",
        "codex-plus-dev-plan/05-admin-operations/task-admin-model-management.md",
        "codex-plus-dev-plan/05-admin-operations/task-admin-usage-policy-management.md",
        "codex-plus-dev-plan/05-admin-operations/task-admin-user-entitlement-view.md"
    )) {
        Test-TextContains $taskFile "## 解耦要求" "05-task-decoupling:$taskFile" "$taskFile must state decoupling requirements."
        Test-TextContains $taskFile "## 禁止改动范围" "05-task-forbidden-scope:$taskFile" "$taskFile must keep worker scopes bounded."
        Test-TextContains $taskFile "## 测试要求" "05-task-tests:$taskFile" "$taskFile must define verification requirements."
    }

    foreach ($symbol in @("Status:\s*(active|passed)", "04-client-user-experience", "06-commerce-and-enforcement")) {
        Test-TextContains "codex-plus-dev-plan/05-admin-operations/README.md" $symbol "05-readme:$symbol" "$symbol must be recorded in the 05 README."
    }
    foreach ($route in @("/config", "/config/validate", "/config/publish", "/config/versions", "/config/rollback", "/options", "/users/:id/entitlement")) {
        Test-TextContains "sub2api-main/backend/internal/server/routes/codexplus_admin.go" ([regex]::Escape($route)) "05-admin-route:$route" "$route must be routed."
    }
    foreach ($symbol in @("GetConfig", "ValidateConfig", "PublishConfig", "ListConfigVersions", "RollbackConfig", "GetOptions", "GetUserEntitlement")) {
        Test-TextContains "sub2api-main/backend/internal/handler/admin/codexplus_handler.go" $symbol "05-handler-symbol:$symbol" "$symbol must exist in admin handler."
    }
    foreach ($symbol in @("GetCurrentConfig", "ValidateConfig", "PublishConfig", "ListConfigVersions", "RollbackConfig", "GetOptions", "GetUserEntitlement")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_admin_service.go" $symbol "05-service-symbol:$symbol" "$symbol must exist in admin service."
    }
    foreach ($symbol in @("CodexPlusPlanCatalog", "CodexPlusModelCatalog", "CodexPlusUsagePolicy", "CodexPlusFeatureFlagsDoc", "CodexPlusUserEntitlement", "publishCodexPlusConfig", "getCodexPlusUserEntitlement")) {
        Test-TextContains "sub2api-main/frontend/src/api/admin/codexPlus.ts" $symbol "05-frontend-api:$symbol" "$symbol must exist in frontend admin API."
    }
    foreach ($symbol in @("price_amount_minor", "entitlement_sources", "subscription_group_ids", "api_key_group_ids", "copy_keys", "usage_policy_id", "rollout_channel", "fallback_model_id", "disabled_replacement_model_id", "monthly_quota", "device_policy", "strict_device_enforcement", "server_only")) {
        Test-TextContains "sub2api-main/frontend/src/api/admin/codexPlus.ts" $symbol "05-shared-api-field:$symbol" "$symbol must exist in shared frontend admin API types."
    }
    foreach ($panel in @("CodexPlusPlanCatalogPanel", "CodexPlusModelCatalogPanel", "CodexPlusUsagePolicyPanel", "CodexPlusUserEntitlementPanel")) {
        Test-TextContains "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusView.vue" $panel "05-view-panel:$panel" "$panel must be wired into the Codex++ admin view."
    }
    foreach ($worker in @(
        @{ Lane = "Plan"; File = "codex-plus-dev-plan/05-admin-operations/reports/worker-plan-management-final.md" },
        @{ Lane = "Model"; File = "codex-plus-dev-plan/05-admin-operations/reports/worker-model-management-final.md" },
        @{ Lane = "Usage"; File = "codex-plus-dev-plan/05-admin-operations/reports/worker-usage-policy-final.md" },
        @{ Lane = "Entitlement"; File = "codex-plus-dev-plan/05-admin-operations/reports/worker-user-entitlement-final.md" }
    )) {
        Test-TextContains $worker.File "Report status:\s*final" "05-worker-report-final:$($worker.Lane)" "$($worker.Lane) worker report must be final."
        Test-TextContains $worker.File "Worker lane:\s*$($worker.Lane)" "05-worker-report-lane:$($worker.Lane)" "$($worker.Lane) worker report lane must match."
        Test-TextContains $worker.File "## Verification" "05-worker-report-verification:$($worker.Lane)" "$($worker.Lane) worker report must include verification."
    }
    foreach ($symbol in @("price_amount_minor", "entitlement_sources", "usage_policy_id", "copy_keys")) {
        Test-TextContains "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusPlanCatalogPanel.vue" $symbol "05-plan-panel-field:$symbol" "$symbol must be editable or visible in plan management."
    }
    foreach ($symbol in @("rollout_channel", "quality_tier", "fallback_model_id", "disabled_replacement_model_id", "operator_tags")) {
        Test-TextContains "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusModelCatalogPanel.vue" $symbol "05-model-panel-field:$symbol" "$symbol must be editable or visible in model management."
    }
    foreach ($symbol in @("monthly_quota", "device_policy", "copy_keys", "Policy preview")) {
        Test-TextContains "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusUsagePolicyPanel.vue" $symbol "05-usage-panel-field:$symbol" "$symbol must be editable or visible in usage policy management."
    }
    foreach ($symbol in @("strict_device_enforcement", "server-only")) {
        Test-TextContains "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusFeatureFlagsPanel.vue" $symbol "05-flags-panel-field:$symbol" "$symbol must preserve server-only feature flag semantics."
    }
    foreach ($symbol in @("safeMaskedKey", "integration_status", "managed_provider_key", "recent_events", "usage_summary")) {
        Test-TextContains "sub2api-main/frontend/src/views/admin/codexPlus/CodexPlusUserEntitlementPanel.vue" $symbol "05-entitlement-panel-field:$symbol" "$symbol must be visible in user entitlement support view."
    }
    foreach ($scriptSymbol in @("05 static audit passed", "CodexPlusPlanCatalogPanel", "GetUserEntitlement", "verify-05-node.ps1", "verify-05-go.ps1")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-05-static.ps1" $scriptSymbol "05-static-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($scriptSymbol in @("npm run typecheck", "npm run build", "05-admin-operations Node/TypeScript gate passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-05-node.ps1" $scriptSymbol "05-node-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($scriptSymbol in @("gofmt -l", "go test ./internal/service ./internal/handler/admin ./internal/server/routes -run", "05-admin-operations Go gate passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-05-go.ps1" $scriptSymbol "05-go-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($reportSymbol in @("Report status:\s*final", "verify-05-static.ps1", "verify-05-node.ps1", "verify-05-go.ps1", "06-commerce-and-enforcement")) {
        Test-TextContains "codex-plus-dev-plan/05-admin-operations/reports/coordinator-admin-ops-verification.md" $reportSymbol "05-report:$reportSymbol" "$reportSymbol must be recorded."
    }
}

if ($Stage -eq "06-commerce-and-enforcement") {
    Test-RequiredFiles @(
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
        "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md",
        "codex-plus-dev-plan/tools/verify-06-static.ps1",
        "codex-plus-dev-plan/tools/verify-06-go.ps1",
        "codex-plus-dev-plan/06-commerce-and-enforcement/README.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/task-payment-entitlement-flow.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/task-gateway-policy-enforcement.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/task-device-management.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/task-audit-and-risk-control.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/reports/README.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/reports/coordinator-commerce-enforcement-verification.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-payment-entitlement-final.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-gateway-enforcement-final.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-device-management-final.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-audit-risk-final.md",
        "sub2api-main/backend/internal/service/codexplus_commerce_entitlement.go",
        "sub2api-main/backend/internal/service/codexplus_commerce_entitlement_test.go",
        "sub2api-main/backend/internal/service/codexplus_device_management.go",
        "sub2api-main/backend/internal/service/codexplus_device_management_test.go",
        "sub2api-main/backend/internal/service/codexplus_audit_risk.go",
        "sub2api-main/backend/internal/service/codexplus_audit_risk_test.go",
        "sub2api-main/backend/internal/service/codexplus_gateway_policy_service.go",
        "sub2api-main/backend/internal/service/codexplus_gateway_policy_service_test.go",
        "sub2api-main/backend/internal/handler/codexplus_gateway_policy.go",
        "sub2api-main/backend/internal/handler/codexplus_gateway_policy_test.go",
        "sub2api-main/backend/internal/service/codexplus_foundation.go",
        "sub2api-main/backend/internal/repository/codexplus_foundation_repo.go",
        "sub2api-main/backend/internal/repository/codexplus_foundation_repo_test.go",
        "sub2api-main/backend/internal/service/codexplus_config_service.go",
        "sub2api-main/backend/internal/service/codexplus_client.go"
    )

    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    foreach ($stage in @("00-contract", "01-backend-config-center", "02-backend-client-api", "03-client-cloud-core", "04-client-user-experience", "05-admin-operations")) {
        Add-Check "previous-stage-passed:$stage" ($ledger -match "\| ``$stage`` \|[^\r\n]*\| passed \|") "$stage must be passed before 06."
    }
    Add-Check "06-active-or-passed" ($ledger -match "\| ``06-commerce-and-enforcement`` \|[^\r\n]*\| (active|passed) \|") "06-commerce-and-enforcement must be active during verification or passed after exit."
    Add-Check "stage-07-sequential" ($ledger -match "\| ``07-integration-release`` \|[^\r\n]*\| (blocked|active) \|") "07 must be blocked during 06 or active after 06 passes."

    foreach ($taskFile in @(
        "codex-plus-dev-plan/06-commerce-and-enforcement/task-payment-entitlement-flow.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/task-gateway-policy-enforcement.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/task-device-management.md",
        "codex-plus-dev-plan/06-commerce-and-enforcement/task-audit-and-risk-control.md"
    )) {
        Test-TextContains $taskFile "## 解耦要求" "06-task-decoupling:$taskFile" "$taskFile must state decoupling requirements."
        Test-TextContains $taskFile "## 禁止改动范围" "06-task-forbidden-scope:$taskFile" "$taskFile must keep worker scopes bounded."
        Test-TextContains $taskFile "## 测试要求" "06-task-tests:$taskFile" "$taskFile must define verification requirements."
    }

    foreach ($symbol in @("Status:\s*(active|passed)", "05-admin-operations", "07-integration-release")) {
        Test-TextContains "codex-plus-dev-plan/06-commerce-and-enforcement/README.md" $symbol "06-readme:$symbol" "$symbol must be recorded in the 06 README."
    }
    foreach ($worker in @(
        @{ Lane = "Payment"; File = "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-payment-entitlement-final.md" },
        @{ Lane = "Gateway"; File = "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-gateway-enforcement-final.md" },
        @{ Lane = "Device"; File = "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-device-management-final.md" },
        @{ Lane = "Audit"; File = "codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-audit-risk-final.md" }
    )) {
        Test-TextContains $worker.File "Report status:\s*final" "06-worker-report-final:$($worker.Lane)" "$($worker.Lane) worker report must be final."
        Test-TextContains $worker.File "Worker lane:\s*$($worker.Lane)" "06-worker-report-lane:$($worker.Lane)" "$($worker.Lane) worker report lane must match."
        Test-TextContains $worker.File "## Verification" "06-worker-report-verification:$($worker.Lane)" "$($worker.Lane) worker report must include verification."
    }
    foreach ($symbol in @("CodexPlusCommerceEntitlementService", "ResolveSubscriptionOrder", "AlreadyGranted", "RecordGrant")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_commerce_entitlement.go" $symbol "06-payment-symbol:$symbol" "$symbol must be available for payment entitlement fulfillment."
    }
    foreach ($symbol in @("CodexPlusGatewayPolicyService", "Evaluate", "StrictDeviceEnforcement", "CodexPlusGatewayPolicyDecision", "CodexPlusGatewayPolicyInput", "CodexPlusEventCreate")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_gateway_policy_service.go" $symbol "06-gateway-policy-symbol:$symbol" "$symbol must be available for 06 enforcement work."
    }
    foreach ($symbol in @("resolveCodexPlusGatewayPolicy", "checkUsagePolicy", "BuildCodexPlusGatewayPolicyUsagePayload", "NormalizeCodexPlusAuditRiskPayload")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_gateway_policy_service.go" $symbol "06-gateway-enforcement-symbol:$symbol" "$symbol must be available for gateway rejection and usage/audit integration."
    }
    foreach ($symbol in @("codexPlusGatewayPolicy", "X-CodexPlus-Device-Id", "CodexPlusGatewayPolicyService")) {
        Test-TextContains "sub2api-main/backend/internal/handler/codexplus_gateway_policy.go" $symbol "06-gateway-handler-symbol:$symbol" "$symbol must be available in the gateway hook layer."
    }
    foreach ($symbol in @("CodexPlusDevice", "CodexPlusEvent", "CodexPlusDeviceRepository", "CodexPlusEventRepository")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_foundation.go" $symbol "06-foundation-symbol:$symbol" "$symbol must be available for devices and audit events."
    }
    foreach ($symbol in @("Upsert", "GetByUserAndDevice", "ListByUser", "Append")) {
        Test-TextContains "sub2api-main/backend/internal/repository/codexplus_foundation_repo.go" $symbol "06-repository-symbol:$symbol" "$symbol must be available for device/audit persistence."
    }
    foreach ($symbol in @("CodexPlusAdminService", "RevokeUserDevice", "RestoreUserDevice", "ListUserDevices")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_device_management.go" $symbol "06-device-management-symbol:$symbol" "$symbol must be available for admin device enforcement operations."
    }
    foreach ($symbol in @("CodexPlusAuditRiskEventCreateInput", "RecordCodexPlusAuditRiskEvent", "QueryCodexPlusAuditRiskEvents", "RedactCodexPlusAuditRiskMetadata")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_audit_risk.go" $symbol "06-audit-risk-symbol:$symbol" "$symbol must be available for audit and risk event handling."
    }
    foreach ($symbol in @("entitlement_sources", "usage_policy_id", "strict_device_enforcement", "device_policy")) {
        Test-TextContains "sub2api-main/backend/internal/service/codexplus_config_service.go" $symbol "06-config-symbol:$symbol" "$symbol must remain backend-driven for commerce/enforcement."
    }
    foreach ($scriptSymbol in @("06 static audit passed", "CodexPlusGatewayPolicyService", "CodexPlusDeviceRepository", "verify-06-go.ps1")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-06-static.ps1" $scriptSymbol "06-static-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($scriptSymbol in @("gofmt -l", "go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository -run", "06-commerce-and-enforcement Go gate passed")) {
        Test-TextContains "codex-plus-dev-plan/tools/verify-06-go.ps1" $scriptSymbol "06-go-script:$scriptSymbol" "$scriptSymbol must be present."
    }
    foreach ($reportSymbol in @("Report status:\s*final", "verify-06-static.ps1", "verify-06-go.ps1", "Payment entitlement flow", "Gateway policy enforcement", "Device management", "Audit and risk control")) {
        Test-TextContains "codex-plus-dev-plan/06-commerce-and-enforcement/reports/coordinator-commerce-enforcement-verification.md" $reportSymbol "06-report:$reportSymbol" "$reportSymbol must be recorded."
    }
}

if ($Stage -eq "07-integration-release") {
    Test-RequiredFiles @(
        "codex-plus-product-spec.html",
        "codex-plus-dev-plan/STAGE-GATE-LEDGER.md",
        "codex-plus-dev-plan/IMPLEMENTATION-STATUS.md",
        "codex-plus-dev-plan/MULTI-SESSION-EXECUTION-TRACE.md",
        "codex-plus-dev-plan/INTEGRATION-VERIFICATION-CHECKLIST.md",
        "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md",
        "codex-plus-dev-plan/PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md",
        "codex-plus-dev-plan/PARALLEL-DISPATCH-PLAN.md",
        "sub2api-main/tools/e2e/codexplus/README.md",
        "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1",
        "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1",
        "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1",
        "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1",
        "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1",
        "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1",
        "sub2api-main/deploy/docker-compose.dev.yml",
        "sub2api-main/deploy/.env.codexplus-local.example",
        "sub2api-main/deploy/.gitignore",
        "codex-plus-dev-plan/tools/verify-07-static.ps1",
        "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1",
        "codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1",
        "codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1",
        "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1",
        "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1",
        "codex-plus-dev-plan/tools/new-07-evidence-run.ps1",
        "codex-plus-dev-plan/tools/verify-07-evidence.ps1",
        "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1",
        "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1",
        "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1",
        "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1",
        "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1",
        "codex-plus-dev-plan/tools/new-07-business-readiness-evidence.ps1",
        "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1",
        "codex-plus-dev-plan/tools/new-07-docs-product-copy-evidence.ps1",
        "codex-plus-dev-plan/tools/verify-07-docs-product-copy-evidence.ps1",
        "codex-plus-dev-plan/tools/new-07-package-evidence.ps1",
        "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1",
        "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1",
        "codex-plus-dev-plan/tools/new-07-compatibility-evidence.ps1",
        "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1",
        "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1",
        "codex-plus-dev-plan/tools/report-07-release-gaps.ps1",
        "codex-plus-dev-plan/07-integration-release/README.md",
        "codex-plus-dev-plan/07-integration-release/task-e2e-buy-login-launch.md",
        "codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md",
        "codex-plus-dev-plan/07-integration-release/task-package-install-check.md",
        "codex-plus-dev-plan/07-integration-release/task-docs-and-product-copy.md",
        "codex-plus-dev-plan/07-integration-release/reports/README.md",
        "codex-plus-dev-plan/07-integration-release/reports/coordinator-integration-release-verification.md",
        "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md",
        "codex-plus-dev-plan/07-integration-release/reports/worker-e2e-buy-login-launch-final.md",
        "codex-plus-dev-plan/07-integration-release/reports/worker-compatibility-migration-final.md",
        "codex-plus-dev-plan/07-integration-release/reports/worker-package-install-final.md",
        "codex-plus-dev-plan/07-integration-release/reports/worker-docs-product-copy-final.md",
        "codex-plus-dev-plan/07-integration-release/release-local-verification.md",
        "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md",
        "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md",
        "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md",
        "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md",
        "codex-plus-dev-plan/07-integration-release/compatibility/rollback-notes.md",
        "codex-plus-dev-plan/07-integration-release/package-install/package-install-checklist.md",
        "codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md",
        "codex-plus-dev-plan/07-integration-release/package-install/local-command-evidence.md",
        "codex-plus-dev-plan/07-integration-release/package-install/pre-release-blockers.md",
        "codex-plus-dev-plan/07-integration-release/docs/user-guide.md",
        "codex-plus-dev-plan/07-integration-release/docs/admin-operations-guide.md",
        "codex-plus-dev-plan/07-integration-release/docs/release-notes-draft.md",
        "codex-plus-dev-plan/07-integration-release/docs/docs-sync-record.md",
        "codex-plus-dev-plan/07-integration-release/docs/html-sync-evidence.md",
        "codex-plus-dev-plan/07-integration-release/docs/html-visual-evidence/in-app-browser-policy-boundary.md",
        "codex-plus-dev-plan/07-integration-release/docs/html-visual-evidence/in-app-browser-http-desktop.png",
        "codex-plus-dev-plan/07-integration-release/docs/html-visual-evidence/in-app-browser-http-mobile.png",
        "codex-plus-dev-plan/07-integration-release/docs/html-visual-evidence/product-spec-desktop.png",
        "codex-plus-dev-plan/07-integration-release/docs/html-visual-evidence/product-spec-mobile.png"
    )

    $ledger = Read-Text "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
    foreach ($stage in @("00-contract", "01-backend-config-center", "02-backend-client-api", "03-client-cloud-core", "04-client-user-experience", "05-admin-operations", "06-commerce-and-enforcement")) {
        Add-Check "previous-stage-passed:$stage" ($ledger -match "\| ``$stage`` \|[^\r\n]*\| passed \|") "$stage must be passed before 07."
    }
    Add-Check "07-active" ($ledger -match "\| ``07-integration-release`` \|[^\r\n]*\| active \|") "07-integration-release must be active."

    foreach ($taskFile in @(
        "codex-plus-dev-plan/07-integration-release/task-e2e-buy-login-launch.md",
        "codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md",
        "codex-plus-dev-plan/07-integration-release/task-package-install-check.md",
        "codex-plus-dev-plan/07-integration-release/task-docs-and-product-copy.md"
    )) {
        Test-TextContains $taskFile "## 解耦要求" "07-task-decoupling:$taskFile" "$taskFile must state decoupling requirements."
        Test-TextContains $taskFile "## 禁止改动范围" "07-task-forbidden-scope:$taskFile" "$taskFile must keep worker scopes bounded."
        Test-TextContains $taskFile "## 测试要求" "07-task-tests:$taskFile" "$taskFile must define verification requirements."
        Test-TextContains $taskFile "## 交付物" "07-task-deliverables:$taskFile" "$taskFile must define deliverables."
    }

    foreach ($symbol in @("Status:\s*active", "06-commerce-and-enforcement", "INTEGRATION-VERIFICATION-CHECKLIST", "PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN", "PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/README.md" $symbol "07-readme:$symbol" "$symbol must be recorded in the 07 README."
    }
    foreach ($symbol in @("verify-07-release-evidence\.ps1", "verify-07-business-readiness\.ps1", "verify-07-release-handoff\.ps1", "report-07-release-gaps\.ps1", "ReadinessSummaryFile", "business/legal approval", "Release coverage summary and readiness summary")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/README.md" $symbol "07-readme-release-chain:$symbol" "$symbol must be recorded in the 07 README release chain."
    }
    foreach ($symbol in @("Browser handoff", "Turnstile", "One model request succeeds through Sub2API gateway", "Rollback notes")) {
        Test-TextContains "codex-plus-dev-plan/INTEGRATION-VERIFICATION-CHECKLIST.md" $symbol "07-checklist:$symbol" "$symbol must be present in the final verification checklist."
    }
    foreach ($symbol in @("Target Evidence Structure", "Release Gate Decisions", "evidence folder", "release gate decision")) {
        Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" $symbol "07-module-i:$symbol" "$symbol must be present in the Module I plan."
    }
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "new-07-release-evidence-set\.ps1" "07-module-i:release-evidence-set-generator" "Module I must reference the full release evidence set generator."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "new-07-evidence-run\.ps1" "07-module-i:evidence-generator" "Module I must reference the evidence scaffold generator."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-client-api-checks\.ps1" "07-module-i:client-api-runner" "Module I must reference the client API runner."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-browser-handoff-checks\.ps1" "07-module-i:browser-handoff-runner" "Module I must reference the browser handoff runner."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-gateway-policy-checks\.ps1" "07-module-i:gateway-policy-runner" "Module I must reference the gateway policy runner."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-admin-audit-checks\.ps1" "07-module-i:admin-audit-runner" "Module I must reference the admin audit runner."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "gateway-matched .*request_id" "07-module-i:admin-audit-request-id-correlation" "Module I must document matched gateway/admin request IDs."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-local-e2e\.ps1" "07-module-i:local-e2e-runner" "Module I must reference the local E2E runner."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "verify-07-e2e-readiness\.ps1" "07-module-i:e2e-readiness-verifier" "Module I must reference the E2E readiness verifier."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "new-07-e2e-env-template\.ps1" "07-module-i:e2e-env-template-generator" "Module I must reference the E2E env template generator."
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "verify-07-evidence\.ps1" "07-module-i:evidence-verifier" "Module I must reference the evidence hygiene verifier."
    foreach ($symbol in @("Final Report Structure", "Go/No-Go Policy", "Rollback notes", "Conflict Resolution Rules", "verify-07-release-evidence\.ps1", "summarize-07-release-coverage\.ps1", "summarize-07-release-readiness\.ps1", "verify-07-business-readiness\.ps1", "verify-07-module-j-report\.ps1", "verify-07-release-handoff\.ps1", "new-07-release-evidence-set\.ps1")) {
        Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md" $symbol "07-module-j:$symbol" "$symbol must be present in the Module J plan."
    }
    Test-TextContains "codex-plus-dev-plan/PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md" "ReadinessSummaryFile" "07-module-j:readiness-summary-file" "Module J plan must pass the readiness summary to the report verifier."
    foreach ($reportSymbol in @("Report status:\s*in-progress", "Worker lane:\s*Coordinator", "verify-07-static\.ps1", "E2E buy/login/launch", "Compatibility and migration", "Package install check", "Docs and product copy")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/coordinator-integration-release-verification.md" $reportSymbol "07-report:$reportSymbol" "$reportSymbol must be recorded."
    }
    foreach ($worker in @(
        @{ Lane = "E2E"; File = "codex-plus-dev-plan/07-integration-release/reports/worker-e2e-buy-login-launch-final.md"; Pending = "E2E evidence pending" },
        @{ Lane = "Compatibility"; File = "codex-plus-dev-plan/07-integration-release/reports/worker-compatibility-migration-final.md"; Pending = "compatibility evidence pending" },
        @{ Lane = "Package"; File = "codex-plus-dev-plan/07-integration-release/reports/worker-package-install-final.md"; Pending = "package evidence pending" },
        @{ Lane = "Docs"; File = "codex-plus-dev-plan/07-integration-release/reports/worker-docs-product-copy-final.md"; Pending = "pending" }
    )) {
        Test-TextContains $worker.File "Report status:\s*final" "07-worker-report-final:$($worker.Lane)" "$($worker.Lane) worker report must be final."
        Test-TextContains $worker.File "Worker lane:\s*$($worker.Lane)" "07-worker-report-lane:$($worker.Lane)" "$($worker.Lane) worker report lane must match."
        Test-TextContains $worker.File "Forbidden edits:\s*none" "07-worker-report-forbidden:$($worker.Lane)" "$($worker.Lane) worker report must state forbidden edits: none."
        Test-TextContains $worker.File "## Verification" "07-worker-report-verification:$($worker.Lane)" "$($worker.Lane) worker report must include verification."
        Test-TextContains $worker.File "## Remaining [Rr]isks" "07-worker-report-risks:$($worker.Lane)" "$($worker.Lane) worker report must include remaining risks."
        Test-TextContains $worker.File $worker.Pending "07-worker-report-pending-boundary:$($worker.Lane)" "$($worker.Lane) worker report must not overstate release readiness."
    }
    foreach ($symbol in @("browser handoff", "Sub2API gateway", "E2E evidence pending", "No real API Key")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" $symbol "07-e2e-checklist:$symbol" "$symbol must be present in E2E checklist."
    }
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "verify-07-evidence\.ps1" "07-e2e-checklist:evidence-verifier" "E2E checklist must require evidence verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "new-07-evidence-run\.ps1" "07-e2e-checklist:evidence-generator" "E2E checklist must mention the evidence generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "verify-07-e2e-readiness\.ps1" "07-e2e-checklist:readiness-verifier" "E2E checklist must mention the readiness verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "new-07-e2e-env-template\.ps1" "07-e2e-checklist:env-template-generator" "E2E checklist must mention the env template generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "run-local-e2e\.ps1" "07-e2e-checklist:local-e2e-runner" "E2E checklist must mention the local E2E runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "run-admin-audit-checks\.ps1" "07-e2e-checklist:admin-audit-runner" "E2E checklist must mention the admin audit runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "verify-07-evidence\.ps1" "07-evidence-template:evidence-verifier" "Evidence template must document the verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "new-07-evidence-run\.ps1" "07-evidence-template:evidence-generator" "Evidence template must document the generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "verify-07-e2e-readiness\.ps1" "07-evidence-template:readiness-verifier" "Evidence template must document the readiness verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "new-07-e2e-env-template\.ps1" "07-evidence-template:env-template-generator" "Evidence template must document the env template generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-client-api-checks\.ps1" "07-evidence-template:client-api-runner" "Evidence template must document the client API runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-browser-handoff-checks\.ps1" "07-evidence-template:browser-handoff-runner" "Evidence template must document the browser handoff runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-gateway-policy-checks\.ps1" "07-evidence-template:gateway-policy-runner" "Evidence template must document the gateway policy runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-admin-audit-checks\.ps1" "07-evidence-template:admin-audit-runner" "Evidence template must document the admin audit runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-local-e2e\.ps1" "07-evidence-template:local-e2e-runner" "Evidence template must document the local E2E runner."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-evidence-run\.ps1" "07-release-evidence-set:e2e-generator" "Release evidence set generator must call E2E scaffold generator."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-package-evidence\.ps1" "07-release-evidence-set:package-generator" "Release evidence set generator must call package scaffold generator."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-compatibility-evidence\.ps1" "07-release-evidence-set:compatibility-generator" "Release evidence set generator must call compatibility scaffold generator."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-business-readiness-evidence\.ps1" "07-release-evidence-set:business-generator" "Release evidence set generator must call business readiness scaffold generator."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-docs-product-copy-evidence\.ps1" "07-release-evidence-set:docs-generator" "Release evidence set generator must call docs product copy scaffold generator."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "verify-07-docs-product-copy-evidence\.ps1" "07-release-evidence-set:docs-verifier" "Release evidence set generator must include docs product copy verification."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "-DocsEvidenceDir" "07-release-evidence-set:docs-evidence-dir" "Release evidence set generator must wire docs evidence into release verification."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "module-j-final-report-template\.md" "07-release-evidence-set:module-j-template" "Release evidence set generator must include Module J report draft."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "summarize-07-release-coverage\.ps1" "07-release-evidence-set:coverage-summary" "Release evidence set generator must mention the coverage summary runner."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "summarize-07-release-readiness\.ps1" "07-release-evidence-set:readiness-summary" "Release evidence set generator must mention the readiness summary runner."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "CoverageSummaryFile" "07-release-evidence-set:coverage-summary-file" "Release evidence set generator must wire coverage summary into Module J report verification."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "ReadinessSummaryFile" "07-release-evidence-set:readiness-summary-file" "Release evidence set generator must wire readiness summary into Module J report verification."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "verify-07-release-handoff\.ps1" "07-release-evidence-set:handoff-verifier" "Release evidence set generator must include the final handoff verifier."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "Final Verification Results" "07-release-evidence-set:final-results" "Release evidence set generator must scaffold final verification result fields."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "aggregate verifier result:\s*pending" "07-release-evidence-set:aggregate-result-placeholder" "Release evidence set generator must create a pending aggregate result placeholder."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "docs product copy verification:\s*pending" "07-release-evidence-set:docs-result-placeholder" "Release evidence set generator must create a pending docs product copy result placeholder."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1" "rust-toolchain-available" "07-rust-preflight:toolchain" "Rust preflight must check toolchain availability."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1" "rust-linker-available" "07-rust-preflight:linker" "Rust preflight must check linker availability."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1" "disk:minimum-free-gb" "07-rust-preflight:disk" "Rust preflight must check disk capacity."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "CODEXPLUS_07_E2E_BACKEND_BASE_URL" "07-e2e-readiness:backend-url-env" "E2E readiness must define the backend URL env var."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "AllowProduction" "07-e2e-readiness:production-guard" "E2E readiness must require an explicit production override."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "ProbeHttp" "07-e2e-readiness:optional-http-probe" "E2E readiness must keep HTTP probing opt-in."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "Add-EndpointPreflightCheck" "07-e2e-readiness:preflight-allowlist-helper" "E2E readiness must route endpoint preflight through an explicit status allowlist helper."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "Expected one of" "07-e2e-readiness:preflight-status-allowlist" "E2E readiness endpoint preflight must reject 404, 5xx and connection failures instead of accepting any non-404 status."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "EndpointPreflightOnly" "07-e2e-readiness:preflight-only-mode" "E2E readiness must support a non-release route diagnostic mode that does not require token/model placeholders."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "OutputPath" "07-e2e-readiness:output-report" "E2E readiness must write a sanitized diagnostic report when requested."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "value intentionally not printed" "07-e2e-readiness:redaction" "E2E readiness must not print token values."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1" "e2e-env.template.ps1" "07-e2e-env-template:env-file" "E2E env template generator must write a PowerShell env file."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1" "Required Manual Inputs" "07-e2e-env-template:manual-inputs" "E2E env template generator must list manual inputs."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1" "Do not commit real token values" "07-e2e-env-template:redaction-boundary" "E2E env template generator must warn against committing token values."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "verify-07-e2e-readiness\.ps1" "07-client-api-runner:readiness" "Client API runner must invoke the readiness verifier."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "FixtureMode" "07-client-api-runner:fixture-mode" "Client API runner must support fixture-mode self-test."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "AllowRedeem" "07-client-api-runner:redeem-opt-in" "Client API runner must keep redeem opt-in."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "X-CodexPlus-Device-Id" "07-client-api-runner:device-header" "Client API runner must pass the device header."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "Token value intentionally not printed" "07-client-api-runner:redaction" "Client API runner must not print token values."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "AllowSessionStart" "07-browser-handoff-runner:start-opt-in" "Browser handoff runner must keep session creation opt-in."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "AllowBrowserComplete" "07-browser-handoff-runner:complete-opt-in" "Browser handoff runner must keep browser completion opt-in."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "BROWSER_AUTH_TOKEN" "07-browser-handoff-runner:browser-token-env" "Browser handoff runner must use a browser token env input."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "poll_token not in authorize_url" "07-browser-handoff-runner:poll-token-url-boundary" "Browser handoff runner must check poll token URL boundary."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "Session token, poll token, browser JWT" "07-browser-handoff-runner:redaction" "Browser handoff runner must document redaction."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "FixtureMode" "07-browser-handoff-runner:fixture-mode" "Browser handoff runner must support fixture-mode self-test."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "AllowGatewayRequests" "07-gateway-policy-runner:request-opt-in" "Gateway policy runner must keep real gateway requests opt-in."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "USER_ACTIVE_GATEWAY_KEY" "07-gateway-policy-runner:active-key-env" "Gateway policy runner must use user-side gateway keys from env."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "USER_MODEL_DENIED_GATEWAY_KEY" "07-gateway-policy-runner:model-denied-key-env" "Gateway policy runner must include model-denied persona key input."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "GATEWAY_POLICY_" "07-gateway-policy-runner:structured-error-code" "Gateway policy runner must retain structured Codex++ policy error codes."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "RequestId" "07-gateway-policy-runner:request-id" "Gateway policy runner must retain safe request IDs for audit correlation."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "Token values are intentionally not printed" "07-gateway-policy-runner:redaction" "Gateway policy runner must not print key values."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "FixtureMode" "07-gateway-policy-runner:fixture-mode" "Gateway policy runner must support fixture-mode self-test."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "AllowAdminAuditReads" "07-admin-audit-runner:read-opt-in" "Admin audit runner must keep real admin event reads opt-in."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "gateway_policy_rejected" "07-admin-audit-runner:rejection-event" "Admin audit runner must require gateway policy rejection events."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "Gateway request_id correlation" "07-admin-audit-runner:request-id-correlation" "Admin audit runner must prove request IDs match gateway evidence."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "redaction_applied" "07-admin-audit-runner:redaction-applied" "Admin audit runner must require redaction markers."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "FixtureMode" "07-admin-audit-runner:fixture-mode" "Admin audit runner must support fixture-mode self-test."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "new-07-evidence-run\.ps1" "07-local-e2e-runner:evidence-generator" "Local E2E runner must create the evidence scaffold."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "run-client-api-checks\.ps1" "07-local-e2e-runner:client-api-runner" "Local E2E runner must call the client API runner."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "run-browser-handoff-checks\.ps1" "07-local-e2e-runner:browser-handoff-runner" "Local E2E runner must optionally call the browser handoff runner."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "run-gateway-policy-checks\.ps1" "07-local-e2e-runner:gateway-policy-runner" "Local E2E runner must optionally call the gateway policy runner."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "run-admin-audit-checks\.ps1" "07-local-e2e-runner:admin-audit-runner" "Local E2E runner must optionally call the admin audit runner."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "docker build" "07-local-source-service:docker-build" "Local source service helper must build the current source image."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "admin-compliance\.\*\.md" "07-local-source-service:legal-docs" "Local source service helper must verify frontend-imported legal docs are available."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "client-bootstrap-route" "07-local-source-service:client-bootstrap-preflight" "Local source service helper must preflight the client bootstrap route."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "desktop-poll-route" "07-local-source-service:desktop-poll-preflight" "Local source service helper must preflight the desktop poll route."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "Expected one of" "07-local-source-service:explicit-status-allowlist" "Local source service helper must reject 404, 5xx and connection failures instead of accepting any non-404 status."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "container:running-for-probe" "07-local-source-service:probe-container-check" "Local source service helper must confirm the expected local source container is running before probe-only checks."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "does not replace real E2E tokens" "07-local-source-service:release-boundary" "Local source service helper must keep the release evidence boundary explicit."
    Test-TextContains "sub2api-main/deploy/docker-compose.dev.yml" "SUB2API_DEV_HOST_PORT:-8081" "07-dev-compose:default-8081" "Dev compose must default the source build to 8081, not 8080."
    Test-TextContains "sub2api-main/deploy/docker-compose.dev.yml" "SUB2API_DEV_DATA_ROOT:-\./\.codexplus-local" "07-dev-compose:isolated-data-root" "Dev compose must isolate local source data from deploy data directories."
    Test-TextContains "sub2api-main/deploy/docker-compose.dev.yml" "SUB2API_DEV_APP_CONTAINER:-sub2api-codexplus-local" "07-dev-compose:app-container-name" "Dev compose must expose a distinct default app container name."
    Test-TextContains "sub2api-main/deploy/.env.codexplus-local.example" "SUB2API_DEV_HOST_PORT=8081" "07-dev-env-example:host-port" "Local source env example must document the default 8081 host port."
    Test-TextContains "sub2api-main/deploy/.env.codexplus-local.example" "SUB2API_DEV_DATA_ROOT=\./\.codexplus-local" "07-dev-env-example:data-root" "Local source env example must document the isolated data root."
    Test-TextContains "sub2api-main/deploy/.env.codexplus-local.example" "JWT_SECRET=[0-9a-f]{64}" "07-dev-env-example:jwt-secret-shape" "Local source env example must provide a bootable 64-hex JWT secret shape."
    Test-TextContains "sub2api-main/deploy/.env.codexplus-local.example" "TOTP_ENCRYPTION_KEY=[0-9a-f]{64}" "07-dev-env-example:totp-key-shape" "Local source env example must provide a bootable 64-hex TOTP encryption key shape."
    Test-TextContains "sub2api-main/deploy/.gitignore" "\.codexplus-local/" "07-dev-gitignore:data-root" "Gitignore must exclude local source runtime data."
    Test-TextContains "sub2api-main/deploy/.gitignore" "\.env\.codexplus-local" "07-dev-gitignore:local-env" "Gitignore must exclude local source runtime env secrets."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/README.md" "docker compose --env-file \.env\.codexplus-local" "07-e2e-readme:dev-compose-command" "E2E runner README must document the isolated dev compose command."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/README.md" "start-local-source-service\.ps1" "07-e2e-readme:local-source-probe" "E2E runner README must document the local source route preflight helper."
    Test-TextContains "sub2api-main/tools/e2e/codexplus/README.md" "SUB2API_DEV_HOST_PORT=8082" "07-e2e-readme:dev-port-conflict" "E2E runner README must document how to move the dev source port."
    Test-TextContains "sub2api-main/deploy/README.md" "Local Source Build Without Replacing 8080" "07-deploy-readme:local-source-section" "Deploy README must document the local source build entry."
    Test-TextContains "sub2api-main/deploy/README.md" "127\.0\.0\.1:8081" "07-deploy-readme:local-source-port" "Deploy README must document the default local source port."
    Test-TextContains "sub2api-main/deploy/README.md" "SUB2API_DEV_HOST_PORT=8082" "07-deploy-readme:dev-port-conflict" "Deploy README must document how to resolve dev source port conflicts."
    Test-TextContains "sub2api-main/deploy/README.md" "down -v" "07-deploy-readme:dev-cleanup-boundary" "Deploy README must document the dev cleanup boundary."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" '\[switch\]\$SkipHandoff' "07-evidence-tooling-self-test:skip-handoff-param" "Evidence tooling self-test must expose a switch for skipping release handoff checks during short reruns."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "handoff-suite-skipped" "07-evidence-tooling-self-test:handoff-skip-marker" "Evidence tooling self-test must record when the handoff suite is intentionally skipped."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/release-local-verification.md" "-SkipHandoff" "07-local-verification:skip-handoff-short-run" "Local verification notes must document the short evidence-tooling rerun."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-aggregate-fails" "07-evidence-tooling-self-test:generated-aggregate-negative" "Evidence tooling self-test must cover generated aggregate negative path."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-e2e-env-template" "07-evidence-tooling-self-test:e2e-env-template" "Evidence tooling self-test must generate the E2E env template."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "e2e-readiness-missing-env-fails" "07-evidence-tooling-self-test:e2e-readiness-negative" "Evidence tooling self-test must cover missing E2E readiness env failure."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "e2e-readiness-fixture-passes" "07-evidence-tooling-self-test:e2e-readiness-positive" "Evidence tooling self-test must cover sanitized E2E readiness fixture pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "client-api-runner-missing-env-fails" "07-evidence-tooling-self-test:client-api-runner-negative" "Evidence tooling self-test must cover missing client API runner env failure."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "client-api-runner-fixture-passes" "07-evidence-tooling-self-test:client-api-runner-positive" "Evidence tooling self-test must cover the client API runner fixture path."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "browser-handoff-runner-session-start-opt-in-fails" "07-evidence-tooling-self-test:browser-handoff-runner-negative" "Evidence tooling self-test must cover the browser handoff runner opt-in failure."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "browser-handoff-runner-fixture-passes" "07-evidence-tooling-self-test:browser-handoff-runner-positive" "Evidence tooling self-test must cover the browser handoff runner fixture path."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "gateway-runner-opt-in-fails" "07-evidence-tooling-self-test:gateway-runner-negative" "Evidence tooling self-test must cover gateway runner opt-in failure."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "gateway-runner-fixture-passes" "07-evidence-tooling-self-test:gateway-runner-positive" "Evidence tooling self-test must cover gateway runner fixture pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "admin-audit-runner-read-opt-in-fails" "07-evidence-tooling-self-test:admin-audit-runner-negative" "Evidence tooling self-test must cover admin audit runner opt-in failure."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "admin-audit-runner-fixture-passes" "07-evidence-tooling-self-test:admin-audit-runner-positive" "Evidence tooling self-test must cover admin audit runner fixture pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "local-e2e-runner-fixture-passes" "07-evidence-tooling-self-test:local-e2e-runner-positive" "Evidence tooling self-test must cover the local E2E runner fixture path."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-gap-helper" "07-evidence-tooling-self-test:release-gap-helper" "Evidence tooling self-test must cover the release gap helper."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-inspection-missing-artifacts-fails" "07-evidence-tooling-self-test:package-artifact-negative" "Evidence tooling self-test must cover missing package artifacts."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-inspection-fixture-passes" "07-evidence-tooling-self-test:package-artifact-positive" "Evidence tooling self-test must cover sanitized package artifact fixture pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-inspection-embedded-fake-sk-token-fails" "07-evidence-tooling-self-test:package-artifact-token-negative" "Evidence tooling self-test must reject package artifacts with embedded API-key-shaped tokens."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-inspection-fixed-commercial-policy-fails" "07-evidence-tooling-self-test:package-artifact-policy-negative" "Evidence tooling self-test must reject package artifacts with fixed commercial policy fields."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-missing-snapshots-fails" "07-evidence-tooling-self-test:compat-snapshot-negative" "Evidence tooling self-test must cover missing compatibility snapshots."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-fixture-passes" "07-evidence-tooling-self-test:compat-snapshot-positive" "Evidence tooling self-test must cover sanitized compatibility snapshot fixture pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-only-evidence-verifier-fails" "07-evidence-tooling-self-test:compat-snapshot-only-verifier-negative" "Evidence tooling self-test must reject snapshot-only compatibility evidence as a complete runtime pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-camel-token-fields-fail" "07-evidence-tooling-self-test:compat-camel-token-negative" "Evidence tooling self-test must reject camelCase token fields in compatibility snapshots."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-manual-provider-content-change-fails" "07-evidence-tooling-self-test:compat-provider-content-negative" "Evidence tooling self-test must reject changed manual provider URL or credential fingerprints."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-token-fields-fail" "07-evidence-tooling-self-test:compat-snapshot-token-negative" "Evidence tooling self-test must reject compatibility snapshots with token fields."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-commercial-policy-fields-fail" "07-evidence-tooling-self-test:compat-snapshot-policy-negative" "Evidence tooling self-test must reject compatibility snapshots with commercial policy fields."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-only-evidence-verifier-fails" "07-evidence-tooling-self-test:compat-snapshot-verifier-negative" "Evidence tooling self-test must reject compatibility snapshot runner output as full runtime evidence."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-business-readiness-fails" "07-evidence-tooling-self-test:business-negative" "Evidence tooling self-test must reject generated business readiness scaffolds."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-business-readiness-passes" "07-evidence-tooling-self-test:business-positive" "Evidence tooling self-test must cover valid business readiness evidence."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "business-source-doc-unresolved-fails" "07-evidence-tooling-self-test:business-source-doc-negative" "Evidence tooling self-test must reject business readiness when required source docs retain unresolved launch decisions."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-readiness-summary-business-failed" "07-evidence-tooling-self-test:readiness-business-negative" "Evidence tooling self-test must record failed generated business readiness in readiness summary."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-coverage-summary-fails" "07-evidence-tooling-self-test:generated-coverage-negative" "Evidence tooling self-test must keep generated scaffold coverage incomplete."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-readiness-summary-fails" "07-evidence-tooling-self-test:generated-readiness-negative" "Evidence tooling self-test must cover readiness summary no-go for generated scaffolds."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-fixture-readiness-summary-fails" "07-evidence-tooling-self-test:fixture-readiness-negative" "Evidence tooling self-test must keep sanitized fixture readiness no-go."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-fixture-readiness-summary-fixture-marker" "07-evidence-tooling-self-test:fixture-readiness-marker" "Evidence tooling self-test must record fixture markers."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-fixture-readiness-summary-business-passed" "07-evidence-tooling-self-test:fixture-readiness-business-positive" "Evidence tooling self-test must verify business readiness inside readiness summaries."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "e2e-result-fail-fails" "07-evidence-tooling-self-test:e2e-result-fail" "Evidence tooling self-test must reject E2E evidence with a failed result."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "e2e-missing-level3-pass-fails" "07-evidence-tooling-self-test:e2e-level3-negative" "Evidence tooling self-test must reject E2E evidence without Level 3 pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-result-fail-fails" "07-evidence-tooling-self-test:package-result-fail" "Evidence tooling self-test must reject package evidence with a failed result."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-platform-manager-missing-codex-evidence-fails" "07-evidence-tooling-self-test:package-platform-negative" "Evidence tooling self-test must reject package evidence missing platform install proof."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-metadata-result-fail-fails" "07-evidence-tooling-self-test:package-metadata-result-fail" "Evidence tooling self-test must reject package metadata with a failed result."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-coverage-missing-fails" "07-evidence-tooling-self-test:package-artifact-coverage-negative" "Evidence tooling self-test must reject package artifact inspection with missing coverage."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-result-fail-fails" "07-evidence-tooling-self-test:compatibility-result-fail" "Evidence tooling self-test must reject compatibility evidence with a failed result."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-context-result-fail-fails" "07-evidence-tooling-self-test:compatibility-context-result-fail" "Evidence tooling self-test must reject compatibility context with a failed result."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-missing-manual-provider-fails" "07-evidence-tooling-self-test:compatibility-manual-provider-negative" "Evidence tooling self-test must reject compatibility evidence when manual providers are missing after upgrade."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-runtime-boundary-evidence-missing-fails" "07-evidence-tooling-self-test:compat-runtime-boundary-negative" "Evidence tooling self-test must reject compatibility evidence missing runtime-boundary proof."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "business-result-fail-fails" "07-evidence-tooling-self-test:business-result-fail" "Evidence tooling self-test must reject business readiness evidence with a failed result."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "readiness-summary-with-mismatched-coverage-input-fails" "07-evidence-tooling-self-test:readiness-coverage-input-negative" "Evidence tooling self-test must reject readiness summaries whose coverage summary points at different inputs."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "readiness-summary-mismatched-coverage-input-marker" "07-evidence-tooling-self-test:readiness-coverage-input-marker" "Evidence tooling self-test must assert the mismatched coverage input marker."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-coverage-summary-fails" "07-evidence-tooling-self-test:module-j-coverage-summary-required" "Evidence tooling self-test must reject Module J go reports without a coverage summary."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-mismatched-summary-paths-fails" "07-evidence-tooling-self-test:module-j-summary-path-consistency" "Evidence tooling self-test must reject Module J reports whose summary paths do not match verifier inputs."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-mismatched-evidence-inputs-fails" "07-evidence-tooling-self-test:module-j-evidence-input-consistency" "Evidence tooling self-test must reject Module J reports whose evidence inputs do not match generated summaries."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-level3-pass-fails" "07-evidence-tooling-self-test:module-j-level3-required" "Evidence tooling self-test must reject Module J go reports without Level 3 pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-missing-readiness-coverage-fails" "07-evidence-tooling-self-test:module-j-readiness-coverage-required" "Evidence tooling self-test must reject Module J reports when readiness summary omits coverage verification."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-readiness-not-allowed-fails" "07-evidence-tooling-self-test:module-j-readiness-allow-required" "Evidence tooling self-test must reject Module J go reports when readiness did not explicitly allow a go candidate."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-nongenerated-readiness-summary-fails" "07-evidence-tooling-self-test:module-j-readiness-generated-required" "Evidence tooling self-test must reject Module J go reports with non-generated readiness summaries."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-readiness-marker-despite-go-posture-fails" "07-evidence-tooling-self-test:module-j-readiness-marker-boundary" "Evidence tooling self-test must reject go-candidate readiness summaries that retain nonrelease markers."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-readiness-level3-failed-fails" "07-evidence-tooling-self-test:module-j-readiness-level3-boundary" "Evidence tooling self-test must reject readiness summaries without Level 3 pass."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-mismatched-readiness-coverage-path-fails" "07-evidence-tooling-self-test:module-j-readiness-coverage-path" "Evidence tooling self-test must reject readiness summaries that reference a different coverage summary path."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-no-go-readiness-summary-fails" "07-evidence-tooling-self-test:module-j-readiness-negative" "Evidence tooling self-test must reject Module J go reports when readiness summary is no-go."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-missing-business-readiness-summary-fails" "07-evidence-tooling-self-test:module-j-business-readiness-negative" "Evidence tooling self-test must reject Module J go reports when readiness summary omits business readiness."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-business-evidence-hygiene-fails" "07-evidence-tooling-self-test:module-j-business-evidence-hygiene-required" "Evidence tooling self-test must reject Module J go reports without business evidence hygiene fields."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-readiness-summary-fails" "07-evidence-tooling-self-test:module-j-readiness-summary-required" "Evidence tooling self-test must reject Module J go reports without a readiness summary."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-module-inputs-fails" "07-evidence-tooling-self-test:module-j-module-inputs-required" "Evidence tooling self-test must reject Module J go reports without all module report inputs."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-unapproved-contract-drift-fails" "07-evidence-tooling-self-test:module-j-contract-drift-required" "Evidence tooling self-test must reject Module J go reports with unapproved contract drift."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-go-policy-signals-fails" "07-evidence-tooling-self-test:module-j-go-policy-signals-required" "Evidence tooling self-test must reject Module J go reports without named go-policy evidence signals."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-verification-availability-fails" "07-evidence-tooling-self-test:module-j-verification-availability-required" "Evidence tooling self-test must reject Module J reports without skipped/unavailable check fields."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-conflict-resolution-fails" "07-evidence-tooling-self-test:module-j-conflict-resolution-required" "Evidence tooling self-test must reject Module J reports without conflict resolution fields."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-accepted-impact-fails" "07-evidence-tooling-self-test:module-j-accepted-impact-required" "Evidence tooling self-test must reject accepted-risk Module J reports without accepted impact."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-template-has-no-pending-field-labels" "07-evidence-tooling-self-test:module-j-template-no-pending-fields" "Evidence tooling self-test must guard against pending field labels in the Module J template."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-camelcase-token-field-fails" "07-evidence-tooling-self-test:module-j-camel-token-negative" "Evidence tooling self-test must reject camelCase token fields in Module J reports."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-env-secret-field-fails" "07-evidence-tooling-self-test:module-j-env-secret-negative" "Evidence tooling self-test must reject env-style secret fields in Module J reports."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-go-policy-keywords-outside-failure-paths-fails" "07-evidence-tooling-self-test:module-j-go-policy-field-scoped" "Evidence tooling self-test must reject go-policy keywords placed outside the failure paths field."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-go-candidate-readiness-summary-passes" "07-evidence-tooling-self-test:module-j-readiness-positive" "Evidence tooling self-test must allow Module J reports with a go-candidate readiness summary."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-release-handoff-fails" "07-evidence-tooling-self-test:handoff-negative" "Evidence tooling self-test must reject generated handoff scaffolds."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-index-run-stamp-mismatch-fails" "07-evidence-tooling-self-test:handoff-run-stamp-negative" "Evidence tooling self-test must reject handoffs whose index run stamp does not match the release directory."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-misleading-index-path-fields-fails" "07-evidence-tooling-self-test:handoff-index-path-field-negative" "Evidence tooling self-test must reject handoff indexes whose path fields do not match expected evidence paths."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-mismatched-readiness-allow-fails" "07-evidence-tooling-self-test:handoff-readiness-allow-negative" "Evidence tooling self-test must reject handoffs whose readiness AllowGoCandidate state does not match the verifier switch."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-mismatched-readiness-docs-result-fails" "07-evidence-tooling-self-test:handoff-readiness-docs-result-negative" "Evidence tooling self-test must reject handoffs whose readiness docs result does not match regenerated evidence."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-mismatched-readiness-business-result-fails" "07-evidence-tooling-self-test:handoff-readiness-business-result-negative" "Evidence tooling self-test must reject handoffs whose readiness business result does not match regenerated evidence."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-without-final-verification-results-fails" "07-evidence-tooling-self-test:handoff-final-results-required" "Evidence tooling self-test must reject handoff indexes without final verification results."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-mismatched-coverage-summary-fails" "07-evidence-tooling-self-test:handoff-coverage-summary-negative" "Evidence tooling self-test must reject handoffs whose stored coverage summary no longer matches regenerated evidence."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-release-handoff-passes" "07-evidence-tooling-self-test:handoff-positive" "Evidence tooling self-test must cover a consistent release handoff package."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-handoff-coverage-summary-passes" "07-evidence-tooling-self-test:handoff-coverage-positive" "Evidence tooling self-test must cover a complete handoff coverage summary."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-aggregate-passes" "07-evidence-tooling-self-test:aggregate-positive" "Evidence tooling self-test must cover aggregate positive path."
    Test-TextContains "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-module-j-report-passes" "07-evidence-tooling-self-test:module-j-positive" "Evidence tooling self-test must cover Module J report positive path."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "result-pass" "07-e2e-verifier:result-pass" "E2E evidence verifier must require result files to pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "final-recommendation-pass" "07-e2e-verifier:recommendation-pass" "E2E evidence verifier must reject no-go release recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "level3:pass-recorded" "07-e2e-verifier:level3-pass" "E2E evidence verifier must require Level 3 pass before Phase 1 go."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:browser-handoff-start-route" "07-e2e-verifier:browser-start-route" "E2E evidence verifier must require browser handoff start route evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:browser-handoff-complete-route" "07-e2e-verifier:browser-complete-route" "E2E evidence verifier must require browser handoff complete route evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:browser-handoff-poll-route" "07-e2e-verifier:browser-poll-route" "E2E evidence verifier must require browser handoff poll route evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:poll-token-not-in-authorize-url" "07-e2e-verifier:poll-token-url-boundary" "E2E evidence verifier must require poll token URL boundary evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:verification-code-six-digit" "07-e2e-verifier:six-digit-code" "E2E evidence verifier must require six digit verification code evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "verify-07-evidence\.ps1" "07-release-evidence-verifier:e2e" "Aggregate release evidence verifier must run E2E verifier."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "verify-07-package-evidence\.ps1" "07-release-evidence-verifier:package" "Aggregate release evidence verifier must run package verifier."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "verify-07-compatibility-evidence\.ps1" "07-release-evidence-verifier:compatibility" "Aggregate release evidence verifier must run compatibility verifier."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "DocsEvidenceDir" "07-release-evidence-verifier:docs-input" "Aggregate release evidence verifier must accept docs product copy evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "verify-07-docs-product-copy-evidence\.ps1" "07-release-evidence-verifier:docs" "Aggregate release evidence verifier must run docs product copy verifier."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "docs:verifier-passed" "07-release-evidence-verifier:docs-passed" "Aggregate release evidence verifier must require docs product copy verification to pass."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "Coverage status" "07-coverage-summary:status" "Coverage summary must emit a coverage status."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "Coverage Matrix" "07-coverage-summary:matrix" "Coverage summary must emit a coverage matrix."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "DocsEvidenceDir" "07-coverage-summary:docs-input" "Coverage summary must accept docs product copy evidence."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "Docs product copy evidence folder" "07-coverage-summary:docs-folder" "Coverage summary must emit docs product copy evidence input."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" 'Scan-NonReleaseMarkers .* "docs"' "07-coverage-summary:docs-marker-scan" "Coverage summary must scan docs product copy nonrelease markers."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "FailOnIncomplete" "07-coverage-summary:fail-on-incomplete" "Coverage summary must support fail-on-incomplete automation."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "Nonrelease Markers" "07-coverage-summary:nonrelease-markers" "Coverage summary must list nonrelease markers."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "does not execute E2E" "07-coverage-summary:boundary" "Coverage summary must state it does not execute release scenarios."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Recommended Module J posture" "07-readiness-summary:posture" "Readiness summary must emit a Module J posture."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "BusinessEvidenceDir" "07-readiness-summary:business-input" "Readiness summary must accept business readiness evidence."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "DocsEvidenceDir" "07-readiness-summary:docs-input" "Readiness summary must accept docs product copy evidence."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "verify-07-docs-product-copy-evidence\.ps1" "07-readiness-summary:docs-verifier" "Readiness summary must run docs product copy verification."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "CoverageSummaryFile" "07-readiness-summary:coverage-input" "Readiness summary must accept the generated coverage summary."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Coverage summary verification" "07-readiness-summary:coverage-verification" "Readiness summary must emit coverage summary verification."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "coverage-e2e-input-mismatch" "07-readiness-summary:coverage-input-consistency" "Readiness summary must reject coverage summaries generated from different evidence inputs."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "coverage-docs-input-mismatch" "07-readiness-summary:coverage-docs-input-consistency" "Readiness summary must reject coverage summaries generated from different docs inputs."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Business readiness verification" "07-readiness-summary:business-verification" "Readiness summary must emit business readiness verification."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Docs product copy verification" "07-readiness-summary:docs-verification" "Readiness summary must emit docs product copy verification."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "E2E Level 3 result" "07-readiness-summary:e2e-level3" "Readiness summary must emit E2E Level 3 result."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "level3-pass-missing" "07-readiness-summary:level3-marker" "Readiness summary must mark missing Level 3 pass as nonrelease."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "AllowGoCandidate" "07-readiness-summary:explicit-go-candidate" "Readiness summary must require explicit go-candidate allowance."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "FailOnNoGo" "07-readiness-summary:fail-on-no-go" "Readiness summary must support fail-on-no-go automation."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Nonrelease Markers" "07-readiness-summary:nonrelease-markers" "Readiness summary must list nonrelease markers."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "fixture" "07-readiness-summary:fixture-marker" "Readiness summary must detect fixture evidence."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "not the final Module J report" "07-readiness-summary:not-final-report" "Readiness summary must not claim to be final Module J."
    Test-TextContains "codex-plus-dev-plan/tools/new-07-business-readiness-evidence.ps1" "Business readiness result: pending" "07-business-readiness-generator:pending" "Business readiness generator must create a pending scaffold."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "Business readiness result" "07-business-readiness-verifier:result" "Business readiness verifier must check final result."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "business-result:pass" "07-business-readiness-verifier:result-pass" "Business readiness verifier must require final result pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" 'gate:\$\{gate\}:pass' "07-business-readiness-verifier:gate-pass" "Business readiness verifier must require each required gate to pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "SourceDocsRoot" "07-business-readiness-verifier:source-docs-root" "Business readiness verifier must allow source-doc roots for synthetic and real evidence checks."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "source-unresolved" "07-business-readiness-verifier:source-unresolved" "Business readiness verifier must fail unresolved required source-doc markers."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "PRODUCTION-ENVIRONMENT-MATRIX\.md" "07-business-readiness-verifier:production-source-scan" "Business readiness verifier must scan the production environment source."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "BUSINESS-CONFIG-DECISION-TABLE\.md" "07-business-readiness-verifier:business-source-scan" "Business readiness verifier must scan the business config source."
    Test-TextContains "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "BusinessSourceDocsRoot" "07-readiness-summary:business-source-docs-root" "Readiness summary must pass business source-doc roots through to the business verifier."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "BusinessSourceDocsRoot" "07-release-handoff-verifier:business-source-docs-root" "Release handoff verifier must pass business source-doc roots through to regenerated readiness checks."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "production environment values" "07-business-readiness-verifier:production-values" "Business readiness verifier must check production environment gate."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "cost control abuse spend caps emergency shutoff" "07-business-readiness-verifier:cost-abuse" "Business readiness verifier must check cost and abuse gate."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "Paid-user support and entitlement correction" "07-business-readiness-verifier:support" "Business readiness verifier must check paid-user support gate."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go with accepted risks" "07-module-j-report-verifier:recommendation-values" "Module J report verifier must allow the approved recommendation values."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "CoverageSummaryFile" "07-module-j-report-verifier:coverage-summary-param" "Module J report verifier must accept a generated coverage summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-coverage-summary" "07-module-j-report-verifier:coverage-summary-required" "Module J report verifier must require a coverage summary for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-coverage-summary-complete" "07-module-j-report-verifier:coverage-summary-complete" "Module J report verifier must require complete coverage for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-no-missing-coverage" "07-module-j-report-verifier:coverage-missing-boundary" "Module J report verifier must reject missing coverage for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-no-coverage-markers" "07-module-j-report-verifier:coverage-marker-boundary" "Module J report verifier must reject nonrelease coverage markers for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "release-evidence:coverage-summary-path" "07-module-j-report-verifier:coverage-summary-path" "Module J report verifier must compare the report coverage summary path with the verifier input."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "release-evidence:readiness-summary-path" "07-module-j-report-verifier:readiness-summary-path" "Module J report verifier must compare the report readiness summary path with the verifier input."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "e2e-path-coverage-consistency" "07-module-j-report-verifier:coverage-input-paths" "Module J report verifier must compare report evidence inputs with the coverage summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "business-path-readiness-consistency" "07-module-j-report-verifier:readiness-input-paths" "Module J report verifier must compare report evidence inputs with the readiness summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "ReadinessSummaryFile" "07-module-j-report-verifier:readiness-summary-param" "Module J report verifier must accept an optional readiness summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-summary" "07-module-j-report-verifier:readiness-summary-required" "Module J report verifier must require a readiness summary for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-coverage-pass" "07-module-j-report-verifier:readiness-coverage-boundary" "Module J report verifier must require readiness summary coverage verification for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-policy:level3-pass" "07-module-j-report-verifier:level3-go-policy" "Module J report verifier must require Level 3 pass for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:generated-status" "07-module-j-report-verifier:readiness-generated" "Module J report verifier must require generated readiness summaries."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:e2e-level3-result" "07-module-j-report-verifier:readiness-level3-field" "Module J report verifier must require readiness summaries to record E2E Level 3 result."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-level3-pass" "07-module-j-report-verifier:readiness-level3-boundary" "Module J report verifier must require readiness Level 3 pass for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-no-readiness-markers" "07-module-j-report-verifier:readiness-marker-boundary" "Module J report verifier must reject readiness nonrelease markers for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:coverage-status-consistency" "07-module-j-report-verifier:readiness-coverage-consistency" "Module J report verifier must compare readiness and coverage summary coverage status."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:coverage-summary-path" "07-module-j-report-verifier:readiness-coverage-summary-path" "Module J report verifier must compare readiness summary coverage path with the supplied coverage summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:allow-go-candidate" "07-module-j-report-verifier:readiness-allow-field" "Module J report verifier must require readiness summaries to record go-candidate allowance."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-allow-go-candidate" "07-module-j-report-verifier:readiness-allow-boundary" "Module J report verifier must reject go recommendations when readiness did not explicitly allow a go candidate."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-module-reports-a-i" "07-module-j-report-verifier:module-reports" "Module J report verifier must require Module A-I report inputs for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-merge-order" "07-module-j-report-verifier:merge-order" "Module J report verifier must require documented merge order for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-go-candidate" "07-module-j-report-verifier:readiness-go-boundary" "Module J report verifier must reject go recommendations when readiness summary is no-go."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:aggregate-consistency" "07-module-j-report-verifier:readiness-aggregate-consistency" "Module J report verifier must compare report and readiness summary aggregate results."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-business-readiness-pass" "07-module-j-report-verifier:business-readiness-boundary" "Module J report verifier must require passed business readiness for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" 'Get-ReportSectionText "Release evidence hygiene"' "07-module-j-report-verifier:release-evidence-section-scoped" "Module J report verifier must scope release evidence hygiene checks to the release evidence section."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Business readiness folder" "07-module-j-report-verifier:business-evidence-field" "Module J report verifier must require a business readiness evidence folder in the report."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Docs product copy evidence folder" "07-module-j-report-verifier:docs-evidence-field" "Module J report verifier must require a docs product copy evidence folder in the report."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Coverage summary status" "07-module-j-report-verifier:coverage-status-field" "Module J report verifier must require coverage summary status in the report."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "release-evidence:business-readiness-result" "07-module-j-report-verifier:business-evidence-result" "Module J report verifier must require a business readiness verification result in the report."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Docs product copy verification" "07-module-j-report-verifier:docs-evidence-result" "Module J report verifier must require a docs product copy verification result in the report."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-docs-product-copy-pass" "07-module-j-report-verifier:docs-go-boundary" "Module J report verifier must require passed docs product copy evidence for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "docs-path-coverage-consistency" "07-module-j-report-verifier:docs-coverage-input-paths" "Module J report verifier must compare docs report input with the coverage summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "docs-path-readiness-consistency" "07-module-j-report-verifier:docs-readiness-input-paths" "Module J report verifier must compare docs report input with the readiness summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-business-readiness-report-pass" "07-module-j-report-verifier:business-report-boundary" "Module J report verifier must require passed business readiness in the report for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "accepted-risks-require-impact" "07-module-j-report-verifier:accepted-impact-boundary" "Module J report verifier must require accepted impact for accepted-risk recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "no-unapproved-contract-drift" "07-module-j-report-verifier:contract-drift-boundary" "Module J report verifier must reject unapproved contract drift for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" 'Get-ReportSectionText "Contract changes from original plan"' "07-module-j-report-verifier:contract-section-scoped" "Module J report verifier must scope contract checks to the contract changes section."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "drift status" "07-module-j-report-verifier:contract-drift-status" "Module J report verifier must require contract drift status."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-policy:not-purchased-rejected" "07-module-j-report-verifier:go-policy-not-purchased" "Module J report verifier must require not-purchased rejection evidence for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Test-FailurePathSignal" "07-module-j-report-verifier:failure-path-field-signal" "Module J report verifier must scope go-policy failure signals to the failure paths field."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-policy:admin-config-bootstrap" "07-module-j-report-verifier:go-policy-admin-bootstrap" "Module J report verifier must require admin config bootstrap evidence for go recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "skipped or unavailable" "07-module-j-report-verifier:verification-availability" "Module J report verifier must require skipped/unavailable check disposition."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "replacement narrower check" "07-module-j-report-verifier:verification-replacement" "Module J report verifier must require replacement check disposition."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "env-secret" "07-module-j-report-verifier:env-secret-scan" "Module J report verifier must scan env-style secret assignments."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "desktopAccessToken" "07-module-j-report-verifier:camel-token-scan" "Module J report verifier must scan camelCase token fields."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" 'Get-ReportSectionText "Conflicts resolved"' "07-module-j-report-verifier:conflict-section-scoped" "Module J report verifier must scope conflict checks to the conflict section."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "rule used" "07-module-j-report-verifier:conflict-rule-used" "Module J report verifier must require ownership rule resolution fields."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "CoverageSummaryFile" "07-module-j-template:coverage-summary-file" "Module J report template must mention the coverage summary parameter."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "ReadinessSummaryFile" "07-module-j-template:readiness-summary-file" "Module J report template must mention the readiness summary parameter."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "module reports:\s*FILL_ME" "07-module-j-template:module-reports" "Module J report template must include module report inputs."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "merge order:\s*FILL_ME" "07-module-j-template:merge-order" "Module J report template must include merge order."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "drift status:\s*FILL_ME" "07-module-j-template:contract-drift-status" "Module J report template must include contract drift status."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Business readiness folder:\s*FILL_ME" "07-module-j-template:business-folder" "Module J report template must include a business readiness folder."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Docs product copy evidence folder" "07-module-j-template:docs-folder" "Module J report template must include a docs product copy evidence folder."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Release coverage summary:\s*FILL_ME" "07-module-j-template:coverage-summary" "Module J report template must include a release coverage summary."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Release readiness summary:\s*FILL_ME" "07-module-j-template:readiness-summary" "Module J report template must include a release readiness summary."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Coverage summary status:\s*incomplete" "07-module-j-template:coverage-status" "Module J report template must include a default incomplete coverage status."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Business readiness verification:\s*failed" "07-module-j-template:business-verification" "Module J report template must include a default failed business readiness verification result."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Docs product copy verification" "07-module-j-template:docs-verification" "Module J report template must include docs product copy verification."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "level 3 result:\s*FILL_ME" "07-module-j-template:level3-result" "Module J report template must include E2E Level 3 result."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "nonrelease boundary removed for release docs" "07-module-j-template:nonrelease-docs-boundary" "Module J report template must avoid pending field labels while preserving docs boundary evidence."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "admin config bootstrap:\s*FILL_ME" "07-module-j-template:admin-config-bootstrap" "Module J report template must include admin config bootstrap evidence."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "skipped or unavailable:\s*FILL_ME" "07-module-j-template:verification-availability" "Module J report template must include skipped/unavailable check disposition."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "impact:\s*FILL_ME" "07-module-j-template:risk-impact-field" "Module J report template must include risk impact."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "rule used:\s*FILL_ME" "07-module-j-template:conflict-rule-used" "Module J report template must include conflict rule used."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "ReleaseDir" "07-release-handoff-verifier:release-dir" "Release handoff verifier must accept a release directory."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "verify-07-release-evidence\.ps1" "07-release-handoff-verifier:aggregate" "Release handoff verifier must rerun aggregate evidence verification."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "verify-07-business-readiness\.ps1" "07-release-handoff-verifier:business-readiness" "Release handoff verifier must verify business readiness."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "verify-07-docs-product-copy-evidence\.ps1" "07-release-handoff-verifier:docs" "Release handoff verifier must verify docs product copy evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "business:evidence-dir-exists" "07-release-handoff-verifier:business-dir" "Release handoff verifier must require business evidence directory."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "docs:evidence-dir-exists" "07-release-handoff-verifier:docs-dir" "Release handoff verifier must require docs product copy evidence directory."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "summarize-07-release-coverage\.ps1" "07-release-handoff-verifier:coverage-summary" "Release handoff verifier must regenerate coverage summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "coverage-summary:complete" "07-release-handoff-verifier:coverage-complete" "Release handoff verifier must require complete coverage."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "coverage-summary:e2e-input" "07-release-handoff-verifier:coverage-input" "Release handoff verifier must compare stored coverage summary inputs."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "coverage-summary:missing-count-consistency" "07-release-handoff-verifier:coverage-count-consistency" "Release handoff verifier must compare stored and regenerated coverage counts."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "summarize-07-release-readiness\.ps1" "07-release-handoff-verifier:readiness-summary" "Release handoff verifier must regenerate readiness summary."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:e2e-input" "07-release-handoff-verifier:readiness-input" "Release handoff verifier must compare stored readiness summary inputs."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:coverage-summary-input" "07-release-handoff-verifier:readiness-coverage-summary-input" "Release handoff verifier must compare readiness summary coverage summary inputs."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:coverage-consistency" "07-release-handoff-verifier:readiness-coverage-consistency" "Release handoff verifier must compare readiness coverage verification."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:business-consistency" "07-release-handoff-verifier:readiness-business-consistency" "Release handoff verifier must compare business readiness summary status."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:docs-result-consistency" "07-release-handoff-verifier:readiness-docs-result-consistency" "Release handoff verifier must compare readiness docs result with regenerated evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:business-result-consistency" "07-release-handoff-verifier:readiness-business-result-consistency" "Release handoff verifier must compare readiness business result with regenerated evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "verify-07-module-j-report\.ps1" "07-release-handoff-verifier:module-j-report" "Release handoff verifier must verify the Module J final report."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:final-status" "07-release-handoff-verifier:final-index" "Release handoff verifier must require a final handoff index."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "run-stamp-release-dir-consistency" "07-release-handoff-verifier:run-stamp-consistency" "Release handoff verifier must require the index run stamp to match the release directory."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:docs-path" "07-release-handoff-verifier:index-docs-path" "Release handoff verifier must require docs evidence path in the index."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:docs-result-field" "07-release-handoff-verifier:index-docs-result" "Release handoff verifier must require docs verification result in the index."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:aggregate-result-field" "07-release-handoff-verifier:index-aggregate-result" "Release handoff verifier must require final aggregate result in the index."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "coverage-summary:docs-input" "07-release-handoff-verifier:coverage-docs-input" "Release handoff verifier must compare stored coverage summary docs input."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:docs-input" "07-release-handoff-verifier:readiness-docs-input" "Release handoff verifier must compare stored readiness summary docs input."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:docs-consistency" "07-release-handoff-verifier:readiness-docs-consistency" "Release handoff verifier must compare docs readiness summary status."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:stored-generated-status" "07-release-handoff-verifier:readiness-generated" "Release handoff verifier must require generated readiness summaries."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:allow-go-candidate-consistency" "07-release-handoff-verifier:readiness-allow-consistency" "Release handoff verifier must compare stored and regenerated AllowGoCandidate state."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:marker-section-consistency" "07-release-handoff-verifier:readiness-marker-consistency" "Release handoff verifier must compare stored and regenerated readiness marker sections."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:allow-go-candidate-switch-consistency" "07-release-handoff-verifier:readiness-allow-switch" "Release handoff verifier must bind readiness AllowGoCandidate state to the verifier switch."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:final-recommendation-consistency" "07-release-handoff-verifier:index-final-recommendation" "Release handoff verifier must compare index and report recommendations."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "module-j-final-report:recommendation-field" "07-release-handoff-verifier:report-recommendation" "Release handoff verifier must read the Module J report recommendation."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff consistency only" "07-release-handoff-verifier:boundary" "Release handoff verifier must state its boundary."
    foreach ($symbol in @("manual providers", "compatibility evidence pending", "rollback", "provider")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" $symbol "07-compatibility-checklist:$symbol" "$symbol must be present in compatibility checklist."
    }
    Test-TextContains "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" "new-07-compatibility-evidence\.ps1" "07-compatibility-checklist:compat-generator" "Compatibility checklist must mention the compatibility evidence generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" "inspect-07-compatibility-snapshots\.ps1" "07-compatibility-checklist:snapshot-inspector" "Compatibility checklist must mention the snapshot inspection runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" "verify-07-compatibility-evidence\.ps1" "07-compatibility-checklist:compat-verifier" "Compatibility checklist must require the compatibility evidence verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "new-07-compatibility-evidence\.ps1" "07-compat-template:compat-generator" "Provider settings evidence template must document the generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "inspect-07-compatibility-snapshots\.ps1" "07-compat-template:snapshot-inspector" "Provider settings evidence template must document the snapshot inspection runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "verify-07-compatibility-evidence\.ps1" "07-compat-template:compat-verifier" "Provider settings evidence template must document the verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/compatibility/rollback-notes.md" "verify-07-compatibility-evidence\.ps1" "07-compat-rollback:compat-verifier" "Rollback notes must reference the compatibility verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md" "new-07-compatibility-evidence\.ps1" "07-compat-task:compat-generator" "Compatibility task must list the evidence generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md" "inspect-07-compatibility-snapshots\.ps1" "07-compat-task:snapshot-inspector" "Compatibility task must list the snapshot inspection runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md" "verify-07-compatibility-evidence\.ps1" "07-compat-task:compat-verifier" "Compatibility task must list the evidence verifier."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "compat-report:result-pass" "07-compat-verifier:result-pass" "Compatibility evidence verifier must require final result pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:result-pass" "07-compat-verifier:context-result-pass" "Compatibility evidence verifier must require the snapshot context result to pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:no-missing-inputs" "07-compat-verifier:no-missing-inputs" "Compatibility evidence verifier must require no missing snapshot inputs."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:pre-upgrade-parsed" "07-compat-verifier:pre-upgrade-parsed" "Compatibility evidence verifier must require the pre-upgrade snapshot to parse."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:post-upgrade-parsed" "07-compat-verifier:post-upgrade-parsed" "Compatibility evidence verifier must require the post-upgrade snapshot to parse."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:logout-parsed" "07-compat-verifier:logout-parsed" "Compatibility evidence verifier must require the logout snapshot to parse."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:rollback-parsed" "07-compat-verifier:rollback-parsed" "Compatibility evidence verifier must require the rollback snapshot to parse."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:no-parse-failures" "07-compat-verifier:no-parse-failures" "Compatibility evidence verifier must require no snapshot parse failures."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "pre-upgrade:manual-provider-count" "07-compat-verifier:manual-provider-count" "Compatibility evidence verifier must require at least one pre-upgrade manual provider."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:managed-cloud" "07-compat-verifier:managed-cloud" "Compatibility evidence verifier must require managed Cloud provider evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:managed-runtime-only-config" "07-compat-verifier:managed-runtime-only-config" "Compatibility evidence verifier must require managed provider runtime-only config proof."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:advanced-configuration-reachable" "07-compat-verifier:advanced-configuration-reachable" "Compatibility evidence verifier must require advanced configuration remains reachable proof."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:no-commercial-policy" "07-compat-verifier:no-commercial-policy" "Compatibility evidence verifier must require no local commercial policy write."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:no-missing-manual" "07-compat-verifier:no-missing-manual-after-upgrade" "Compatibility evidence verifier must require no missing manual providers after upgrade."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:manual-content-unchanged" "07-compat-verifier:manual-content-unchanged-after-upgrade" "Compatibility evidence verifier must require manual provider URL and credential fingerprints to remain unchanged."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "manual-switch:runtime-request-pass" "07-compat-verifier:runtime-manual-provider-request" "Compatibility evidence verifier must require runtime manual provider request proof."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "manual-switch:default-managed-entry-point" "07-compat-verifier:default-managed-entry-point" "Compatibility evidence verifier must require default managed entry-point proof."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "Runtime compatibility result" "07-compat-verifier:runtime-result-required" "Compatibility evidence verifier must require runtime compatibility result, not snapshot-only proof."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "logout:token-scan-clear" "07-compat-verifier:logout-token-scan-clear" "Compatibility evidence verifier must require logout token-field scan clearance."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "provider-sync:log-secret-scan-clear" "07-compat-verifier:provider-sync-secret-scan" "Compatibility evidence verifier must require provider sync log secret-scan proof."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "rollback:no-missing-manual" "07-compat-verifier:rollback-no-missing-manual" "Compatibility evidence verifier must require no missing manual providers after rollback."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "rollback:failed-provider-write-recovery" "07-compat-verifier:failed-provider-write-recovery" "Compatibility evidence verifier must require failed provider write recovery proof."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "compat-report:snapshot-inspector-command" "07-compat-verifier:snapshot-inspector-command" "Compatibility evidence verifier must require the snapshot inspector command to be recorded."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "FixtureMode" "07-compat-snapshot-inspector:fixture-mode" "Compatibility snapshot inspector must support fixture-mode self-test."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "PreUpgradeSnapshot" "07-compat-snapshot-inspector:pre-snapshot" "Compatibility snapshot inspector must accept pre-upgrade snapshot input."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "PostUpgradeSnapshot" "07-compat-snapshot-inspector:post-snapshot" "Compatibility snapshot inspector must accept post-upgrade snapshot input."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "LogoutSnapshot" "07-compat-snapshot-inspector:logout-snapshot" "Compatibility snapshot inspector must accept logout snapshot input."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "RollbackSnapshot" "07-compat-snapshot-inspector:rollback-snapshot" "Compatibility snapshot inspector must accept rollback snapshot input."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "Manual providers preserved after upgrade" "07-compat-snapshot-inspector:manual-preserved" "Compatibility snapshot inspector must check manual provider preservation."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "relayProfiles" "07-compat-snapshot-inspector:relay-profiles" "Compatibility snapshot inspector must parse legacy relayProfiles/settings snapshots."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "Manual provider content unchanged after upgrade" "07-compat-snapshot-inspector:manual-content-unchanged" "Compatibility snapshot inspector must compare manual provider content without printing secrets."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "nonprinted base URL/API key hashes" "07-compat-snapshot-inspector:manual-content-hashes" "Compatibility snapshot inspector must use nonprinted URL/API key hashes for manual content comparison."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "accessToken" "07-compat-snapshot-inspector:camel-token-scan" "Compatibility snapshot inspector must scan camelCase token fields."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "desktopAccessToken" "07-compat-snapshot-inspector:desktop-token-scan" "Compatibility snapshot inspector must scan desktop token fields."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "KEY\|TOKEN\|SECRET\|PASSWORD" "07-compat-snapshot-inspector:env-secret-redaction" "Compatibility snapshot inspector must redact env-style secret assignments."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "Codex\+\+ Cloud" "07-compat-snapshot-inspector:managed-cloud" "Compatibility snapshot inspector must check managed Cloud provider presence."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "No plan, price, multiplier, entitlement, or usage policy" "07-compat-snapshot-inspector:no-commercial-policy" "Compatibility snapshot inspector must check local commercial-policy absence."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "allSnapshotsHaveNoCommercialPolicy" "07-compat-snapshot-inspector:all-snapshots-policy-scan" "Compatibility snapshot inspector must scan every snapshot for commercial policy fields."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "allSnapshotsClearTokenFields" "07-compat-snapshot-inspector:all-snapshots-token-scan" "Compatibility snapshot inspector must scan every snapshot for token fields."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "token values intentionally not printed" "07-compat-snapshot-inspector:redaction" "Compatibility snapshot inspector must not print token values."
    foreach ($symbol in @("Windows", "macOS", "package evidence pending", "shared Key")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/package-install/package-install-checklist.md" $symbol "07-package-checklist:$symbol" "$symbol must be present in package checklist."
    }
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "FixtureMode" "07-package-artifact-inspector:fixture-mode" "Package artifact inspector must support fixture-mode self-test."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "windows-x64-setup" "07-package-artifact-inspector:windows-coverage" "Package artifact inspector must check Windows setup coverage."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "macos-x64-dmg" "07-package-artifact-inspector:macos-x64-coverage" "Package artifact inspector must check macOS x64 DMG coverage."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "macos-arm64-dmg" "07-package-artifact-inspector:macos-arm64-coverage" "Package artifact inspector must check macOS arm64 DMG coverage."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "values intentionally not printed" "07-package-artifact-inspector:redaction" "Package artifact inspector must not print matched secret values."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "Package install does not overwrite existing manual provider configuration" "07-package-artifact-inspector:manual-provider-boundary" "Package artifact inspector must keep provider overwrite as a separate platform proof."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "plan_id" "07-package-artifact-inspector:plan-id-scan" "Package artifact inspector must scan for fixed plan ids."
    Test-TextContains "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "quota" "07-package-artifact-inspector:quota-scan" "Package artifact inspector must scan for fixed quota policy."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/package-install/package-install-checklist.md" "verify-07-package-evidence\.ps1" "07-package-checklist:package-verifier" "Package checklist must require the package evidence verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/package-install/package-install-checklist.md" "inspect-07-package-artifacts\.ps1" "07-package-checklist:artifact-inspector" "Package checklist must mention the artifact inspection runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md" "new-07-package-evidence\.ps1" "07-package-template:package-generator" "Platform evidence template must document the package evidence generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md" "inspect-07-package-artifacts\.ps1" "07-package-template:artifact-inspector" "Platform evidence template must document the artifact inspection runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md" "verify-07-package-evidence\.ps1" "07-package-template:package-verifier" "Platform evidence template must document the package evidence verifier."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/task-package-install-check.md" "new-07-package-evidence\.ps1" "07-package-task:package-generator" "Package task must list the package evidence generator."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/task-package-install-check.md" "inspect-07-package-artifacts\.ps1" "07-package-task:artifact-inspector" "Package task must list the artifact inspection runner."
    Test-TextContains "codex-plus-dev-plan/07-integration-release/task-package-install-check.md" "verify-07-package-evidence\.ps1" "07-package-task:package-verifier" "Package task must list the package evidence verifier."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "package-report:result-pass" "07-package-verifier:result-pass" "Package evidence verifier must require final result pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "metadata:result-pass" "07-package-verifier:metadata-result-pass" "Package evidence verifier must require artifact metadata result to pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "metadata:windows-coverage-present" "07-package-verifier:metadata-coverage" "Package evidence verifier must require expected artifact coverage in metadata."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "metadata:macos-x64-coverage-present" "07-package-verifier:metadata-macos-x64-coverage" "Package evidence verifier must require expected macOS x64 artifact coverage in metadata."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "metadata:macos-arm64-coverage-present" "07-package-verifier:metadata-macos-arm64-coverage" "Package evidence verifier must require expected macOS arm64 artifact coverage in metadata."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "windows:desktop-main-shortcut" "07-package-verifier:windows-desktop-shortcut" "Package evidence verifier must require Windows desktop shortcut evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "windows:apps-and-features" "07-package-verifier:windows-apps-and-features" "Package evidence verifier must require Windows Apps and Features evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "windows:manager-advanced-configuration" "07-package-verifier:windows-manager-advanced-configuration" "Package evidence verifier must require Manager advanced configuration evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "windows:missing-codex-first-run" "07-package-verifier:windows-missing-codex-first-run" "Package evidence verifier must require Missing-Codex first-run evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "macos-x64:hidden-dock" "07-package-verifier:macos-x64-hidden-dock" "Package evidence verifier must require macOS x64 hidden Dock evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "macos-arm64:missing-codex-first-run" "07-package-verifier:macos-arm64-missing-codex-first-run" "Package evidence verifier must require macOS arm64 Missing-Codex evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:result-pass" "07-package-verifier:inspection-result-pass" "Package evidence verifier must require artifact inspection result to pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:inspector-command" "07-package-verifier:inspector-command" "Package evidence verifier must require the artifact inspector command to be recorded."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:no-shared-key" "07-package-verifier:no-shared-key" "Package evidence verifier must require no shared key evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:no-user-credentials" "07-package-verifier:no-user-credentials" "Package evidence verifier must require no user credential evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:no-fixed-policy" "07-package-verifier:no-fixed-policy" "Package evidence verifier must require no fixed commercial policy evidence."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:installer-script-pass" "07-package-verifier:installer-script-pass" "Package evidence verifier must require installer script credential scan pass."
    Test-TextContains "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:no-scanner-findings" "07-package-verifier:no-scanner-findings" "Package evidence verifier must require scanner findings to be clear."
    foreach ($symbol in @("backend-configured", "Control Plane", "Rollback", "pending")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/docs/docs-sync-record.md" $symbol "07-docs-sync:$symbol" "$symbol must be present in docs sync record."
    }
    foreach ($symbol in @("static sync passed", "local Chromium visual evidence passed", "in-app browser HTTP preview visual evidence passed", "direct local-file navigation blocked", "URL policy", "in-app-browser-policy-boundary", "in-app-browser-http-desktop\.png", "in-app-browser-http-mobile\.png", "Control Plane", "gpt-5")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/docs/html-sync-evidence.md" $symbol "07-html-sync:$symbol" "$symbol must be present in HTML sync evidence."
    }
    foreach ($symbol in @("Browser runtime connection: succeeded", "HTTP visual evidence passed", "Preview URL", "127\.0\.0\.1:8099", "in-app-browser-http-desktop\.png", "in-app-browser-http-mobile\.png", "Direct .*file://.* rendering")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/docs/html-visual-evidence/in-app-browser-policy-boundary.md" $symbol "07-in-app-browser-boundary:$symbol" "$symbol must be present in in-app browser boundary evidence."
    }
    foreach ($symbol in @("local verification passed", "release evidence pending", "release recommendation no-go", "go test ./\.\.\.", "npm run typecheck", "npm run build", "npm run vite:build", "cargo fmt --check -p codex-plus-core", "quotaProgress", "Local Chromium desktop/mobile screenshot evidence passed", "URL policy", "in-app-browser-policy-boundary", "verify-07-e2e-readiness\.ps1", "new-07-e2e-env-template\.ps1", "CODEXPLUS_07_E2E", "test-environment readiness", "run-client-api-checks\.ps1", "run-browser-handoff-checks\.ps1", "AllowSessionStart", "AllowBrowserComplete", "run-gateway-policy-checks\.ps1", "AllowGatewayRequests", "run-admin-audit-checks\.ps1", "AllowAdminAuditReads", "run-local-e2e\.ps1", "start-local-source-service\.ps1", "docker-compose.dev\.yml", "\.env\.codexplus-local\.example", "\.codexplus-local", "sub2api-codexplus-local", "127\.0\.0\.1:8081", "client API subset", "verify-07-rust-preflight\.ps1", "rust-toolchain", "9\.56GB", "test-07-evidence-tooling\.ps1", "new-07-release-evidence-set\.ps1", "new-07-evidence-run\.ps1", "new-07-package-evidence\.ps1", "inspect-07-package-artifacts\.ps1", "verify-07-package-evidence\.ps1", "new-07-compatibility-evidence\.ps1", "inspect-07-compatibility-snapshots\.ps1", "verify-07-compatibility-evidence\.ps1", "report-07-release-gaps\.ps1", "new-07-business-readiness-evidence\.ps1", "verify-07-business-readiness\.ps1", "verify-07-release-evidence\.ps1", "summarize-07-release-coverage\.ps1", "summarize-07-release-readiness\.ps1", "verify-07-release-handoff\.ps1")) {
        Test-TextContains "codex-plus-dev-plan/07-integration-release/release-local-verification.md" $symbol "07-local-verification:$symbol" "$symbol must be present in local release verification evidence."
    }
    foreach ($symbol in @("--quota-progress", "quotaProgress", "Control Plane", "Data Plane", "Client Runtime", "Platform Ops")) {
        Test-TextContains "codex-plus-product-spec.html" $symbol "07-html-product-spec:$symbol" "$symbol must be present in HTML product spec."
    }
    Test-NoResidue @(
        "codex-plus-product-spec.html"
    ) @("--quota:65%", "style\.setProperty\(`"--quota`",\s*data\.quota\)", "gpt-5-mini", "sk-user-managed-token", "remaining_percent", "today_tokens", "82%")
}
$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "Stage gate failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Stage gate passed." -ForegroundColor Green
exit 0
