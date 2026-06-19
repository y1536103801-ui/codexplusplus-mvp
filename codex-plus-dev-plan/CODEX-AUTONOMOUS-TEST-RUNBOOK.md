# Codex Autonomous Test Runbook

本文档是给 Codex 执行的完整测试操作手册。目标是让 Codex 在具备本地命令行、浏览器控制、电脑控制、测试账号和测试环境权限后，尽可能自主完成从本地 MVP 到生产上线前的测试。

本文件不替代 [QA-TESTING-ACCEPTANCE-PLAN.md](QA-TESTING-ACCEPTANCE-PLAN.md)，而是把其中的测试流程翻译成 Codex 可执行步骤、命令、证据要求和阻断规则。

## Execution Principle

Codex 可以自主完成：

- 代码静态检查、单元测试、集成测试。
- 启动本地后端、前端、桌面端。
- 控制浏览器测试后台和用户页面。
- 控制桌面应用测试 Codex++ Manager。
- 检查日志、数据库、Redis、HTTP 响应、构建产物。
- 执行 staging 和 production smoke test。
- 生成测试报告、缺陷清单、上线 go/no-go 建议。

Codex 不能凭空完成：

- 没有账号密码的登录。
- 没有沙盒凭证的支付回调。
- 没有生产授权的真实支付、小额扣款、服务器改配置。
- 需要人类二次验证、短信、扫码、硬件 Key 或 CAPTCHA 的动作。
- 法务、商业、价格策略、真实用户通知等非技术决策。

遇到这些情况，Codex 必须停止并向用户索要测试凭证、授权或人工确认。

## Required Inputs Before Autonomous Testing

Codex 开始完整测试前，需要用户或 coordinator 提供：

### Workspace

- `sub2api-main` 源码目录。
- `CodexPlusPlus-main` 源码目录。
- `codex-plus-dev-plan` 文档目录。

### Local runtime

- Go toolchain matching `sub2api-main/backend/go.mod`.
- Node.js and pnpm/npm.
- Rust toolchain and Cargo.
- Tauri build prerequisites.
- PostgreSQL.
- Redis.
- Browser available for admin/frontend tests.
- Desktop app automation available for Codex++ Manager tests.

### Test credentials

- Admin test account.
- Active user account.
- Not-purchased user account.
- Expired user account.
- Low-balance user account.
- Device-revoked user account.
- Model-denied user account.
- Payment sandbox account if payment tests are in scope.

### Environment values

Use a secure handoff mechanism. Do not paste real production secrets into public docs.

Required values may include:

- Backend base URL.
- Admin frontend URL.
- Codex++ Manager build path.
- PostgreSQL DSN or local env file path.
- Redis URL.
- JWT/session secret for test environment.
- Upstream model provider test key.
- Payment sandbox callback secret.
- Test domain and HTTPS URL for staging.

## Evidence Directory

For each test run, Codex must create an evidence folder:

```text
test-runs/
  YYYY-MM-DD-HHMM-codexplus/
    00-run-metadata.md
    01-environment.md
    02-command-results.md
    03-api-results/
    04-browser-screenshots/
    05-desktop-screenshots/
    06-logs/
    07-db-checks/
    08-payment-checks/
    09-defects.md
    10-go-no-go.md
    11-business-readiness.md
    12-final-report.md
```

Evidence requirements:

- Every command must record command, cwd, result, and important output.
- Every browser test must record URL, account used, expected result, actual result, and screenshot when useful.
- Every desktop test must record app version/build, scenario, expected result, actual result, screenshot when useful.
- Every failed test must create a defect entry.
- No evidence file may contain full API Key, JWT, Authorization header, payment secret, database password, or upstream credential.

## Severity and Stop Rules

Use the same severity as [QA-TESTING-ACCEPTANCE-PLAN.md](QA-TESTING-ACCEPTANCE-PLAN.md):

- P0: must stop release. Examples: secret leak, paid user cannot use, unpaid user can use, payment duplicates credit, data loss.
- P1: must fix or define a rollback/degrade plan. Examples: expired user not blocked, gateway enforcement missing, admin cannot recover user entitlement.
- P2: can defer with owner and risk acceptance. Examples: unclear message, non-critical UI bug.
- P3: cosmetic or documentation issue.

