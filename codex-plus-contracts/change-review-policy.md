# Codex++ Contract Change Review Policy

This file controls contract changes after MVP Phase 0 starts implementation.

## Change Classes

| Class | Examples | Required approval |
| --- | --- | --- |
| Patch | typo, description-only update, extra example | Coordinator |
| Additive | optional nullable response field, new mock metadata | Coordinator + affected consumers |
| Behavioral | changed retryability, changed error mapping, changed default | Coordinator + owning backend/client module |
| Breaking | rename/remove field, endpoint path change, enum removal | Full stop and downstream prompt update |

## Required Change Record

Every contract change must record:

- contract file
- old value or behavior
- new value or behavior
- reason
- affected modules
- fixture updates
- test updates
- rollback path

## Stop Conditions

A worker must stop and report instead of editing implementation when:

- it needs a response field missing from `api/client-openapi.yaml`
- it needs a config field missing from `config/*.schema.json`
- it needs a status or error code missing from `status-error/client-status-errors.md`
- it needs an event field missing from `events/client-events.schema.json`
- a mock fixture contradicts OpenAPI or status/error definitions
- a requested change would make the client calculate price, model multiplier, quota or limit state

## Coordinator Duties

- Review contract patch before implementation changes.
- Update `WORKER-PROMPTS.md` if any module input/output changes.
- Update `PARALLEL-DISPATCH-PLAN.md` if a change alters dependencies or merge order.
- Update `INTEGRATION-VERIFICATION-CHECKLIST.md` if a new failure path or gate is introduced.

## Post-Change Validation

Run or perform:

- YAML parse for `api/client-openapi.yaml`
- JSON parse for schema and fixture files
- manual check that fixtures use known statuses/error codes
- link check for docs that reference changed files
