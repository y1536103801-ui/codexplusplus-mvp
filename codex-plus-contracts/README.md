# Codex++ MVP Contracts

This directory contains the Phase 0 contract artifacts required before backend,
desktop, admin and E2E implementation workers start.

Source gate:

- `codex-plus-dev-plan/CONTRACT-GATE.md`

## Artifacts

- `api/client-openapi.yaml`: client API contract for browser handoff login, bootstrap, usage, devices and redeem.
- `config/*.schema.json`: PlanCatalog, ModelCatalog, UsagePolicy and FeatureFlags contracts.
- `status-error/client-status-errors.md`: shared service statuses, error codes and redaction rules.
- `events/client-events.schema.json`: shared event schema for bootstrap, devices, usage, redeem, gateway and local provider write failure.
- `test-fixtures/client/*.json`: stable mock fixtures for desktop runtime and UI work, including browser handoff pending/completed states.
- `compatibility-matrix.md`: consumer ownership and type sync plan.
- `change-review-policy.md`: change control after Phase 0.
- `storage-decision.md`: MVP storage and migration decision.
- `coordinator-review.md`: Phase 0 approval record and worker rules.

## Dispatch Rule

Implementation workers may consume these files. They must not edit contract
artifacts unless the coordinator explicitly approves a contract patch under
`change-review-policy.md`.