Codex must stop autonomous release testing and report immediately when:

- It detects secret leakage.
- It sees production data unexpectedly in staging.
- A destructive operation would affect real users.
- Payment tests require real money and no explicit authorization exists.
- A P0 appears.
- A P1 appears and no safe workaround exists.

## Autonomous Test Phases

```text
0. Readiness and environment discovery
1. Static and contract checks
2. Backend automated tests
3. Admin frontend automated tests
4. Desktop build and runtime tests
5. Local/MVP E2E tests
6. Browser-driven admin and user tests
7. Desktop computer-use tests
8. Staging QA regression
9. Business readiness gate checks
10. Production smoke test
11. Post-launch monitoring
12. Final report
```

## Phase 0: Readiness and Environment Discovery

Purpose:

- Confirm the test target, environment and credentials before running anything risky.

Codex steps:

1. Read:
   - `README.md`
   - `MVP-IMPLEMENTATION-PLAN.md`
   - `PRODUCTION-LAUNCH-PLAN.md`
   - `PRODUCTION-ENVIRONMENT-MATRIX.md`
   - `BUSINESS-CONFIG-DECISION-TABLE.md`
   - `SERVER-SIZING-AND-SCALING-GUIDE.md`
   - `DEPLOYMENT-AUTOMATION-RUNBOOK.md`
   - `QA-TESTING-ACCEPTANCE-PLAN.md`
   - `CONTRACT-GATE.md`
   - `INTEGRATION-VERIFICATION-CHECKLIST.md`
   - `SECURITY-REVIEW-PLAN.md`
   - `COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md`
   - `OBSERVABILITY-SLO-ALERTING-PLAN.md`
   - `COST-CONTROL-AND-ABUSE-RUNBOOK.md`
   - `SUPPORT-OPERATIONS-RUNBOOK.md`
   - this runbook.
2. Locate source repos:
   - `sub2api-main`
   - `CodexPlusPlus-main`
3. Check deployment templates:
   - `deployment/templates/env.production.template`
   - `deployment/templates/docker-compose.prod.template.yml`
   - `deployment/templates/Caddyfile.template`
   - `deployment/scripts/deploy.sh`
   - `deployment/scripts/rollback.sh`
   - `deployment/scripts/backup-postgres.sh`
   - `deployment/scripts/restore-postgres.sh`
   - `deployment/scripts/healthcheck.sh`
4. Check whether each repo is a git repo.
5. Record branch and dirty state.
6. Record tool versions:
   - `go version`
   - `node --version`
   - `pnpm --version`
   - `npm --version`
   - `cargo --version`
   - `rustc --version`
7. Check PostgreSQL and Redis availability.
8. Check whether required test credentials are available.
9. Check whether production environment/business/server sizing/deployment decisions are complete enough for the selected test scope.
10. Create evidence directory.

Suggested commands:

```powershell
git status --short
git branch --show-current
go version
node --version
pnpm --version
npm --version
cargo --version
rustc --version
```

Pass criteria:

- Repos exist.
- Toolchain exists.
- Test environment target is known.
- Missing credentials are listed before test execution.

## Phase 1: Static and Contract Checks

Purpose:

- Confirm implementation still matches contract and docs before functional testing.

Codex steps:

1. Verify contract files exist:
   - OpenAPI.
   - Config schemas.
   - Mock fixtures.
   - Error/status table.
   - Event schema.
2. Verify actual backend route names match contract:
   - `GET /api/v1/client/bootstrap`
   - `GET /api/v1/client/usage`
   - `POST /api/v1/client/devices`
   - `POST /api/v1/client/redeem`
3. Verify desktop/client code does not reference fields missing from mocks.
4. Search for suspicious hardcoded values:
   - hardcoded prices.
   - hardcoded plan names used as business truth.
   - hardcoded model allowlists in desktop UI.
   - full API Key logs.
5. Run secret scan if tool exists.

Suggested searches:

