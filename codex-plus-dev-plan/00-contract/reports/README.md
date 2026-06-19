# 00-Contract Worker Final Reports

This folder stores final reports for the active `00-contract` parallel restart.

Required files:

- `worker-a-client-api-final.md`
- `worker-b-admin-config-final.md`
- `worker-c-status-error-event-final.md`

Templates:

- `worker-a-client-api-final.template.md`
- `worker-b-admin-config-final.template.md`
- `worker-c-status-error-event-final.template.md`

Templates are intentionally marked `Report status: draft` and must not be renamed in place without completing the report content.

Each report must include:

- `Report status: final`
- matching `Worker lane: A`, `Worker lane: B` or `Worker lane: C`
- `Forbidden edits: none`
- changed files
- verification commands and results
- forbidden-file pressure, if any
- preaudit item answers
- downstream assumptions

Preaudit answers must mark every assigned item as one of:

- `fixed`
- `deferred`
- `rejected`

The coordinator may not mark `00-contract` passed until all three reports exist and `tools/validate-stage-gate.ps1` passes.
