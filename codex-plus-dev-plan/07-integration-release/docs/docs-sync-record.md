# Docs Sync Record

Report status: final
Worker lane: Docs
Date: 2026-06-17
Final docs evidence run stamp: 20260618-1403-docs

## Public documents updated

- `产品方案书.md`
  - Created product plan.
  - Added purchase/install/login/launch user journey.
  - Added admin configuration model for plans, models, quotas and feature flags.
  - Added Industrial v2 architecture using Control Plane, Data Plane, Client Runtime and Platform Ops.
  - Added rollout, rollback, audit and reconciliation copy.
  - Explicitly states that variable sales content is backend-configured.

- `产品说明书.md`
  - Created user and administrator manual.
  - Added first-task guidance for users unfamiliar with Codex.
  - Added state explanations for not purchased, expired, insufficient balance, revoked device, unavailable model, gateway unavailable and local config failed.
  - Added release and rollback operating notes.
  - Explicitly states that fixed prices, fixed models and fixed quotas are not client promises.

- `codex-plus-product-spec.html`
  - Reviewed existing page for docs sync scope.
  - Synced visible HTML copy to remove fixed model/quota/API-key examples.
  - Aligned architecture wording with Control Plane, Data Plane, Client Runtime and Platform Ops.
  - Replaced demo model/quota values with backend-configured and backend snapshot wording.

## Stage docs created

- `codex-plus-dev-plan/07-integration-release/docs/user-guide.md`
- `codex-plus-dev-plan/07-integration-release/docs/admin-operations-guide.md`
- `codex-plus-dev-plan/07-integration-release/docs/release-notes-draft.md`
- `codex-plus-dev-plan/07-integration-release/docs/docs-sync-record.md`
- `codex-plus-dev-plan/07-integration-release/docs/html-sync-evidence.md`

## E2E evidence still required

- Purchase or test entitlement opens the account, then desktop login, device refresh, bootstrap, provider write, Codex launch and one successful request.
- Failure path evidence for not purchased, expired entitlement, insufficient balance, revoked device, unavailable model, gateway unhealthy and local config failed.
- Admin config change evidence showing plan/model/quota updates appear in the client without a client release.
- Config rollback evidence showing the client refreshes to a known stable snapshot.
- Usage event and billing/metering reconciliation evidence.
- Installer, upgrade and rollback evidence showing old manual providers survive.

## Consistency notes

- Documentation follows `ARCHITECTURE.md` four-layer boundaries.
- Documentation follows `INTEGRATION-VERIFICATION-CHECKLIST.md` release readiness language.
- No public copy includes upstream credentials, internal risk rules or real cost structure.
- No public copy promises a fixed built-in price, fixed built-in model or fixed built-in quota.
- HTML page now avoids fixed model, fixed quota and user-key examples.
- Local Chromium visual evidence passed; in-app Browser visual evidence for the local HTML file passed through local HTTP preview on this host, while direct `file://` rendering remains blocked by URL policy and is recorded in `html-visual-evidence/in-app-browser-policy-boundary.md`.
- Final Docs product copy release evidence uses `test-runs/20260618-1403-docs` and passes `tools/verify-07-docs-product-copy-evidence.ps1`.
- Aggregate evidence, coverage, readiness, handoff and Module J report checks should consume this docs lane as docs/product-copy evidence only. It does not replace real E2E, package, compatibility or business readiness evidence; those non-doc release lanes remain pending/no-go until their own real evidence passes.
