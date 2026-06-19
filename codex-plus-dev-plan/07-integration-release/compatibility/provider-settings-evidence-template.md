# Provider Settings Evidence Template

Status: compatibility evidence pending

Use this template when a runnable environment and old-version provider settings are available. Do not mark any item as passed without attaching concrete evidence.

## Scaffold And Verification

Generate a timestamped compatibility evidence folder with:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-compatibility-evidence.ps1
```

For Windows MVP runtime testing, prepare an isolated desktop profile and helper scripts with:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-desktop-compatibility-harness.ps1
```

The harness creates isolated `USERPROFILE`, `HOME`, `APPDATA`, `LOCALAPPDATA`, and `CODEX_HOME` values plus a fake manual provider seed. It does not launch Desktop Manager or prove runtime compatibility by itself. Launch and provider-write steps still require explicit owner authorization.

If provider snapshots are collected manually, use the hash-only snapshot capturer instead of copying raw config files into evidence:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/capture-07-desktop-provider-snapshot.ps1 -ProfileRoot <isolated-userprofile> -Label <pre-upgrade|post-upgrade|logout|rollback> -OutputPath <snapshot.json>
```

The generated snapshot stores provider names and URL/key hashes only. It must not contain raw API keys, JWTs, Authorization headers, upstream credentials, or `.env` secrets.

After collecting pre-upgrade, post-upgrade, logout, and rollback provider snapshots, run the read-only snapshot inspection runner:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1 -PreUpgradeSnapshot <pre.json-or-toml> -PostUpgradeSnapshot <post.json-or-toml> -LogoutSnapshot <logout.json-or-toml> -RollbackSnapshot <rollback.json-or-toml> -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility
```

The runner updates the compatibility evidence folder with sanitized provider-name comparisons, managed `Codex++ Cloud` presence, logout token-field checks, rollback provider preservation, and a check that no plan, price, multiplier, entitlement, or usage policy data was written into the local provider snapshot. It does not prove UI navigation, actual provider request success, or rollback command execution.

After replacing every placeholder with sanitized upgrade, logout, provider sync, and rollback evidence, run:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-compatibility-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility
```

After E2E, package and Docs product copy evidence also exist, Module J must include this compatibility folder in the aggregate release evidence gate:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1 -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs
```

Generated scaffolds intentionally contain `TODO` and `pending` placeholders. The verifier must fail until all placeholders are replaced with real sanitized compatibility evidence.

The latest compatibility verifier requires both the snapshot-inspection subset and runtime compatibility evidence. Snapshot-only output from `inspect-07-compatibility-snapshots.ps1` is intentionally not enough for a final compatibility pass. Before Module J consumes the folder:

- `00-test-context.md` must contain `Result: pass`.
- `00-test-context.md` must include `Snapshot Inputs` with `pre-upgrade`, `post-upgrade`, `logout`, and `rollback` entries marked `parsed`.
- `00-test-context.md` must include `Missing Inputs` with `- none`.
- `00-test-context.md` must include `Parse Failures` with `- none`.
- `00-test-context.md` must include `All snapshots token-field scan clear: True.`, `All snapshots commercial-policy scan clear: True.`, `Legacy relayProfiles/settings parsed: True.`, and `Manual provider content comparison uses nonprinted base URL/API key hashes: True.`.
- `01-pre-upgrade-snapshot.md` must contain `Result: pass`, `Snapshot Inspection`, `Legacy relayProfiles/settings parsed: True.`, and a positive `Manual provider count: <n>.` line.
- `02-post-upgrade-cloud.md` must prove manual providers survived, manual provider content is unchanged, managed `Codex++ Cloud` was written or refreshed without overwriting manual providers, no local commercial-policy data was written, `Missing manual providers after upgrade: none.`, and `Manual providers with changed content after upgrade: none.`.
- `03-cloud-logout-boundary.md` must prove runtime login/logout passed, manual providers remained unchanged after logout, manual provider content remained unchanged, `Missing manual providers after logout: none.`, `Manual providers with changed content after logout: none.`, and `Logout token-field scan clear: True.`.
- `04-manual-provider-switch.md` must prove runtime manual provider selection passed, runtime manual provider request passed, and manual provider content remained unchanged after managed cloud refresh.
- `05-provider-sync.md` must prove runtime provider sync log review passed and no manual provider content changed after upgrade or logout.
- `06-rollback-rehearsal.md` must prove runtime rollback rehearsal passed, config/desktop/backend-gateway rollback boundaries, manual provider content remained unchanged, `Missing manual providers after rollback: none.`, and `Manual providers with changed content after rollback: none.`.
- `07-compatibility-gate-report.md` must record `Compatibility snapshot subset result: pass`, `Runtime compatibility result: pass`, and the `inspect-07-compatibility-snapshots.ps1` command.

## Test Context

Result: pending / pass / fail

- Tester:
- Date:
- Codex++ desktop version before upgrade:
- Codex++ desktop version after upgrade:
- Operating system:
- Test environment:
- Legacy settings source:
- Runtime build or commit:

## Snapshot Hygiene

- All snapshots token-field scan clear: pending / True / False.
- All snapshots commercial-policy scan clear: pending / True / False.
- Legacy relayProfiles/settings parsed: pending / True / False.
- Manual provider content comparison uses nonprinted base URL/API key hashes: pending / True / False.

## Snapshot Inputs

- pre-upgrade: pending / parsed / missing
- post-upgrade: pending / parsed / missing
- logout: pending / parsed / missing
- rollback: pending / parsed / missing

## Missing Inputs