```powershell
rg -n "sk-|Authorization|Bearer |api_key|jwt|password|secret|private_key" sub2api-main CodexPlusPlus-main
rg -n "price|plan|quota|rpm|tpm|model" CodexPlusPlus-main/apps CodexPlusPlus-main/crates
```

Pass criteria:

- No contract drift.
- No obvious secret leak.
- Client does not own backend business rules.

## Phase 2: Backend Automated Tests

Purpose:

- Validate Sub2API backend logic before UI/E2E testing.

Codex steps:

1. Install/confirm backend dependencies if required.
2. Run unit/integration tests.
3. Run targeted Codex++ tests if they exist.
4. Record failures with package, test name, error summary and likely owner.

Commands:

```powershell
cd sub2api-main/backend
go test ./...
```

If Makefile is available:

```powershell
cd sub2api-main
make test-backend
```

Required targeted backend coverage:

- Config validation.
- Bootstrap requires JWT.
- Bootstrap active user success.
- Bootstrap not purchased.
- Bootstrap expired.
- Bootstrap low balance.
- Device upsert idempotency.
- Device user isolation.
- Revoked device status.
- Key reuse on repeated bootstrap.
- API Key/JWT log redaction.
- Gateway unauthorized model rejection.
- Gateway expired rejection.
- Gateway insufficient balance rejection.
- Gateway revoked device rejection.
- Valid gateway request still forwards.
- Usage event recorded.

Pass criteria:

- Full backend tests pass, or failures are unrelated and documented.
- All Codex++ targeted tests pass.
- No P0/P1 backend failure remains.

## Phase 3: Admin Frontend Automated Tests

Purpose:

- Validate admin frontend build, type safety and critical UI logic.

Commands:

```powershell
cd sub2api-main/frontend
pnpm run lint:check
pnpm run typecheck
pnpm run test:run
```

Required targeted frontend coverage:

- Admin can view Codex++ config.
- Admin validation errors are displayed.
- Admin can edit plan/model/usage/feature flags.
- User entitlement view hides full secrets.
- Device status is visible.
- Loading, empty, error and permission-denied states exist.

Pass criteria:

- Lint/typecheck/tests pass.
- No UI path exposes secrets.
- No admin mutation relies only on frontend validation.

## Phase 4: Desktop Build and Runtime Tests

Purpose:

- Validate Codex++ Manager and Rust core before interactive desktop testing.

Commands:

```powershell
cd CodexPlusPlus-main
cargo test --workspace
```

```powershell
cd CodexPlusPlus-main/apps/codex-plus-manager
npm run check
npm run build
```

If Tauri dev/build is needed:

```powershell
cd CodexPlusPlus-main/apps/codex-plus-manager
npm run dev
```

or:

```powershell
cd CodexPlusPlus-main/apps/codex-plus-manager
npm run build
```

Required targeted desktop coverage:

- Bootstrap client parses all expected states.
- Session/cache stores minimum data.
- Provider writer creates or updates only `Codex++ Cloud`.
- Existing manual providers survive.
- Local logs redact secrets.
- Local errors map to user-safe statuses.

Pass criteria:

- Rust and manager checks pass.
- Provider writer safety tests pass.
- No secret in desktop build/config logs.

## Phase 5: Local/MVP API E2E

Purpose:

- Test the core product chain through HTTP before browser/desktop automation.

Prerequisites:

- Backend running.
- PostgreSQL/Redis running.
- Test users seeded.
- Test upstream provider configured.

Codex steps:

1. Start or confirm backend.
2. Authenticate as `user_active`.
3. Call device API.
4. Call bootstrap.
5. Validate response shape.
6. Call usage.
7. Send a gateway model request using the returned provider settings.
8. Check usage/log/event.
9. Repeat for failure users.

Suggested API checks:

```powershell
# Pseudocode. Replace URLs and tokens with test environment values.
$base = "http://127.0.0.1:PORT"
$token = "<TEST_USER_JWT>"
Invoke-RestMethod -Headers @{ Authorization = "Bearer $token" } -Uri "$base/api/v1/client/bootstrap"
```

Expected happy-path assertions:

