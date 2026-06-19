# Codex++ Contract Compatibility Matrix

This document assigns ownership for generated or manually maintained types that
consume Phase 0 contracts.

## Contract Version

- Contract version: `0.1.0-mvp`
- Date: 2026-06-16
- Scope: local/test MVP
- Breaking changes after this file is adopted require coordinator review.

## Consumer Matrix

| Contract artifact | Go backend | Desktop Rust | Manager TypeScript | Admin frontend | E2E |
| --- | --- | --- | --- | --- | --- |
| `api/client-openapi.yaml` | Module D owns auth handoff and client request/response structs | Module F consumes desktop handoff/bootstrap/device/usage structs | Module G consumes UI DTOs | read-only | Module I validates actual response |
| `config/*.schema.json` | Module C owns validation/defaults | read-only | read-only | Module H owns form validation display | Module I verifies config impact |
| `status-error/client-status-errors.md` | Modules D/E map backend and gateway errors | Module F maps local/runtime errors | Module G maps UI states | Module H displays admin summaries | Module I checks failure paths |
| `events/client-events.schema.json` | Modules D/E emit server/gateway events | Module F emits local write failure event | read-only | Module H displays audit/risk summary | Module I verifies event visibility |
| `test-fixtures/client/*.json` | Module D contract tests | Modules F/G mock runtime/UI | Module G story/test fixtures | read-only | Module I uses scenario setup |

## Type Ownership

| Language | Owner module | Initial approach | Notes |
| --- | --- | --- | --- |
| Go | D/E/C | Manual DTOs are acceptable for MVP | Must match OpenAPI and schema names. |
| Rust | F | Manual serde structs are acceptable for MVP | Must preserve unknown fields only if explicitly needed. |
| TypeScript | G/H | Generated or hand-written types beside feature modules | UI cannot add fields absent from fixtures. |

## Compatibility Rules

- Additive nullable fields are allowed after coordinator review.
- Field rename/removal is breaking and requires updates to OpenAPI, fixtures, worker prompts and E2E checks.
- New service status requires status table update, at least one fixture and UI mapping.
- New gateway rejection requires event schema update and E2E failure path.
- Config schema default changes require rollback notes and backend defaulting tests.

## Manual Sync Checklist

Before implementation workers start:

- [ ] Module D confirms OpenAPI paths and legacy-compatible envelope shape.
- [ ] Module F confirms desktop handoff and bootstrap fixtures can be deserialized.
- [ ] Module G confirms all UI states have fixture examples.
- [ ] Module E confirms gateway rejection codes are in the status table.
- [ ] Module H confirms config schemas cover admin form fields.
