param(
    [string]$Root
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

function Read-RepoText {
    param([string]$RelativePath)
    return Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $Root $RelativePath)
}

function Test-RepoFile {
    param([string]$RelativePath)
    Add-Check "file-exists:$RelativePath" (Test-Path -LiteralPath (Join-Path $Root $RelativePath) -PathType Leaf) $RelativePath
}

function Test-RepoText {
    param(
        [string]$RelativePath,
        [string]$Pattern,
        [string]$CheckName
    )
    $text = Read-RepoText $RelativePath
    Add-Check $CheckName ($text -match $Pattern) "$Pattern in $RelativePath"
}

function Test-RepoTextNot {
    param(
        [string]$RelativePath,
        [string]$Pattern,
        [string]$CheckName
    )
    $text = Read-RepoText $RelativePath
    Add-Check $CheckName ($text -notmatch $Pattern) "$Pattern absent from $RelativePath"
}

foreach ($file in @(
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
    "sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1",
    "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1",
    "sub2api-main/deploy/docker-compose.dev.yml",
    "sub2api-main/deploy/.env.codexplus-local.example",
    "sub2api-main/deploy/.gitignore",
    "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1",
    "codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1",
    "codex-plus-dev-plan/tools/accept-07-local-admin-compliance.ps1",
    "codex-plus-dev-plan/tools/new-07-desktop-compatibility-harness.ps1",
    "codex-plus-dev-plan/tools/capture-07-desktop-provider-snapshot.ps1",
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
)) {
    Test-RepoFile $file
}

$ledger = Read-RepoText "codex-plus-dev-plan/STAGE-GATE-LEDGER.md"
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
    Test-RepoText $taskFile "## 解耦要求" "07-task-decoupling:$taskFile"
    Test-RepoText $taskFile "## 禁止改动范围" "07-task-forbidden-scope:$taskFile"
    Test-RepoText $taskFile "## 测试要求" "07-task-tests:$taskFile"
    Test-RepoText $taskFile "## 交付物" "07-task-deliverables:$taskFile"
}