- `service.status` is `available`.
- `provider.base_url` points to Sub2API gateway.
- `provider.api_key` is a user-side gateway key and is redacted in logs, reports and screenshots.
- `models` contains default model.
- `usage` exists.
- `config_version` and `snapshot_version` exist.
- No upstream real credential appears.

Failure-path assertions:

- `user_not_purchased` returns `not_purchased`.
- `user_expired` returns `expired`.
- `user_low_balance` returns low/insufficient balance state.
- `user_device_revoked` returns `device_revoked`.
- Unauthorized model request is rejected by gateway.

Pass criteria:

- Happy path succeeds.
- Core failure paths are enforced server-side.
- Logs/events are present and redacted.

## Phase 6: Browser-Driven Admin Tests

Purpose:

- Use browser automation to test admin workflows like a human operator.

Tool:

- Codex in-app Browser or Chrome automation, depending on session availability and login state.

Codex browser steps:

1. Open admin frontend URL.
2. Log in as `admin_test`.
3. Navigate to Codex++ plan/config area.
4. Create or edit test plan.
5. Create or edit model catalog entry.
6. Change default model.
7. Change usage policy.
8. Toggle feature flags.
9. Open user entitlement view.
10. Open device status view.
11. Revoke and restore test device if safe.
12. Verify validation errors by entering invalid config.
13. Capture screenshots for major pages.

Required admin scenarios:

- Valid plan save.
- Invalid negative price rejected.
- Empty model group rejected.
- Disabled default model rejected.
- Feature flag change visible in bootstrap.
- User entitlement summary visible.
- Full API Key not visible.
- Ordinary user cannot access admin pages.

Pass criteria:

- Admin actions work.
- Invalid actions are rejected by backend.
- Screenshots and notes saved.
- No secret exposure.

## Phase 7: Desktop Computer-Use Tests

Purpose:

- Use desktop control to test Codex++ Manager as an installed/running application.

Tool:

- Codex Computer Use for Windows app control.
- Browser only for web-based manager preview if desktop packaging is unavailable.

Codex desktop steps:

1. Launch Codex++ Manager.
2. Confirm app opens without crash.
3. Log in as `user_active`.
4. Confirm service dashboard shows available state.
5. Confirm plan, usage and model list render.
6. Trigger bootstrap refresh.
7. Trigger provider write/sync.
8. Check `Codex++ Cloud` provider exists.
9. Confirm old manual providers still exist.
10. Click launch Codex.
11. Send a low-cost model request.
12. Confirm request succeeds.
13. Log out and confirm manual providers remain.
14. Repeat key failure users or mocked states.

Required desktop scenarios:

- Active user happy path.
- Not purchased state.
- Expired state.
- Low balance state.
- Device revoked state.
- Gateway unhealthy state.
- Local Codex missing or install assistant state.
- Local provider write failure state.
- Diagnostic export redaction.

Pass criteria:

- App can be used by a normal user without manual API configuration.
- Provider write is safe.
- User-facing messages are actionable.
- Screenshots/evidence saved.

## Phase 8: Staging QA Regression

Purpose:

- Prove the system works in a production-like environment before production.

Prerequisites:

- Staging deployed with production-like topology.
- Staging domain and HTTPS.
- Staging PostgreSQL/Redis.
- Staging payment sandbox or approved low-value payment.
- Staging Codex++ Manager build points to staging API.
- Monitoring/logging enabled.

Codex steps:

1. Verify staging health endpoints.
2. Verify HTTPS certificate.
3. Verify CORS from manager origin.
4. Run backend smoke tests against staging.
5. Run admin browser tests.
6. Run user desktop tests.
7. Run payment sandbox callback.
8. Replay payment callback to verify idempotency.
9. Run backup and restore to temporary database.
10. Run rollback drill in staging.
11. Run log redaction scan.
12. Run secret scan on build artifacts.
13. Generate staging QA report.

Staging required cases:

- Happy path.
- Not purchased.
- Expired.
- Low balance.
- Device revoked.
- Model denied.
- Payment success.
- Duplicate payment callback.
- Admin config change reflected in bootstrap.
- Backup restore.
- Rollback.
- Monitoring alert test if safe.

Pass criteria:

- Staging full regression passes.
- P0/P1 closed.
- P2/P3 accepted or assigned.
- Go/no-go recommendation prepared.

## Phase 9: Business Readiness Gate Checks

Purpose:

- Confirm the launch is ready as a paid product operation, not only as a working technical deployment.

Prerequisites:

- Staging QA regression completed.
- Security, compliance, observability, cost/abuse and support documents exist.
- A human owner is available for business/legal decisions Codex cannot make.

Codex steps:

1. Read [PRODUCTION-ENVIRONMENT-MATRIX.md](PRODUCTION-ENVIRONMENT-MATRIX.md) and list unresolved required production values.
2. Read [BUSINESS-CONFIG-DECISION-TABLE.md](BUSINESS-CONFIG-DECISION-TABLE.md) and list unresolved package/model/quota/payment/cost decisions.
3. Read [SERVER-SIZING-AND-SCALING-GUIDE.md](SERVER-SIZING-AND-SCALING-GUIDE.md) and verify the selected server shape matches the selected launch stage.
4. Read [DEPLOYMENT-AUTOMATION-RUNBOOK.md](DEPLOYMENT-AUTOMATION-RUNBOOK.md) and verify deployment, backup, rollback and healthcheck paths are defined.
5. Read [SECURITY-REVIEW-PLAN.md](SECURITY-REVIEW-PLAN.md) and list all open P0/P1/P2 security items.
6. Read [COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md](COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md) and record unresolved legal/privacy/payment-provider decisions.
7. Read [OBSERVABILITY-SLO-ALERTING-PLAN.md](OBSERVABILITY-SLO-ALERTING-PLAN.md) and verify dashboards, SLO targets and alert routing are defined.
8. Read [COST-CONTROL-AND-ABUSE-RUNBOOK.md](COST-CONTROL-AND-ABUSE-RUNBOOK.md) and verify spend caps, quota enforcement, abuse signals and emergency shutoff exist.
9. Read [SUPPORT-OPERATIONS-RUNBOOK.md](SUPPORT-OPERATIONS-RUNBOOK.md) and verify support channels, ticket severities, refund/compensation flow and admin recovery procedures exist.
10. Create `11-business-readiness.md` in the evidence directory.
11. Mark each gate as pass, fail or deferred.
12. For every deferred item, record owner, accepted risk, mitigation and latest decision date.
13. Produce final production go/no-go recommendation.

Pass criteria:

- No open P0/P1 across security, compliance, observability, cost/abuse and support.
- Production environment, business config, server sizing and deployment automation have no unresolved required items.
- Deferred P2/P3 items have owners and risk acceptance.
- Paid-user support, refund/compensation and entitlement correction paths are documented.
- Emergency stop path exists for runaway cost, leaked credential, gateway bypass and payment callback error.
- Codex has clearly listed any remaining questions that require human business/legal decisions.

No-go examples:

- Open security P0/P1.
- Required production environment value is missing.
- First-launch plan/model/quota/payment/cost decision is missing.
- Selected server spec does not match the selected launch stage.
- Deployment, backup or rollback path is undefined.
- Privacy policy, terms, refund policy or provider terms are missing for the target launch market.
- No usable dashboard or alert path for production incidents.
- No cost cap or emergency stop for upstream model spend.
- No support process for paid users who cannot use the product.

## Phase 10: Production Smoke Test

Purpose:

- Confirm production deployment is healthy after release.

Prerequisites:

- Go decision approved.
- Production release deployed.
- Production test admin and test user available.
- Production smoke test authorized.

Codex steps:

1. Open production health URL.
2. Open production admin URL.
3. Log in as production test admin.
4. Confirm plan/model/feature flag config visible.
5. Open production Codex++ Manager build.
6. Log in as production test user.
7. Call bootstrap.
8. Write/sync `Codex++ Cloud`.
9. Launch Codex.
10. Send one low-cost request.
11. Confirm usage/log/event.
12. Confirm dashboards and alerts are normal.
13. Record evidence.

Do not do in production smoke:

- destructive device deletion on real users.
- mass entitlement changes.
- refund tests on real users.
- load/stress tests.
- database failure drills.