- pending / none / list missing snapshot inputs

## Parse Failures

- pending / none / list parse failures

## Pre-Upgrade Snapshot

Record the manual provider state before upgrade.

Result: pending / pass / fail

| Provider name | Provider type | Base URL redacted | API key redacted | Default provider | Notes |
| --- | --- | --- | --- | --- | --- |
|  |  |  |  |  |  |

Evidence attachments:

- Settings snapshot path:
- Screenshot path:
- Log excerpt path:

## Snapshot Inspection

- Pre-upgrade snapshot parsed: pending / True / False
- Legacy relayProfiles/settings parsed: pending / True / False
- Manual provider count: pending / positive integer followed by `.`
- Managed provider count before upgrade:
- Snapshot contents were not copied into this evidence folder.

## Post-Upgrade Snapshot

Record the provider state after upgrade and managed cloud login or refresh.

Result: pending / pass / fail

Manual providers preserved after upgrade: pending / True / False.
Manual provider content unchanged after upgrade: pending / True / False.
Codex++ Cloud provider written or refreshed without overwriting manual providers: pending / True / False.
No plan, price, multiplier, entitlement, or usage policy data was written by migration: pending / True / False.
Managed provider stores only required runtime configuration:

| Provider name | Provider type | Base URL redacted | API key redacted | Default provider | Notes |
| --- | --- | --- | --- | --- | --- |
| Codex++ Cloud | Managed |  |  |  |  |
|  | Manual |  |  |  |  |

Evidence attachments:

- Settings snapshot path:
- Screenshot path:
- Log excerpt path:

## Snapshot Inspection

- Manual providers before upgrade:
- Manual providers after upgrade:
- Missing manual providers after upgrade: pending / none. / list names
- Manual providers with changed content after upgrade: pending / none. / list names
- Managed providers after upgrade:

Final evidence must use `Missing manual providers after upgrade: none.` and `Manual providers with changed content after upgrade: none.`.

## Cloud Logout Boundary

Result: pending / pass / fail

Cloud login creates only expected cloud/session state:
Cloud logout clears cloud session state: pending / True / False.
Runtime cloud login/logout evidence result: pending / pass / fail.
Manual providers remain unchanged after logout: pending / True / False.
Manual provider content unchanged after logout: pending / True / False.
Redacted before and after provider snapshots are compared by provider names and nonprinted URL/API key hashes.

## Snapshot Inspection

- Manual providers before upgrade:
- Manual providers after logout:
- Missing manual providers after logout: pending / none. / list names
- Manual providers with changed content after logout: pending / none. / list names
- Logout token-field scan clear: pending / True. / False.

Final evidence must use `Missing manual providers after logout: none.`, `Manual providers with changed content after logout: none.`, and `Logout token-field scan clear: True.`.

## Manual Provider Switch

Result: pending / pass / fail

Manual provider can still be selected after upgrade:
Manual provider can still be used after managed cloud refresh:
Runtime manual provider selection result: pending / pass / fail.
Runtime manual provider request result: pending / pass / fail.
Manual provider content unchanged after managed cloud refresh: pending / True / False.
Default user path still shows managed cloud entry point:
Advanced users can reach provider configuration:

## Provider Sync

Result: pending / pass / fail

Provider sync recognizes legacy profiles:
Provider sync does not corrupt manual provider entries:
Runtime provider sync log review result: pending / pass / fail.
Provider sync does not log full API keys, JWTs, Authorization headers, upstream credentials, or `.env` secrets:
Redacted sync logs and snapshot diff:

## Snapshot Inspection

- Changed content after upgrade: pending / none. / list names
- Changed content after logout: pending / none. / list names

## Rollback Rehearsal

Result: pending / pass / fail

Config rollback preserves or recovers manual providers: pending / True / False.
Runtime rollback rehearsal result: pending / pass / fail.
Manual provider content unchanged after rollback: pending / True / False.
Desktop rollback keeps advanced provider settings reachable:
Backend/gateway rollback does not force managed-provider-only assumptions:
Failed provider write recovery from last settings snapshot:
User-side key exposure response, if applicable, is redacted and owned.

## Snapshot Inspection

- Manual providers before upgrade:
- Manual providers after rollback:
- Missing manual providers after rollback: pending / none. / list names
- Manual providers with changed content after rollback: pending / none. / list names

Final evidence must use `Missing manual providers after rollback: none.` and `Manual providers with changed content after rollback: none.`.

## Required Assertions

- [ ] Manual providers from the pre-upgrade snapshot still exist.
- [ ] Manual provider base URLs are unchanged.
- [ ] Manual provider API keys are unchanged and redacted in evidence.
- [ ] `Codex++ Cloud` stores only required runtime configuration.
- [ ] No plan, price, multiplier, entitlement, or usage policy data is written by migration.
- [ ] Cloud logout removes cloud session state without deleting manual providers.
- [ ] Provider sync still recognizes legacy profiles.
- [ ] Logs do not include full API keys, JWTs, Authorization headers, upstream credentials, or `.env` secrets.

## Result

- Overall result: compatibility evidence pending
- Snapshot inspector command:
  - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-compatibility-snapshots.ps1 -PreUpgradeSnapshot <path> -PostUpgradeSnapshot <path> -LogoutSnapshot <path> -RollbackSnapshot <path> -EvidenceDir <compatibility-evidence-dir>`
- Compatibility snapshot subset result: pending / pass / fail
- Runtime compatibility result: pending / pass / fail
- Blocking issues:
- Follow-up owner:
- Follow-up due date:
