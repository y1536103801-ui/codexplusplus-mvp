# 04 Manual Provider Switch

Result: pass
Snapshot subset result: pass

Runtime manual provider selection result: pass.
Runtime manual provider request result: pass.
Manual provider can still be selected after upgrade: isolated Desktop Manager UI showed `manual-e2e` and `Codex++ Cloud` together, then showed `manual-e2e` as `使用中`.
Manual provider can still be used after managed cloud refresh: local switch request recorded `manager.switch_relay_profile.ok` and wrote isolated `config.toml` back to `model_provider = "manual-e2e"`; no real upstream call was made because the seed provider uses a `.invalid` endpoint.
Manual provider content unchanged after managed cloud refresh: True.
Default user path still shows managed cloud entry point: snapshot evidence shows managed Cloud provider exists after upgrade.
Advanced users can reach provider configuration: isolated Desktop Manager UI navigated to advanced providers and displayed both provider cards.

## Snapshot Inspection

- Manual providers after upgrade: custom, manual-e2e.
- Managed providers after upgrade: Codex++ Cloud.
