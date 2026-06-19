# Integration Verification Checklist

本文档是 Module J 集成会话的最终验收清单。它不替代各模块测试，而是确认所有模块合并后仍满足 MVP 和工业级边界。更完整的测试执行流程见 [QA-TESTING-ACCEPTANCE-PLAN.md](QA-TESTING-ACCEPTANCE-PLAN.md)，Codex 自动执行步骤见 [CODEX-AUTONOMOUS-TEST-RUNBOOK.md](CODEX-AUTONOMOUS-TEST-RUNBOOK.md)。

## Pre-Merge Review

每个模块合并前检查：

- [ ] worker final report 已提交。
- [ ] 依赖契约已满足，或假设已记录。
- [ ] 没有修改 `FILE-OWNERSHIP-MATRIX.md` 中的 forbidden 文件。
- [ ] 没有引入真实 API Key、JWT、Authorization header、上游凭证或 `.env` secrets。
- [ ] 没有绕过后端权限/权益/网关 enforcement。
- [ ] 没有删除或覆盖用户已有手动供应商配置。
- [ ] package manifest/lockfile 只由明确 owner 修改。
- [ ] 新增状态、字段、事件、配置项已经更新契约文件和 mocks。

## Merge Order

按以下顺序合并：

1. Module A: Contracts.
2. Module B: Storage and migration decision.
3. Module C: Config Registry.
4. Module D: Client API.
5. Module E: Gateway Enforcement.
6. Module F: Desktop Runtime.
7. Module H: Admin Operations.
8. Module G: Desktop UX.
9. Module I: E2E Release Gate.
10. Module J: docs sync and final fixes.

## Contract Checks

- [ ] `GET /api/v1/client/bootstrap` actual response matches OpenAPI and mocks.
- [ ] `POST /api/v1/auth/desktop/start` returns browser URL plus desktop-private `poll_token`, and the URL does not contain `poll_token`.
- [ ] `POST /api/v1/auth/desktop/complete` requires Web JWT and returns no desktop token.
- [ ] `POST /api/v1/auth/desktop/poll` consumes completed sessions before issuing token pairs and rejects repeated completed polls.
- [ ] `GET /api/v1/client/usage` actual response matches contract.
- [ ] `POST /api/v1/client/devices` is idempotent.
- [ ] `POST /api/v1/client/redeem` maps errors to approved status/error codes.
- [ ] Config schema defaults match backend defaults.
- [ ] Status/error table covers every backend, gateway and desktop-local status used by code.
- [ ] Event fields include user/device/request/config context where required.
- [ ] No consumer uses fields absent from contract fixtures.

## Backend Checks

Run from `sub2api-main/`:

```powershell
make test-backend
```

If unavailable, run from `sub2api-main/backend/`:

```powershell
go test ./...
```

Required targeted coverage:

- [ ] Config validation rejects invalid plan/model/usage/feature flag data.
- [ ] Bootstrap requires JWT.
- [ ] Bootstrap does not leak upstream real credentials.
- [ ] Same user repeated bootstrap does not duplicate Codex++ Key.
- [ ] Device upsert is user-isolated.
- [ ] Revoked device causes bootstrap `device_revoked`.
- [ ] Gateway rejects unauthorized model.
- [ ] Gateway rejects expired entitlement.
- [ ] Gateway rejects insufficient balance according to MVP policy.
- [ ] Valid request still forwards and records usage.
- [ ] Logs redact API Key/JWT/Authorization/Base URL tokens.
- [ ] Browser handoff diagnostics never include `poll_token`, access token or refresh token.

## Admin Frontend Checks

Run from `sub2api-main/frontend/`:

```powershell
pnpm run lint:check
pnpm run typecheck
pnpm run test:run
```

Required behavior:

- [ ] Admin can configure plans without client-side-only validation.
- [ ] Admin can configure models and default model.
- [ ] Admin can configure usage policy and feature flags.
- [ ] Admin can inspect user entitlement/device/Key summary without full secrets.
- [ ] Invalid backend validation errors are visible and actionable.

## Desktop Checks

Run from `CodexPlusPlus-main/`:

```powershell
cargo test --workspace
```

Run from `CodexPlusPlus-main/apps/codex-plus-manager/`:

```powershell
npm run check
npm run build
```

Required behavior:

- [ ] Desktop can fetch bootstrap using authenticated user state.
- [ ] Desktop default login uses browser handoff; direct password login remains compatibility-only and is not the production Turnstile path.
- [ ] Pending browser handoff exposes authorize URL and 6 digit verification code to UI, but never exposes `poll_token`.
- [ ] Desktop writes or updates only `Codex++ Cloud`.
- [ ] Existing manual providers survive login, refresh and logout.
- [ ] Provider API Key is not displayed or logged in full.
- [ ] Local session/cache stores only minimum required data.
- [ ] UI handles available, not purchased, expired, low balance, device revoked, gateway unhealthy and local config failed.
- [ ] Advanced provider config is hidden by default and feature-flag controlled.

## E2E Checks

Use a test environment and test user only.

Happy path:

- [ ] Admin opens entitlement or creates test paid state.
- [ ] User logs into Codex++ Manager.
- [ ] User completes Web browser authorization with matching 6 digit confirmation code.
- [ ] Device registers or refreshes.
- [ ] Bootstrap returns available.
- [ ] `Codex++ Cloud` provider is written.
- [ ] Codex launches from Manager.
- [ ] One model request succeeds through Sub2API gateway.
- [ ] Usage/log event is visible for the request.

Failure paths:

- [ ] User without entitlement sees not purchased.
- [ ] Expired entitlement blocks usage and shows renewal/action hint.
- [ ] Low or insufficient balance blocks gateway request according to MVP policy.
- [ ] Revoked device blocks bootstrap or launch path.
- [ ] Removed/default-disabled model disappears or is rejected server-side.
- [ ] Gateway unhealthy state gives actionable message.
- [ ] Local Codex missing/config write failure gives local repair hint.

## Release Readiness

- [ ] Docs match implemented behavior.
- [ ] HTML index does not promise features outside MVP unless marked later phase.
- [ ] Release notes include config version, contract version and snapshot behavior.
- [ ] Rollback notes cover config rollback, backend rollback, desktop rollback and manual provider preservation.
- [ ] Known risks are listed.
- [ ] Manual recovery path exists for bad config, accidental entitlement, leaked user-side Key and failed provider write.

## Final Report Template

```text
Modules merged:
- 

Conflicts resolved:
- 

Contract changes from original plan:
- 

Verification commands:
- command:
  result:

E2E result:
- 

Remaining risks:
- 

Rollback notes:
- 
```