Pass criteria:

- Production happy path passes.
- No P0/P1 alerts.
- Logs redacted.
- Monitoring normal.

## Phase 11: Post-Launch Monitoring

Purpose:

- Watch real production behavior after release.

Codex steps:

1. Collect metrics every 2-4 hours during the observation window.
2. Review errors by category.
3. Review payment callback failures.
4. Review gateway rejection spikes.
5. Review upstream provider errors and cost.
6. Review user support issues.
7. Compare usage and entitlement data for sample users.
8. Produce post-launch report.

Observation window:

- Minimum: 24 hours.
- Recommended: 72 hours for paid launch.

Metrics:

- Login success rate.
- Bootstrap success rate.
- Gateway success/error rate.
- Payment success/callback failure rate.
- Average API latency.
- Database and Redis health.
- Upstream provider error rate.
- Cost per user or total upstream cost.
- P0/P1/P2 support tickets.

Pass criteria:

- No ongoing P0/P1.
- Metrics stable.
- Known issues have owners.
- Continue/rollback/fix-forward decision recorded.

## Phase 12: Final Report

Purpose:

- Produce a single decision artifact that tells the project owner what was tested, what passed, what failed and whether the release should proceed.

Codex steps:

1. Read all evidence files in the current `test-runs/<timestamp>/` directory.
2. Summarize environment, build/version, accounts used and test scope.
3. List commands executed and whether each passed.
4. List browser and desktop scenarios executed and whether each passed.
5. List defects grouped by P0/P1/P2/P3.
6. Summarize security, compliance, observability, cost/abuse and support readiness.
7. State go/no-go recommendation.
8. Write `12-final-report.md`.

Pass criteria:

- Final report includes evidence references.
- P0/P1 status is explicit.
- Deferred risks have owner, impact and target date.
- Recommendation is one of: `go`, `go with accepted risks`, `no-go`.

## Defect Record Format

```text
ID:
Title:
Severity: P0 | P1 | P2 | P3
Environment:
Account:
Build/version:
Steps:
Expected:
Actual:
Evidence:
Likely owner:
Blocking release: yes/no
Decision:
```

## Autonomous Test Prompt

Use this prompt when asking Codex to execute tests:

```text
请阅读 C:\Users\23293\Desktop\codex+++\codex-plus-dev-plan 下的 README.md、QA-TESTING-ACCEPTANCE-PLAN.md、CODEX-AUTONOMOUS-TEST-RUNBOOK.md、MVP-IMPLEMENTATION-PLAN.md、PRODUCTION-LAUNCH-PLAN.md、PRODUCTION-ENVIRONMENT-MATRIX.md、BUSINESS-CONFIG-DECISION-TABLE.md、SERVER-SIZING-AND-SCALING-GUIDE.md、DEPLOYMENT-AUTOMATION-RUNBOOK.md、CONTRACT-GATE.md、INTEGRATION-VERIFICATION-CHECKLIST.md、SECURITY-REVIEW-PLAN.md、COMPLIANCE-PRIVACY-LEGAL-CHECKLIST.md、OBSERVABILITY-SLO-ALERTING-PLAN.md、COST-CONTROL-AND-ABUSE-RUNBOOK.md 和 SUPPORT-OPERATIONS-RUNBOOK.md。

你现在是测试执行 coordinator。请先执行 Phase 0 Readiness and Environment Discovery，不要进行真实支付、生产配置修改或破坏性操作。创建 test-runs/<timestamp>/ 证据目录，检查源码目录、工具链、服务依赖、测试账号和环境变量是否齐全。然后给出可自动执行、需要用户授权、当前阻断三类清单。
```

## Human Authorization Checklist

Codex 需要用户明确授权后才能执行：

- 真实支付或小额真实扣款。
- 生产环境配置修改。
- 生产数据库迁移。
- 生产回滚。
- 生产 smoke test 中的真实请求。
- 删除/撤销真实用户设备。
- 修改真实用户权益。
- 发送真实用户通知。
- 使用真实上游模型 Key 做成本较高请求。

Without authorization, Codex should use local, MVP test or staging only.
