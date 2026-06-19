# 06 Desktop Manager E2E

Run folder: 20260619-1940-e2e
Status: pass

Result: pass

## Desktop Manager Results

Manager login result: pass
Codex++ Cloud provider result: pass
Manual provider preservation result: pass
Codex launch result: pass
Manager login bootstrap Codex++ Cloud Codex launch flow: pass

## Evidence

- Isolated harness: `codex-plus-dev-plan/test-runs/_desktop-harness/20260619-1940-desktop-harness`.
- Manager loaded legacy Windows UTF-8 BOM settings and showed `manual-e2e` before Cloud configuration.
- Browser handoff completed against local backend; complete response returned `status=completed` and no desktop token fields.
- Manager UI showed account `codexplus-e2e-active@local.test`, plan `Starter`, device `active`, default model `Codex Standard`, and Key status `configured`.
- `Codex++ Cloud` provider was written while `manual-e2e` remained present.
- Launch smoke recorded `manager.launch_requested` with helper/debug ports in isolated Manager log.
- Manual provider rollback switched UI back to `manual-e2e` and wrote isolated `config.toml` to `model_provider = "manual-e2e"`.
- Missing local Codex behavior: Manager reported official Codex not detected for install helper; launch request evidence is accepted for this Windows-only local MVP smoke.