foreach ($symbol in @("Status:\s*active", "06-commerce-and-enforcement", "INTEGRATION-VERIFICATION-CHECKLIST", "PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN", "PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/README.md" $symbol "07-readme:$symbol"
}
foreach ($symbol in @("verify-07-release-evidence\.ps1", "verify-07-business-readiness\.ps1", "verify-07-release-handoff\.ps1", "report-07-release-gaps\.ps1", "ReadinessSummaryFile", "business/legal approval", "Release coverage summary and readiness summary")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/README.md" $symbol "07-readme-release-chain:$symbol"
}

foreach ($symbol in @("Browser handoff", "Turnstile", "One model request succeeds through Sub2API gateway", "Rollback notes")) {
    Test-RepoText "codex-plus-dev-plan/INTEGRATION-VERIFICATION-CHECKLIST.md" $symbol "07-checklist:$symbol"
}

foreach ($symbol in @("Target Evidence Structure", "Release Gate Decisions", "evidence folder", "release gate decision")) {
    Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" $symbol "07-module-i:$symbol"
}
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "new-07-release-evidence-set\.ps1" "07-module-i:release-evidence-set-generator"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "new-07-evidence-run\.ps1" "07-module-i:evidence-generator"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-client-api-checks\.ps1" "07-module-i:client-api-runner"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-browser-handoff-checks\.ps1" "07-module-i:browser-handoff-runner"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-gateway-policy-checks\.ps1" "07-module-i:gateway-policy-runner"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-admin-audit-checks\.ps1" "07-module-i:admin-audit-runner"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "gateway-matched .*request_id" "07-module-i:admin-audit-request-id-correlation"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "run-local-e2e\.ps1" "07-module-i:local-e2e-runner"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "verify-07-e2e-readiness\.ps1" "07-module-i:e2e-readiness-verifier"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "new-07-e2e-env-template\.ps1" "07-module-i:e2e-env-template-generator"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-I-E2E-RELEASE-GATE-PLAN.md" "verify-07-evidence\.ps1" "07-module-i:evidence-verifier"

foreach ($symbol in @("Final Report Structure", "Go/No-Go Policy", "Rollback notes", "Conflict Resolution Rules", "verify-07-release-evidence\.ps1", "summarize-07-release-coverage\.ps1", "summarize-07-release-readiness\.ps1", "verify-07-business-readiness\.ps1", "verify-07-module-j-report\.ps1", "verify-07-release-handoff\.ps1")) {
    Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md" $symbol "07-module-j:$symbol"
}
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md" "ReadinessSummaryFile" "07-module-j:readiness-summary-file"
Test-RepoText "codex-plus-dev-plan/PHASE1-MODULE-J-INTEGRATION-COORDINATOR-PLAN.md" "new-07-release-evidence-set\.ps1" "07-module-j:release-evidence-set-generator"

foreach ($reportSymbol in @("Report status:\s*in-progress", "Worker lane:\s*Coordinator", "verify-07-static\.ps1", "E2E buy/login/launch", "Compatibility and migration", "Package install check", "Docs and product copy")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/coordinator-integration-release-verification.md" $reportSymbol "07-report:$reportSymbol"
}

foreach ($worker in @(
    @{ Lane = "E2E"; File = "codex-plus-dev-plan/07-integration-release/reports/worker-e2e-buy-login-launch-final.md"; Pending = "E2E evidence pending" },
    @{ Lane = "Compatibility"; File = "codex-plus-dev-plan/07-integration-release/reports/worker-compatibility-migration-final.md"; Pending = "compatibility evidence pending" },
    @{ Lane = "Package"; File = "codex-plus-dev-plan/07-integration-release/reports/worker-package-install-final.md"; Pending = "package evidence pending" },
    @{ Lane = "Docs"; File = "codex-plus-dev-plan/07-integration-release/reports/worker-docs-product-copy-final.md"; Pending = "pending" }
)) {
    Test-RepoText $worker.File "Report status:\s*final" "07-worker-report-final:$($worker.Lane)"
    Test-RepoText $worker.File "Worker lane:\s*$($worker.Lane)" "07-worker-report-lane:$($worker.Lane)"
    Test-RepoText $worker.File "Forbidden edits:\s*none" "07-worker-report-forbidden:$($worker.Lane)"
    Test-RepoText $worker.File "## Verification" "07-worker-report-verification:$($worker.Lane)"
    Test-RepoText $worker.File "## Remaining [Rr]isks" "07-worker-report-risks:$($worker.Lane)"
    Test-RepoText $worker.File $worker.Pending "07-worker-report-pending-boundary:$($worker.Lane)"
}

foreach ($symbol in @("browser handoff", "Sub2API gateway", "E2E evidence pending", "No real API Key")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" $symbol "07-e2e-checklist:$symbol"
}
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "verify-07-evidence\.ps1" "07-e2e-checklist:evidence-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "new-07-evidence-run\.ps1" "07-e2e-checklist:evidence-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "verify-07-e2e-readiness\.ps1" "07-e2e-checklist:readiness-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "new-07-e2e-env-template\.ps1" "07-e2e-checklist:env-template-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "run-local-e2e\.ps1" "07-e2e-checklist:local-e2e-runner"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "run-admin-audit-checks\.ps1" "07-e2e-checklist:admin-audit-runner"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "accept-07-local-admin-compliance\.ps1" "07-e2e-checklist:local-compliance-accept"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "new-07-desktop-compatibility-harness\.ps1" "07-e2e-checklist:desktop-harness"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/manual-e2e-checklist.md" "capture-07-desktop-provider-snapshot\.ps1" "07-e2e-checklist:provider-snapshot-capture"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "verify-07-evidence\.ps1" "07-evidence-template:evidence-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "new-07-evidence-run\.ps1" "07-evidence-template:evidence-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "verify-07-e2e-readiness\.ps1" "07-evidence-template:readiness-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "new-07-e2e-env-template\.ps1" "07-evidence-template:env-template-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-client-api-checks\.ps1" "07-evidence-template:client-api-runner"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-browser-handoff-checks\.ps1" "07-evidence-template:browser-handoff-runner"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-gateway-policy-checks\.ps1" "07-evidence-template:gateway-policy-runner"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-admin-audit-checks\.ps1" "07-evidence-template:admin-audit-runner"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "run-local-e2e\.ps1" "07-evidence-template:local-e2e-runner"
Test-RepoText "codex-plus-dev-plan/07-integration-release/e2e/evidence-template/README.md" "-DocsEvidenceDir" "07-evidence-template:aggregate-docs-evidence-dir"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-evidence-run\.ps1" "07-release-evidence-set:e2e-generator"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-package-evidence\.ps1" "07-release-evidence-set:package-generator"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-compatibility-evidence\.ps1" "07-release-evidence-set:compatibility-generator"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-business-readiness-evidence\.ps1" "07-release-evidence-set:business-generator"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "new-07-docs-product-copy-evidence\.ps1" "07-release-evidence-set:docs-generator"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "verify-07-docs-product-copy-evidence\.ps1" "07-release-evidence-set:docs-verifier"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "-DocsEvidenceDir" "07-release-evidence-set:docs-evidence-dir"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "module-j-final-report-template\.md" "07-release-evidence-set:module-j-template"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "verify-07-release-evidence\.ps1" "07-release-evidence-set:aggregate-verifier"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "summarize-07-release-coverage\.ps1" "07-release-evidence-set:coverage-summary"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "summarize-07-release-readiness\.ps1" "07-release-evidence-set:readiness-summary"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "CoverageSummaryFile" "07-release-evidence-set:coverage-summary-file"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "ReadinessSummaryFile" "07-release-evidence-set:readiness-summary-file"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "verify-07-module-j-report\.ps1" "07-release-evidence-set:module-j-verifier"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "verify-07-release-handoff\.ps1" "07-release-evidence-set:handoff-verifier"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "Final Verification Results" "07-release-evidence-set:final-results"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "aggregate verifier result:\s*pending" "07-release-evidence-set:aggregate-result-placeholder"
Test-RepoText "codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1" "docs product copy verification:\s*pending" "07-release-evidence-set:docs-result-placeholder"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1" "rust-toolchain-available" "07-rust-preflight:toolchain"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1" "rust-linker-available" "07-rust-preflight:linker"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-rust-preflight.ps1" "disk:minimum-free-gb" "07-rust-preflight:disk"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "CODEXPLUS_07_E2E_BACKEND_BASE_URL" "07-e2e-readiness:backend-url-env"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "AllowProduction" "07-e2e-readiness:production-guard"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "ProbeHttp" "07-e2e-readiness:optional-http-probe"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "Add-EndpointPreflightCheck" "07-e2e-readiness:preflight-allowlist-helper"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "Expected one of" "07-e2e-readiness:preflight-status-allowlist"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "EndpointPreflightOnly" "07-e2e-readiness:preflight-only-mode"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "OutputPath" "07-e2e-readiness:output-report"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "value intentionally not printed" "07-e2e-readiness:redaction"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-e2e-readiness.ps1" "env-manager-build-windows-exe" "07-e2e-readiness:manager-exe-check"
Test-RepoText "codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1" "e2e-env.template.ps1" "07-e2e-env-template:env-file"
Test-RepoText "codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1" "Required Manual Inputs" "07-e2e-env-template:manual-inputs"
Test-RepoText "codex-plus-dev-plan/tools/new-07-e2e-env-template.ps1" "Do not commit real token values" "07-e2e-env-template:redaction-boundary"
Test-RepoText "codex-plus-dev-plan/tools/accept-07-local-admin-compliance.ps1" "AllowLocalComplianceAccept" "07-local-compliance-accept:explicit-opt-in"
Test-RepoText "codex-plus-dev-plan/tools/accept-07-local-admin-compliance.ps1" "Test-LocalAdminBaseUrl" "07-local-compliance-accept:local-url-guard"
Test-RepoText "codex-plus-dev-plan/tools/accept-07-local-admin-compliance.ps1" "Token values were intentionally not printed" "07-local-compliance-accept:redaction"
Test-RepoText "codex-plus-dev-plan/tools/new-07-desktop-compatibility-harness.ps1" "USERPROFILE" "07-desktop-harness:userprofile-isolation"
Test-RepoText "codex-plus-dev-plan/tools/new-07-desktop-compatibility-harness.ps1" "CODEX_HOME" "07-desktop-harness:codex-home-isolation"
Test-RepoText "codex-plus-dev-plan/tools/new-07-desktop-compatibility-harness.ps1" "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS" "07-desktop-harness:webview2-debug"
Test-RepoText "codex-plus-dev-plan/tools/new-07-desktop-compatibility-harness.ps1" "Safe prep only" "07-desktop-harness:safe-prep-boundary"
Test-RepoText "codex-plus-dev-plan/tools/capture-07-desktop-provider-snapshot.ps1" "base_url_hash" "07-provider-snapshot:base-url-hash"
Test-RepoText "codex-plus-dev-plan/tools/capture-07-desktop-provider-snapshot.ps1" "api_key_hash" "07-provider-snapshot:api-key-hash"
Test-RepoText "codex-plus-dev-plan/tools/capture-07-desktop-provider-snapshot.ps1" "raw secrets are not written" "07-provider-snapshot:no-raw-secret-output"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "verify-07-e2e-readiness\.ps1" "07-client-api-runner:readiness"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "FixtureMode" "07-client-api-runner:fixture-mode"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "AllowRedeem" "07-client-api-runner:redeem-opt-in"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "X-CodexPlus-Device-Id" "07-client-api-runner:device-header"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-client-api-checks.ps1" "Token value intentionally not printed" "07-client-api-runner:redaction"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "AllowSessionStart" "07-browser-handoff-runner:start-opt-in"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "AllowBrowserComplete" "07-browser-handoff-runner:complete-opt-in"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "BROWSER_AUTH_TOKEN" "07-browser-handoff-runner:browser-token-env"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "poll_token not in authorize_url" "07-browser-handoff-runner:poll-token-url-boundary"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "Session token, poll token, browser JWT" "07-browser-handoff-runner:redaction"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-browser-handoff-checks.ps1" "FixtureMode" "07-browser-handoff-runner:fixture-mode"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "AllowGatewayRequests" "07-gateway-policy-runner:request-opt-in"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "USER_ACTIVE_GATEWAY_KEY" "07-gateway-policy-runner:active-key-env"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "USER_MODEL_DENIED_GATEWAY_KEY" "07-gateway-policy-runner:model-denied-key-env"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "GATEWAY_POLICY_" "07-gateway-policy-runner:structured-error-code"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "RequestId" "07-gateway-policy-runner:request-id"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "Token values are intentionally not printed" "07-gateway-policy-runner:redaction"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-gateway-policy-checks.ps1" "FixtureMode" "07-gateway-policy-runner:fixture-mode"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "AllowAdminAuditReads" "07-admin-audit-runner:read-opt-in"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "gateway_policy_rejected" "07-admin-audit-runner:rejection-event"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "Gateway request_id correlation" "07-admin-audit-runner:request-id-correlation"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "redaction_applied" "07-admin-audit-runner:redaction-applied"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-admin-audit-checks.ps1" "FixtureMode" "07-admin-audit-runner:fixture-mode"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "new-07-evidence-run\.ps1" "07-local-e2e-runner:evidence-generator"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "run-client-api-checks\.ps1" "07-local-e2e-runner:client-api-runner"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "run-browser-handoff-checks\.ps1" "07-local-e2e-runner:browser-handoff-runner"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "run-gateway-policy-checks\.ps1" "07-local-e2e-runner:gateway-policy-runner"
Test-RepoText "sub2api-main/tools/e2e/codexplus/run-local-e2e.ps1" "run-admin-audit-checks\.ps1" "07-local-e2e-runner:admin-audit-runner"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "docker build" "07-local-source-service:docker-build"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "admin-compliance\.\*\.md" "07-local-source-service:legal-docs"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "client-bootstrap-route" "07-local-source-service:client-bootstrap-preflight"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "desktop-poll-route" "07-local-source-service:desktop-poll-preflight"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "Expected one of" "07-local-source-service:explicit-status-allowlist"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "container:running-for-probe" "07-local-source-service:probe-container-check"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-source-service.ps1" "does not replace real E2E tokens" "07-local-source-service:release-boundary"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1" "docker.*compose" "07-local-dev-compose:compose-command"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1" "com\.docker\.compose\.project" "07-local-dev-compose:compose-project-label"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1" "start-local-source-service\.ps1" "07-local-dev-compose:route-probe"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1" "InitEnv" "07-local-dev-compose:init-env"
Test-RepoText "sub2api-main/tools/e2e/codexplus/start-local-dev-compose.ps1" "ReplaceExisting" "07-local-dev-compose:replace-existing"
Test-RepoText "sub2api-main/deploy/docker-compose.dev.yml" "name:\s*sub2api-codexplus-local" "07-dev-compose:fixed-project-name"
Test-RepoText "sub2api-main/deploy/docker-compose.dev.yml" "SUB2API_DEV_HOST_PORT:-8081" "07-dev-compose:default-8081"
Test-RepoText "sub2api-main/deploy/docker-compose.dev.yml" "SUB2API_DEV_DATA_ROOT:-\./\.codexplus-local" "07-dev-compose:isolated-data-root"
Test-RepoText "sub2api-main/deploy/docker-compose.dev.yml" "SUB2API_DEV_APP_CONTAINER:-sub2api-codexplus-local" "07-dev-compose:app-container-name"
Test-RepoText "sub2api-main/deploy/.env.codexplus-local.example" "SUB2API_DEV_HOST_PORT=8081" "07-dev-env-example:host-port"
Test-RepoText "sub2api-main/deploy/.env.codexplus-local.example" "SUB2API_DEV_DATA_ROOT=\./\.codexplus-local" "07-dev-env-example:data-root"
Test-RepoText "sub2api-main/deploy/.env.codexplus-local.example" "JWT_SECRET=[0-9a-f]{64}" "07-dev-env-example:jwt-secret-shape"
Test-RepoText "sub2api-main/deploy/.env.codexplus-local.example" "TOTP_ENCRYPTION_KEY=[0-9a-f]{64}" "07-dev-env-example:totp-key-shape"
Test-RepoText "sub2api-main/deploy/.gitignore" "\.codexplus-local/" "07-dev-gitignore:data-root"
Test-RepoText "sub2api-main/deploy/.gitignore" "\.env\.codexplus-local" "07-dev-gitignore:local-env"
Test-RepoText "sub2api-main/tools/e2e/codexplus/README.md" "docker compose --env-file \.env\.codexplus-local" "07-e2e-readme:dev-compose-command"
Test-RepoText "sub2api-main/tools/e2e/codexplus/README.md" "start-local-dev-compose\.ps1" "07-e2e-readme:local-dev-compose-wrapper"
Test-RepoText "sub2api-main/tools/e2e/codexplus/README.md" "start-local-source-service\.ps1" "07-e2e-readme:local-source-probe"
Test-RepoText "sub2api-main/tools/e2e/codexplus/README.md" "SUB2API_DEV_HOST_PORT=8082" "07-e2e-readme:dev-port-conflict"
Test-RepoText "sub2api-main/deploy/README.md" "Local Source Build Without Replacing 8080" "07-deploy-readme:local-source-section"
Test-RepoText "sub2api-main/deploy/README.md" "start-local-dev-compose\.ps1" "07-deploy-readme:local-dev-compose-wrapper"
Test-RepoText "sub2api-main/deploy/README.md" "127\.0\.0\.1:8081" "07-deploy-readme:local-source-port"
Test-RepoText "sub2api-main/deploy/README.md" "SUB2API_DEV_HOST_PORT=8082" "07-deploy-readme:dev-port-conflict"
Test-RepoText "sub2api-main/deploy/README.md" "down -v" "07-deploy-readme:dev-cleanup-boundary"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" '\[switch\]\$SkipHandoff' "07-evidence-tooling-self-test:skip-handoff-param"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "handoff-suite-skipped" "07-evidence-tooling-self-test:handoff-skip-marker"
Test-RepoText "codex-plus-dev-plan/07-integration-release/release-local-verification.md" "-SkipHandoff" "07-local-verification:skip-handoff-short-run"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-aggregate-fails" "07-evidence-tooling-self-test:generated-aggregate-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-e2e-env-template" "07-evidence-tooling-self-test:e2e-env-template"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "e2e-readiness-missing-env-fails" "07-evidence-tooling-self-test:e2e-readiness-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "e2e-readiness-fixture-passes" "07-evidence-tooling-self-test:e2e-readiness-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "local-admin-compliance-accept-opt-in-fails" "07-evidence-tooling-self-test:local-compliance-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "desktop-compatibility-harness-generates" "07-evidence-tooling-self-test:desktop-harness-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "desktop-provider-snapshot-capture-generates" "07-evidence-tooling-self-test:provider-snapshot-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "desktop-provider-snapshot-hash-only" "07-evidence-tooling-self-test:provider-snapshot-hash-only"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "client-api-runner-missing-env-fails" "07-evidence-tooling-self-test:client-api-runner-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "client-api-runner-fixture-passes" "07-evidence-tooling-self-test:client-api-runner-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "browser-handoff-runner-session-start-opt-in-fails" "07-evidence-tooling-self-test:browser-handoff-runner-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "browser-handoff-runner-fixture-passes" "07-evidence-tooling-self-test:browser-handoff-runner-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "gateway-runner-opt-in-fails" "07-evidence-tooling-self-test:gateway-runner-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "gateway-runner-fixture-passes" "07-evidence-tooling-self-test:gateway-runner-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "admin-audit-runner-read-opt-in-fails" "07-evidence-tooling-self-test:admin-audit-runner-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "admin-audit-runner-fixture-passes" "07-evidence-tooling-self-test:admin-audit-runner-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "local-e2e-runner-fixture-passes" "07-evidence-tooling-self-test:local-e2e-runner-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-gap-helper" "07-evidence-tooling-self-test:release-gap-helper"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-inspection-missing-artifacts-fails" "07-evidence-tooling-self-test:package-artifact-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-inspection-fixture-passes" "07-evidence-tooling-self-test:package-artifact-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-inspection-embedded-fake-sk-token-fails" "07-evidence-tooling-self-test:package-artifact-token-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-inspection-fixed-commercial-policy-fails" "07-evidence-tooling-self-test:package-artifact-policy-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-missing-snapshots-fails" "07-evidence-tooling-self-test:compat-snapshot-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-fixture-passes" "07-evidence-tooling-self-test:compat-snapshot-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-only-evidence-verifier-fails" "07-evidence-tooling-self-test:compat-snapshot-only-verifier-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-camel-token-fields-fail" "07-evidence-tooling-self-test:compat-camel-token-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-manual-provider-content-change-fails" "07-evidence-tooling-self-test:compat-provider-content-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-token-fields-fail" "07-evidence-tooling-self-test:compat-snapshot-token-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-inspection-commercial-policy-fields-fail" "07-evidence-tooling-self-test:compat-snapshot-policy-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-snapshot-only-evidence-verifier-fails" "07-evidence-tooling-self-test:compat-snapshot-verifier-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-business-readiness-fails" "07-evidence-tooling-self-test:business-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-business-readiness-passes" "07-evidence-tooling-self-test:business-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "business-source-doc-unresolved-fails" "07-evidence-tooling-self-test:business-source-doc-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-readiness-summary-business-failed" "07-evidence-tooling-self-test:readiness-business-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-readiness-summary-fails" "07-evidence-tooling-self-test:generated-readiness-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-coverage-summary-fails" "07-evidence-tooling-self-test:generated-coverage-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-fixture-readiness-summary-fails" "07-evidence-tooling-self-test:fixture-readiness-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-fixture-readiness-summary-fixture-marker" "07-evidence-tooling-self-test:fixture-readiness-marker"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-fixture-readiness-summary-business-passed" "07-evidence-tooling-self-test:fixture-readiness-business-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "e2e-result-fail-fails" "07-evidence-tooling-self-test:e2e-result-fail"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "e2e-missing-level3-pass-fails" "07-evidence-tooling-self-test:e2e-level3-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-result-fail-fails" "07-evidence-tooling-self-test:package-result-fail"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-platform-manager-missing-codex-evidence-fails" "07-evidence-tooling-self-test:package-platform-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-metadata-result-fail-fails" "07-evidence-tooling-self-test:package-metadata-result-fail"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "package-artifact-coverage-missing-fails" "07-evidence-tooling-self-test:package-artifact-coverage-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-result-fail-fails" "07-evidence-tooling-self-test:compatibility-result-fail"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-context-result-fail-fails" "07-evidence-tooling-self-test:compatibility-context-result-fail"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-missing-manual-provider-fails" "07-evidence-tooling-self-test:compatibility-manual-provider-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "compatibility-runtime-boundary-evidence-missing-fails" "07-evidence-tooling-self-test:compat-runtime-boundary-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "business-result-fail-fails" "07-evidence-tooling-self-test:business-result-fail"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "readiness-summary-with-mismatched-coverage-input-fails" "07-evidence-tooling-self-test:readiness-coverage-input-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "readiness-summary-mismatched-coverage-input-marker" "07-evidence-tooling-self-test:readiness-coverage-input-marker"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-coverage-summary-fails" "07-evidence-tooling-self-test:module-j-coverage-summary-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-mismatched-summary-paths-fails" "07-evidence-tooling-self-test:module-j-summary-path-consistency"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-mismatched-evidence-inputs-fails" "07-evidence-tooling-self-test:module-j-evidence-input-consistency"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-level3-pass-fails" "07-evidence-tooling-self-test:module-j-level3-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-missing-readiness-coverage-fails" "07-evidence-tooling-self-test:module-j-readiness-coverage-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-readiness-not-allowed-fails" "07-evidence-tooling-self-test:module-j-readiness-allow-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-nongenerated-readiness-summary-fails" "07-evidence-tooling-self-test:module-j-readiness-generated-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-readiness-marker-despite-go-posture-fails" "07-evidence-tooling-self-test:module-j-readiness-marker-boundary"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-readiness-level3-failed-fails" "07-evidence-tooling-self-test:module-j-readiness-level3-boundary"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-mismatched-readiness-coverage-path-fails" "07-evidence-tooling-self-test:module-j-readiness-coverage-path"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-no-go-readiness-summary-fails" "07-evidence-tooling-self-test:module-j-readiness-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-missing-business-readiness-summary-fails" "07-evidence-tooling-self-test:module-j-business-readiness-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-business-evidence-hygiene-fails" "07-evidence-tooling-self-test:module-j-business-evidence-hygiene-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-readiness-summary-fails" "07-evidence-tooling-self-test:module-j-readiness-summary-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-module-inputs-fails" "07-evidence-tooling-self-test:module-j-module-inputs-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-unapproved-contract-drift-fails" "07-evidence-tooling-self-test:module-j-contract-drift-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-go-policy-signals-fails" "07-evidence-tooling-self-test:module-j-go-policy-signals-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-verification-availability-fails" "07-evidence-tooling-self-test:module-j-verification-availability-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-conflict-resolution-fails" "07-evidence-tooling-self-test:module-j-conflict-resolution-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-without-accepted-impact-fails" "07-evidence-tooling-self-test:module-j-accepted-impact-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-template-has-no-pending-field-labels" "07-evidence-tooling-self-test:module-j-template-no-pending-fields"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-camelcase-token-field-fails" "07-evidence-tooling-self-test:module-j-camel-token-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-env-secret-field-fails" "07-evidence-tooling-self-test:module-j-env-secret-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-go-policy-keywords-outside-failure-paths-fails" "07-evidence-tooling-self-test:module-j-go-policy-field-scoped"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "module-j-report-with-go-candidate-readiness-summary-passes" "07-evidence-tooling-self-test:module-j-readiness-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "generated-release-handoff-fails" "07-evidence-tooling-self-test:handoff-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-index-run-stamp-mismatch-fails" "07-evidence-tooling-self-test:handoff-run-stamp-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-misleading-index-path-fields-fails" "07-evidence-tooling-self-test:handoff-index-path-field-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-mismatched-readiness-allow-fails" "07-evidence-tooling-self-test:handoff-readiness-allow-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-mismatched-readiness-docs-result-fails" "07-evidence-tooling-self-test:handoff-readiness-docs-result-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-mismatched-readiness-business-result-fails" "07-evidence-tooling-self-test:handoff-readiness-business-result-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-without-final-verification-results-fails" "07-evidence-tooling-self-test:handoff-final-results-required"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "release-handoff-with-mismatched-coverage-summary-fails" "07-evidence-tooling-self-test:handoff-coverage-summary-negative"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-release-handoff-passes" "07-evidence-tooling-self-test:handoff-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-handoff-coverage-summary-passes" "07-evidence-tooling-self-test:handoff-coverage-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-aggregate-passes" "07-evidence-tooling-self-test:aggregate-positive"
Test-RepoText "codex-plus-dev-plan/tools/test-07-evidence-tooling.ps1" "valid-module-j-report-passes" "07-evidence-tooling-self-test:module-j-positive"
Test-RepoText "codex-plus-dev-plan/tools/new-07-evidence-run.ps1" "Result: pending" "07-e2e-generator:pending-result-markers"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "resultFiles" "07-e2e-verifier:result-marker-loop"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "result-pass" "07-e2e-verifier:result-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "final-recommendation-pass" "07-e2e-verifier:recommendation-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "level3:pass-recorded" "07-e2e-verifier:level3-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:browser-handoff-start-route" "07-e2e-verifier:browser-start-route"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:browser-handoff-complete-route" "07-e2e-verifier:browser-complete-route"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:browser-handoff-poll-route" "07-e2e-verifier:browser-poll-route"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:poll-token-not-in-authorize-url" "07-e2e-verifier:poll-token-url-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "client-api:verification-code-six-digit" "07-e2e-verifier:six-digit-code"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "gateway-policy:unauthorized-model" "07-e2e-verifier:gateway-coverage"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-evidence.ps1" "unfinished-placeholder.*pending" "07-e2e-verifier:pending-rejected"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "verify-07-evidence\.ps1" "07-release-evidence-verifier:e2e"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "verify-07-package-evidence\.ps1" "07-release-evidence-verifier:package"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "verify-07-compatibility-evidence\.ps1" "07-release-evidence-verifier:compatibility"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "DocsEvidenceDir" "07-release-evidence-verifier:docs-input"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "verify-07-docs-product-copy-evidence\.ps1" "07-release-evidence-verifier:docs"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "docs:verifier-passed" "07-release-evidence-verifier:docs-passed"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-evidence.ps1" "go/no-go recommendation" "07-release-evidence-verifier:boundary"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "Coverage status" "07-coverage-summary:status"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "Coverage Matrix" "07-coverage-summary:matrix"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "DocsEvidenceDir" "07-coverage-summary:docs-input"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "Docs product copy evidence folder" "07-coverage-summary:docs-folder"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" 'Scan-NonReleaseMarkers .* "docs"' "07-coverage-summary:docs-marker-scan"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "FailOnIncomplete" "07-coverage-summary:fail-on-incomplete"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "Nonrelease Markers" "07-coverage-summary:nonrelease-markers"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-coverage.ps1" "does not execute E2E" "07-coverage-summary:boundary"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Recommended Module J posture" "07-readiness-summary:posture"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "BusinessEvidenceDir" "07-readiness-summary:business-input"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "DocsEvidenceDir" "07-readiness-summary:docs-input"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "verify-07-docs-product-copy-evidence\.ps1" "07-readiness-summary:docs-verifier"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "CoverageSummaryFile" "07-readiness-summary:coverage-input"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Coverage summary verification" "07-readiness-summary:coverage-verification"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "coverage-e2e-input-mismatch" "07-readiness-summary:coverage-input-consistency"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "coverage-docs-input-mismatch" "07-readiness-summary:coverage-docs-input-consistency"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Business readiness verification" "07-readiness-summary:business-verification"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Docs product copy verification" "07-readiness-summary:docs-verification"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "E2E Level 3 result" "07-readiness-summary:e2e-level3"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "level3-pass-missing" "07-readiness-summary:level3-marker"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "AllowGoCandidate" "07-readiness-summary:explicit-go-candidate"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "FailOnNoGo" "07-readiness-summary:fail-on-no-go"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "Nonrelease Markers" "07-readiness-summary:nonrelease-markers"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "fixture" "07-readiness-summary:fixture-marker"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "not the final Module J report" "07-readiness-summary:not-final-report"
Test-RepoText "codex-plus-dev-plan/tools/new-07-business-readiness-evidence.ps1" "Business readiness result: pending" "07-business-readiness-generator:pending"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "Business readiness result" "07-business-readiness-verifier:result"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "business-result:pass" "07-business-readiness-verifier:result-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" 'gate:\$\{gate\}:pass' "07-business-readiness-verifier:gate-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "SourceDocsRoot" "07-business-readiness-verifier:source-docs-root"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "source-unresolved" "07-business-readiness-verifier:source-unresolved"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "PRODUCTION-ENVIRONMENT-MATRIX\.md" "07-business-readiness-verifier:production-source-scan"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "BUSINESS-CONFIG-DECISION-TABLE\.md" "07-business-readiness-verifier:business-source-scan"
Test-RepoText "codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1" "BusinessSourceDocsRoot" "07-readiness-summary:business-source-docs-root"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "BusinessSourceDocsRoot" "07-release-handoff-verifier:business-source-docs-root"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "production environment values" "07-business-readiness-verifier:production-values"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "cost control abuse spend caps emergency shutoff" "07-business-readiness-verifier:cost-abuse"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-business-readiness.ps1" "Paid-user support and entitlement correction" "07-business-readiness-verifier:support"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go with accepted risks" "07-module-j-report-verifier:recommendation-values"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "CoverageSummaryFile" "07-module-j-report-verifier:coverage-summary-param"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-coverage-summary" "07-module-j-report-verifier:coverage-summary-required"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-coverage-summary-complete" "07-module-j-report-verifier:coverage-summary-complete"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-no-missing-coverage" "07-module-j-report-verifier:coverage-missing-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-no-coverage-markers" "07-module-j-report-verifier:coverage-marker-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "release-evidence:coverage-summary-path" "07-module-j-report-verifier:coverage-summary-path"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "release-evidence:readiness-summary-path" "07-module-j-report-verifier:readiness-summary-path"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "e2e-path-coverage-consistency" "07-module-j-report-verifier:coverage-input-paths"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "business-path-readiness-consistency" "07-module-j-report-verifier:readiness-input-paths"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "release-evidence:aggregate-verifier" "07-module-j-report-verifier:aggregate-evidence"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-aggregate-pass" "07-module-j-report-verifier:go-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-module-reports-a-i" "07-module-j-report-verifier:module-reports"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-merge-order" "07-module-j-report-verifier:merge-order"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "ReadinessSummaryFile" "07-module-j-report-verifier:readiness-summary-param"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-summary" "07-module-j-report-verifier:readiness-summary-required"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-coverage-pass" "07-module-j-report-verifier:readiness-coverage-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-policy:level3-pass" "07-module-j-report-verifier:level3-go-policy"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:generated-status" "07-module-j-report-verifier:readiness-generated"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:e2e-level3-result" "07-module-j-report-verifier:readiness-level3-field"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-level3-pass" "07-module-j-report-verifier:readiness-level3-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-no-readiness-markers" "07-module-j-report-verifier:readiness-marker-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:coverage-status-consistency" "07-module-j-report-verifier:readiness-coverage-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:coverage-summary-path" "07-module-j-report-verifier:readiness-coverage-summary-path"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:allow-go-candidate" "07-module-j-report-verifier:readiness-allow-field"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-allow-go-candidate" "07-module-j-report-verifier:readiness-allow-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-readiness-go-candidate" "07-module-j-report-verifier:readiness-go-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "readiness-summary:aggregate-consistency" "07-module-j-report-verifier:readiness-aggregate-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-business-readiness-pass" "07-module-j-report-verifier:business-readiness-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" 'Get-ReportSectionText "Release evidence hygiene"' "07-module-j-report-verifier:release-evidence-section-scoped"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Business readiness folder" "07-module-j-report-verifier:business-evidence-field"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Docs product copy evidence folder" "07-module-j-report-verifier:docs-evidence-field"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Coverage summary status" "07-module-j-report-verifier:coverage-status-field"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "release-evidence:business-readiness-result" "07-module-j-report-verifier:business-evidence-result"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Docs product copy verification" "07-module-j-report-verifier:docs-evidence-result"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-docs-product-copy-pass" "07-module-j-report-verifier:docs-go-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "docs-path-coverage-consistency" "07-module-j-report-verifier:docs-coverage-input-paths"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "docs-path-readiness-consistency" "07-module-j-report-verifier:docs-readiness-input-paths"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-requires-business-readiness-report-pass" "07-module-j-report-verifier:business-report-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "accepted-risks-require-impact" "07-module-j-report-verifier:accepted-impact-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "no-unapproved-contract-drift" "07-module-j-report-verifier:contract-drift-boundary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" 'Get-ReportSectionText "Contract changes from original plan"' "07-module-j-report-verifier:contract-section-scoped"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "drift status" "07-module-j-report-verifier:contract-drift-status"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-policy:not-purchased-rejected" "07-module-j-report-verifier:go-policy-not-purchased"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "Test-FailurePathSignal" "07-module-j-report-verifier:failure-path-field-signal"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "go-policy:admin-config-bootstrap" "07-module-j-report-verifier:go-policy-admin-bootstrap"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "skipped or unavailable" "07-module-j-report-verifier:verification-availability"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "replacement narrower check" "07-module-j-report-verifier:verification-replacement"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "env-secret" "07-module-j-report-verifier:env-secret-scan"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "desktopAccessToken" "07-module-j-report-verifier:camel-token-scan"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" 'Get-ReportSectionText "Conflicts resolved"' "07-module-j-report-verifier:conflict-section-scoped"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-module-j-report.ps1" "rule used" "07-module-j-report-verifier:conflict-rule-used"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "verify-07-module-j-report\.ps1" "07-module-j-template:verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "ReadinessSummaryFile" "07-module-j-template:readiness-summary-file"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "CoverageSummaryFile" "07-module-j-template:coverage-summary-file"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "module reports:\s*FILL_ME" "07-module-j-template:module-reports"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "merge order:\s*FILL_ME" "07-module-j-template:merge-order"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "drift status:\s*FILL_ME" "07-module-j-template:contract-drift-status"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Business readiness folder:\s*FILL_ME" "07-module-j-template:business-folder"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Docs product copy evidence folder" "07-module-j-template:docs-folder"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Release coverage summary:\s*FILL_ME" "07-module-j-template:coverage-summary"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Release readiness summary:\s*FILL_ME" "07-module-j-template:readiness-summary"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Coverage summary status:\s*incomplete" "07-module-j-template:coverage-status"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Business readiness verification:\s*failed" "07-module-j-template:business-verification"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "Docs product copy verification" "07-module-j-template:docs-verification"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "level 3 result:\s*FILL_ME" "07-module-j-template:level3-result"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "nonrelease boundary removed for release docs" "07-module-j-template:nonrelease-docs-boundary"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "admin config bootstrap:\s*FILL_ME" "07-module-j-template:admin-config-bootstrap"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "skipped or unavailable:\s*FILL_ME" "07-module-j-template:verification-availability"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "impact:\s*FILL_ME" "07-module-j-template:risk-impact-field"
Test-RepoText "codex-plus-dev-plan/07-integration-release/reports/module-j-final-report-template.md" "rule used:\s*FILL_ME" "07-module-j-template:conflict-rule-used"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "ReleaseDir" "07-release-handoff-verifier:release-dir"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "verify-07-release-evidence\.ps1" "07-release-handoff-verifier:aggregate"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "verify-07-business-readiness\.ps1" "07-release-handoff-verifier:business-readiness"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "verify-07-docs-product-copy-evidence\.ps1" "07-release-handoff-verifier:docs"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "business:evidence-dir-exists" "07-release-handoff-verifier:business-dir"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "docs:evidence-dir-exists" "07-release-handoff-verifier:docs-dir"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "summarize-07-release-coverage\.ps1" "07-release-handoff-verifier:coverage-summary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "coverage-summary:complete" "07-release-handoff-verifier:coverage-complete"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "coverage-summary:e2e-input" "07-release-handoff-verifier:coverage-input"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "coverage-summary:missing-count-consistency" "07-release-handoff-verifier:coverage-count-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "summarize-07-release-readiness\.ps1" "07-release-handoff-verifier:readiness-summary"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:e2e-input" "07-release-handoff-verifier:readiness-input"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:coverage-summary-input" "07-release-handoff-verifier:readiness-coverage-summary-input"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:coverage-consistency" "07-release-handoff-verifier:readiness-coverage-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:business-consistency" "07-release-handoff-verifier:readiness-business-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:docs-result-consistency" "07-release-handoff-verifier:readiness-docs-result-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:business-result-consistency" "07-release-handoff-verifier:readiness-business-result-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "verify-07-module-j-report\.ps1" "07-release-handoff-verifier:module-j-report"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:final-status" "07-release-handoff-verifier:final-index"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "run-stamp-release-dir-consistency" "07-release-handoff-verifier:run-stamp-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:docs-path" "07-release-handoff-verifier:index-docs-path"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:docs-result-field" "07-release-handoff-verifier:index-docs-result"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:aggregate-result-field" "07-release-handoff-verifier:index-aggregate-result"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "coverage-summary:docs-input" "07-release-handoff-verifier:coverage-docs-input"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:docs-input" "07-release-handoff-verifier:readiness-docs-input"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:docs-consistency" "07-release-handoff-verifier:readiness-docs-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:stored-generated-status" "07-release-handoff-verifier:readiness-generated"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:allow-go-candidate-consistency" "07-release-handoff-verifier:readiness-allow-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:marker-section-consistency" "07-release-handoff-verifier:readiness-marker-consistency"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "readiness-summary:allow-go-candidate-switch-consistency" "07-release-handoff-verifier:readiness-allow-switch"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff-index:final-recommendation-consistency" "07-release-handoff-verifier:index-final-recommendation"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "module-j-final-report:recommendation-field" "07-release-handoff-verifier:report-recommendation"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-release-handoff.ps1" "handoff consistency only" "07-release-handoff-verifier:boundary"
foreach ($symbol in @("manual providers", "compatibility evidence pending", "rollback", "provider")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" $symbol "07-compatibility-checklist:$symbol"
}
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" "new-07-compatibility-evidence\.ps1" "07-compatibility-checklist:compat-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" "inspect-07-compatibility-snapshots\.ps1" "07-compatibility-checklist:snapshot-inspector"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" "verify-07-compatibility-evidence\.ps1" "07-compatibility-checklist:compat-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" "new-07-desktop-compatibility-harness\.ps1" "07-compatibility-checklist:desktop-harness"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/compatibility-migration-checklist.md" "capture-07-desktop-provider-snapshot\.ps1" "07-compatibility-checklist:provider-snapshot-capture"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "new-07-compatibility-evidence\.ps1" "07-compat-template:compat-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "inspect-07-compatibility-snapshots\.ps1" "07-compat-template:snapshot-inspector"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "verify-07-compatibility-evidence\.ps1" "07-compat-template:compat-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "new-07-desktop-compatibility-harness\.ps1" "07-compat-template:desktop-harness"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "capture-07-desktop-provider-snapshot\.ps1" "07-compat-template:provider-snapshot-capture"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/provider-settings-evidence-template.md" "-DocsEvidenceDir" "07-compat-template:aggregate-docs-evidence-dir"
Test-RepoText "codex-plus-dev-plan/07-integration-release/compatibility/rollback-notes.md" "verify-07-compatibility-evidence\.ps1" "07-compat-rollback:compat-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md" "new-07-compatibility-evidence\.ps1" "07-compat-task:compat-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md" "inspect-07-compatibility-snapshots\.ps1" "07-compat-task:snapshot-inspector"
Test-RepoText "codex-plus-dev-plan/07-integration-release/task-compatibility-and-migration.md" "verify-07-compatibility-evidence\.ps1" "07-compat-task:compat-verifier"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "compat-report:result-pass" "07-compat-verifier:result-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:result-pass" "07-compat-verifier:context-result-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:no-missing-inputs" "07-compat-verifier:no-missing-inputs"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:pre-upgrade-parsed" "07-compat-verifier:pre-upgrade-parsed"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:post-upgrade-parsed" "07-compat-verifier:post-upgrade-parsed"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:logout-parsed" "07-compat-verifier:logout-parsed"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:rollback-parsed" "07-compat-verifier:rollback-parsed"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "context:no-parse-failures" "07-compat-verifier:no-parse-failures"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "pre-upgrade:manual-provider-count" "07-compat-verifier:manual-provider-count"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:managed-cloud" "07-compat-verifier:managed-cloud"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:managed-runtime-only-config" "07-compat-verifier:managed-runtime-only-config"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:advanced-configuration-reachable" "07-compat-verifier:advanced-configuration-reachable"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:no-commercial-policy" "07-compat-verifier:no-commercial-policy"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:no-missing-manual" "07-compat-verifier:no-missing-manual-after-upgrade"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "post-upgrade:manual-content-unchanged" "07-compat-verifier:manual-content-unchanged-after-upgrade"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "manual-switch:runtime-request-pass" "07-compat-verifier:runtime-manual-provider-request"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "manual-switch:default-managed-entry-point" "07-compat-verifier:default-managed-entry-point"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "Runtime compatibility result" "07-compat-verifier:runtime-result-required"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "logout:token-scan-clear" "07-compat-verifier:logout-token-scan-clear"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "provider-sync:log-secret-scan-clear" "07-compat-verifier:provider-sync-secret-scan"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "rollback:no-missing-manual" "07-compat-verifier:rollback-no-missing-manual"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "rollback:failed-provider-write-recovery" "07-compat-verifier:failed-provider-write-recovery"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1" "compat-report:snapshot-inspector-command" "07-compat-verifier:snapshot-inspector-command"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "FixtureMode" "07-compat-snapshot-inspector:fixture-mode"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "PreUpgradeSnapshot" "07-compat-snapshot-inspector:pre-snapshot"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "PostUpgradeSnapshot" "07-compat-snapshot-inspector:post-snapshot"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "LogoutSnapshot" "07-compat-snapshot-inspector:logout-snapshot"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "RollbackSnapshot" "07-compat-snapshot-inspector:rollback-snapshot"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "Manual providers preserved after upgrade" "07-compat-snapshot-inspector:manual-preserved"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "relayProfiles" "07-compat-snapshot-inspector:relay-profiles"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "Manual provider content unchanged after upgrade" "07-compat-snapshot-inspector:manual-content-unchanged"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "nonprinted base URL/API key hashes" "07-compat-snapshot-inspector:manual-content-hashes"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "accessToken" "07-compat-snapshot-inspector:camel-token-scan"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "desktopAccessToken" "07-compat-snapshot-inspector:desktop-token-scan"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "KEY\|TOKEN\|SECRET\|PASSWORD" "07-compat-snapshot-inspector:env-secret-redaction"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "Codex\+\+ Cloud" "07-compat-snapshot-inspector:managed-cloud"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "No plan, price, multiplier, entitlement, or usage policy" "07-compat-snapshot-inspector:no-commercial-policy"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "allSnapshotsHaveNoCommercialPolicy" "07-compat-snapshot-inspector:all-snapshots-policy-scan"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "allSnapshotsClearTokenFields" "07-compat-snapshot-inspector:all-snapshots-token-scan"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1" "token values intentionally not printed" "07-compat-snapshot-inspector:redaction"
foreach ($symbol in @("Windows", "macOS", "package evidence pending", "shared Key")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/package-install/package-install-checklist.md" $symbol "07-package-checklist:$symbol"
}
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "FixtureMode" "07-package-artifact-inspector:fixture-mode"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "windows-x64-setup" "07-package-artifact-inspector:windows-coverage"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "macos-x64-dmg" "07-package-artifact-inspector:macos-x64-coverage"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "macos-arm64-dmg" "07-package-artifact-inspector:macos-arm64-coverage"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "values intentionally not printed" "07-package-artifact-inspector:redaction"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "Package install does not overwrite existing manual provider configuration" "07-package-artifact-inspector:manual-provider-boundary"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "plan_id" "07-package-artifact-inspector:plan-id-scan"
Test-RepoText "codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1" "quota" "07-package-artifact-inspector:quota-scan"
Test-RepoText "codex-plus-dev-plan/07-integration-release/package-install/package-install-checklist.md" "verify-07-package-evidence\.ps1" "07-package-checklist:package-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/package-install/package-install-checklist.md" "inspect-07-package-artifacts\.ps1" "07-package-checklist:artifact-inspector"
Test-RepoText "codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md" "new-07-package-evidence\.ps1" "07-package-template:package-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md" "inspect-07-package-artifacts\.ps1" "07-package-template:artifact-inspector"
Test-RepoText "codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md" "verify-07-package-evidence\.ps1" "07-package-template:package-verifier"
Test-RepoText "codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md" "-DocsEvidenceDir" "07-package-template:aggregate-docs-evidence-dir"
Test-RepoText "codex-plus-dev-plan/07-integration-release/task-package-install-check.md" "new-07-package-evidence\.ps1" "07-package-task:package-generator"
Test-RepoText "codex-plus-dev-plan/07-integration-release/task-package-install-check.md" "inspect-07-package-artifacts\.ps1" "07-package-task:artifact-inspector"
Test-RepoText "codex-plus-dev-plan/07-integration-release/task-package-install-check.md" "verify-07-package-evidence\.ps1" "07-package-task:package-verifier"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "package-report:result-pass" "07-package-verifier:result-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "metadata:result-pass" "07-package-verifier:metadata-result-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "metadata:windows-coverage-present" "07-package-verifier:metadata-coverage"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "metadata:macos-x64-coverage-present" "07-package-verifier:metadata-macos-x64-coverage"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "metadata:macos-arm64-coverage-present" "07-package-verifier:metadata-macos-arm64-coverage"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "windows:desktop-main-shortcut" "07-package-verifier:windows-desktop-shortcut"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "windows:apps-and-features" "07-package-verifier:windows-apps-and-features"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "windows:manager-advanced-configuration" "07-package-verifier:windows-manager-advanced-configuration"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "windows:missing-codex-first-run" "07-package-verifier:windows-missing-codex-first-run"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "macos-x64:hidden-dock" "07-package-verifier:macos-x64-hidden-dock"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "macos-arm64:missing-codex-first-run" "07-package-verifier:macos-arm64-missing-codex-first-run"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:result-pass" "07-package-verifier:inspection-result-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:inspector-command" "07-package-verifier:inspector-command"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:no-shared-key" "07-package-verifier:no-shared-key"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:no-user-credentials" "07-package-verifier:no-user-credentials"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:no-fixed-policy" "07-package-verifier:no-fixed-policy"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:installer-script-pass" "07-package-verifier:installer-script-pass"
Test-RepoText "codex-plus-dev-plan/tools/verify-07-package-evidence.ps1" "inspection:no-scanner-findings" "07-package-verifier:no-scanner-findings"
foreach ($symbol in @("backend-configured", "Control Plane", "Rollback", "pending")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/docs/docs-sync-record.md" $symbol "07-docs-sync:$symbol"
}
foreach ($symbol in @("static sync passed", "local Chromium visual evidence passed", "in-app browser HTTP preview visual evidence passed", "direct local-file navigation blocked", "URL policy", "in-app-browser-policy-boundary", "in-app-browser-http-desktop\.png", "in-app-browser-http-mobile\.png", "Control Plane", "gpt-5")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/docs/html-sync-evidence.md" $symbol "07-html-sync:$symbol"
}
foreach ($symbol in @("Browser runtime connection: succeeded", "HTTP visual evidence passed", "Preview URL", "127\.0\.0\.1:8099", "in-app-browser-http-desktop\.png", "in-app-browser-http-mobile\.png", "Direct .*file://.* rendering")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/docs/html-visual-evidence/in-app-browser-policy-boundary.md" $symbol "07-in-app-browser-boundary:$symbol"
}

foreach ($symbol in @("local verification passed", "release evidence pending", "release recommendation no-go", "go test ./\.\.\.", "npm run typecheck", "npm run build", "npm run vite:build", "cargo fmt --check -p codex-plus-core", "quotaProgress", "Local Chromium desktop/mobile screenshot evidence passed", "URL policy", "in-app-browser-policy-boundary", "verify-07-e2e-readiness\.ps1", "new-07-e2e-env-template\.ps1", "CODEXPLUS_07_E2E", "test-environment readiness", "run-client-api-checks\.ps1", "run-browser-handoff-checks\.ps1", "AllowSessionStart", "AllowBrowserComplete", "run-gateway-policy-checks\.ps1", "AllowGatewayRequests", "run-admin-audit-checks\.ps1", "AllowAdminAuditReads", "run-local-e2e\.ps1", "start-local-dev-compose\.ps1", "start-local-source-service\.ps1", "docker-compose.dev\.yml", "\.env\.codexplus-local\.example", "\.codexplus-local", "sub2api-codexplus-local", "127\.0\.0\.1:8081", "client API subset", "verify-07-rust-preflight\.ps1", "rust-toolchain", "9\.56GB", "test-07-evidence-tooling\.ps1", "new-07-release-evidence-set\.ps1", "new-07-evidence-run\.ps1", "new-07-package-evidence\.ps1", "inspect-07-package-artifacts\.ps1", "verify-07-package-evidence\.ps1", "new-07-compatibility-evidence\.ps1", "inspect-07-compatibility-snapshots\.ps1", "verify-07-compatibility-evidence\.ps1", "report-07-release-gaps\.ps1", "new-07-business-readiness-evidence\.ps1", "verify-07-business-readiness\.ps1", "verify-07-release-evidence\.ps1", "summarize-07-release-coverage\.ps1", "summarize-07-release-readiness\.ps1", "verify-07-release-handoff\.ps1")) {
    Test-RepoText "codex-plus-dev-plan/07-integration-release/release-local-verification.md" $symbol "07-local-verification:$symbol"
}

foreach ($symbol in @("--quota-progress", "quotaProgress", "Control Plane", "Data Plane", "Client Runtime", "Platform Ops")) {
    Test-RepoText "codex-plus-product-spec.html" $symbol "07-html-product-spec:$symbol"
}

foreach ($pattern in @("--quota:65%", "style\.setProperty\(`"--quota`",\s*data\.quota\)", "gpt-5-mini", "sk-user-managed-token", "remaining_percent", "today_tokens", "82%")) {
    Test-RepoTextNot "codex-plus-product-spec.html" $pattern "07-html-product-spec-no-residue:$pattern"
}

$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Result -eq "FAIL" })
if ($failed.Count -gt 0) {
    Write-Host ""
    Write-Host "07 static audit failed: $($failed.Count) failing check(s)." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "07 static audit passed."
exit 0
