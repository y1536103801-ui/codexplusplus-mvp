# Compatibility Migration Checklist

Status: compatibility evidence pending

This checklist is a scaffold for verifying that the new managed provider mode does not break existing Codex++ manual provider settings, update/install entry points, logs, or provider sync behavior.

## Scope

- Preserve legacy manual provider profiles and settings after upgrade.
- Show the managed `Codex++ Cloud` entry point by default for regular users.
- Keep advanced provider configuration reachable for advanced users.
- Ensure cloud logout clears only cloud session state and does not delete manual providers.
- Confirm provider sync remains compatible with existing settings.

## Required Evidence

- [ ] Legacy settings upgrade evidence captured.
- [ ] Existing manual providers remain present after upgrade.
- [ ] Existing manual provider base URL/API key fingerprints remain unchanged after upgrade, logout, provider sync, and rollback; evidence must not print the original values.
- [ ] `Codex++ Cloud` provider is written or updated without overwriting manual providers.
- [ ] Default user path shows managed cloud entry point.
- [ ] Advanced provider configuration remains accessible when enabled.
- [ ] Cloud login/logout does not remove manual provider entries.
- [ ] Manual provider switching still works.
- [ ] Provider sync behavior is unchanged for legacy profiles.
- [ ] Provider API keys are not displayed or logged in full.
- [ ] Migration does not write package, price, multiplier, entitlement, or usage policy data.
- [ ] Rollback rehearsal confirms legacy manual providers can be recovered or preserved.
- [ ] `tools/inspect-07-compatibility-snapshots.ps1` passes against pre-upgrade `relayProfiles/settings`, post-upgrade, logout, and rollback provider snapshots.
- [ ] `tools/verify-07-compatibility-evidence.ps1` passes against the final timestamped compatibility evidence folder.

## Verification Matrix

| Scenario | Expected result | Evidence status | Notes |
| --- | --- | --- | --- |
| Upgrade from legacy provider settings | Manual providers preserved | Pending | No old-version sample available. |
| Add or refresh `Codex++ Cloud` | Only managed provider runtime config is updated | Pending | Runtime unavailable. |
| Cloud logout | Manual provider list remains unchanged | Pending | Runtime unavailable. |
| Switch to manual provider | Manual provider can still be selected and used | Pending | Runtime unavailable. |
| Provider sync | Existing sync contract remains intact | Pending | Runtime unavailable. |
| Secrets in UI/logs | API keys and tokens are redacted | Pending | Runtime unavailable. |
| Rollback | Manual providers and cloud session boundaries remain recoverable | Pending | Runtime unavailable. |

## Evidence Notes

Compatibility evidence pending because no old-version sample settings, installed desktop runtime, or executable test environment was provided for this replacement worker lane.

Use `tools/new-07-compatibility-evidence.ps1` to generate the evidence folder once a runnable environment and old-version settings are available.
Use `tools/new-07-desktop-compatibility-harness.ps1` to prepare an isolated Windows desktop profile for Manager runtime evidence without touching the real user profile.
Use `tools/capture-07-desktop-provider-snapshot.ps1` to write hash-only provider snapshots from that isolated profile; do not copy raw provider config or real upstream keys into evidence.
Use `tools/inspect-07-compatibility-snapshots.ps1` to fill the snapshot-inspection subset from sanitized provider snapshots. This proves legacy `relayProfiles/settings` parsing, provider-list preservation, nonprinted manual provider URL/API key fingerprint preservation, token cleanup and local commercial-policy absence only; UI navigation, real provider switching, gateway requests, provider sync log review, and rollback execution still require runtime evidence. Snapshot-only output must not be treated as a full compatibility pass.
