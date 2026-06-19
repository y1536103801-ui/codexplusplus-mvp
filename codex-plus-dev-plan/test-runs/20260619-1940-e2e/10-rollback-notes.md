# 10 Rollback Notes

Run folder: 20260619-1940-e2e
Status: pass

Result: pass

## Rollback Coverage

- Config rollback: pass; isolated `config.toml` returned to `model_provider = "manual-e2e"`.
- Backend rollback: pass; local backend/gateway policy remains loopback-only and can be stopped without production impact.
- Desktop rollback: pass; advanced provider UI switched from `Codex++ Cloud` back to `manual-e2e`.
- Entitlement correction: pass; admin audit and gateway scenarios expose structured policy status for entitlement/device correction.
- Provider write recovery: pass; compatibility snapshots recorded no missing manual providers and no changed manual provider content.
- Leaked user-side Key response process: release process owns user-facing response; evidence contains redacted/no-secret records only.
