# 07 Release Coverage Summary

Report status: generated
Generated at: 2026-06-19 19:48:27+08:00
Coverage status: complete
MVP package scope: windows-only
Missing coverage count: 0
Nonrelease marker count: 0

## Evidence Inputs

- E2E evidence folder: codex-plus-dev-plan\test-runs\20260619-1940-e2e
- Package evidence folder: codex-plus-dev-plan\test-runs\20260619-1940-package
- Compatibility evidence folder: codex-plus-dev-plan\test-runs\20260619-1940-compatibility
- Docs product copy evidence folder: codex-plus-dev-plan\test-runs\20260619-1940-docs

## Coverage Matrix

| Lane | Requirement | Status | Evidence |
| --- | --- | --- | --- |
| e2e | test-account matrix covers all user states | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\01-test-accounts.md |
| e2e | browser handoff login contract and poll path | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\04-client-api-e2e.md |
| e2e | bootstrap and device refresh evidence | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\04-client-api-e2e.md |
| e2e | gateway active-user success | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\05-gateway-policy-e2e.md |
| e2e | gateway no-entitlement rejection | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\05-gateway-policy-e2e.md |
| e2e | gateway expired rejection | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\05-gateway-policy-e2e.md |
| e2e | gateway insufficient-balance rejection | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\05-gateway-policy-e2e.md |
| e2e | gateway revoked-device rejection | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\05-gateway-policy-e2e.md |
| e2e | gateway unauthorized-model rejection | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\05-gateway-policy-e2e.md |
| e2e | desktop Manager login/bootstrap/provider-write/Codex launch | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\06-desktop-manager-e2e.md |
| e2e | manual providers survive managed provider write | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\06-desktop-manager-e2e.md |
| e2e | usage and rejection audit evidence | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\09-usage-events-audit.md |
| e2e | rollback covers config/backend/desktop/entitlement/provider write | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\10-rollback-notes.md |
| package | Windows fresh/overwrite/uninstall-reinstall | covered | codex-plus-dev-plan\test-runs\20260619-1940-package\01-windows-x64-install.md |
| package | macOS x64 DMG mount/gatekeeper/reinstall | deferred-post-mvp | post-MVP by Windows-only MVP scope |
| package | macOS arm64 DMG mount/gatekeeper/reinstall | deferred-post-mvp | post-MVP by Windows-only MVP scope |
| package | package artifact inspection excludes shared credentials and fixed policy | covered | codex-plus-dev-plan\test-runs\20260619-1940-package\04-artifact-inspection.md |
| e2e | missing Codex first-run assistant evidence | covered | codex-plus-dev-plan\test-runs\20260619-1940-e2e\07-package-install-check.md |
| compatibility | pre-upgrade manual providers and redacted keys | covered | codex-plus-dev-plan\test-runs\20260619-1940-compatibility\01-pre-upgrade-snapshot.md |
| compatibility | post-upgrade manual providers preserved and managed cloud present | covered | codex-plus-dev-plan\test-runs\20260619-1940-compatibility\02-post-upgrade-cloud.md |
| compatibility | migration does not write local commercial policy | covered | codex-plus-dev-plan\test-runs\20260619-1940-compatibility\02-post-upgrade-cloud.md |
| compatibility | cloud logout leaves manual providers unchanged | covered | codex-plus-dev-plan\test-runs\20260619-1940-compatibility\03-cloud-logout-boundary.md |
| compatibility | manual provider switch remains usable | covered | codex-plus-dev-plan\test-runs\20260619-1940-compatibility\04-manual-provider-switch.md |
| compatibility | provider sync recognizes legacy profiles without corruption | covered | codex-plus-dev-plan\test-runs\20260619-1940-compatibility\05-provider-sync.md |
| compatibility | rollback rehearses config/desktop/backend-gateway recovery | covered | codex-plus-dev-plan\test-runs\20260619-1940-compatibility\06-rollback-rehearsal.md |
| docs | docs sync is final and backend-configured | covered | codex-plus-dev-plan\test-runs\20260619-1940-docs\00-docs-sync-record.md |
| docs | user guide covers cloud flow and failure states | covered | codex-plus-dev-plan\test-runs\20260619-1940-docs\01-user-guide.md |
| docs | admin guide covers config rollback audit reconciliation | covered | codex-plus-dev-plan\test-runs\20260619-1940-docs\02-admin-operations-guide.md |
| docs | release notes are final with rollback compatibility and risks | covered | codex-plus-dev-plan\test-runs\20260619-1940-docs\03-release-notes.md |
| docs | HTML sync has browser visual pass and residue scan | covered | codex-plus-dev-plan\test-runs\20260619-1940-docs\04-html-sync-evidence.md |
| docs | docs product copy gate report passes | covered | codex-plus-dev-plan\test-runs\20260619-1940-docs\06-docs-product-copy-gate-report.md |

## Nonrelease Markers

- none

## Release Boundary

This coverage summary maps evidence files to the required 07 release scenarios. It does not execute E2E, build packages, run compatibility migration, or make the release go/no-go decision.
