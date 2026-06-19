Report status: final
Worker lane: Audit
Forbidden edits: none

## Changed files

- `sub2api-main/backend/internal/service/codexplus_audit_risk.go`
- `sub2api-main/backend/internal/service/codexplus_audit_risk_test.go`
- `codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-audit-risk-final.md`

## Implementation

- Kept the previous audit/risk implementation and finished it in place instead of rewriting it.
- Defined the Codex++ audit/risk event surface for login, device registration, bootstrap, managed key lifecycle, gateway rejection, admin enforcement, config changes, entitlement/payment/usage reconciliation, and risk signals.
- Added `RecordCodexPlusAuditRiskEvent` as the minimal append landing helper over the existing `CodexPlusEvent` store contract.
- Made audit/risk helpers self-contained from the gateway worker file by pinning the gateway rejection event and error-code strings used for tagging. This preserves the stage contract while allowing audit tests to run even if gateway work is temporarily in flux.
- Normalized payloads with event category, severity, retention policy, risk tags, redaction marker, timestamps, and deterministic request-linked event IDs.
- Hardened redaction so audit records drop or redact full API keys, bearer/JWT-like values, token-bearing URLs, prompts, request/response payloads, file/code content, source code, and user local paths even when they appear in top-level summaries or nested metadata.
- Implemented query projection and summaries that support lookup by user, user+device, request ID when the store supports it, event type, and risk tags. Gateway rejection summaries retain request ID, device ID, model, config version, error code, risk tags, and usage event ID linkage.
- No automatic ban, account blocking, or client-decided risk enforcement was implemented.

## Verification

- `gofmt -l sub2api-main/backend/internal/service/codexplus_audit_risk.go sub2api-main/backend/internal/service/codexplus_audit_risk_test.go`
- `go test .\internal\service\codexplus_foundation.go .\internal\service\codexplus_audit_risk.go .\internal\service\codexplus_audit_risk_test.go -run CodexPlusAuditRisk -count=1 -v`
- `go test ./internal/service -run CodexPlusAuditRisk -count=1 -v`
- `go test ./internal/service -count=1`

All commands passed in `sub2api-main/backend`.

## Coordinator follow-up

- If admin/support needs global request ID or global device ID search without first knowing the user ID, add repository-level `ListByRequestID` and/or global device query support outside this worker lane. The audit helper already detects optional request/device store interfaces.
- Wire any new admin API endpoints to `QueryCodexPlusAuditRiskEvents` with `IncludePayload=false` for normal support views unless a privileged diagnostic workflow explicitly needs redacted payload metadata.
- Keep gateway rejection event payloads routed through `NormalizeCodexPlusAuditRiskPayload` so new gateway fields inherit the same redaction rules.

## Remaining risks

- The existing production event repository only exposes `Append` and `ListByUser`; request-ID-only lookup currently requires an optional repository extension or a user-scoped query.
- Redaction is intentionally conservative and may remove diagnostic detail when strings look like secrets, local paths, or token-bearing URLs.
