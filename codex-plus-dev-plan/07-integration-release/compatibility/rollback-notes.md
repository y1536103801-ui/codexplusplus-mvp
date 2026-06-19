# Compatibility Rollback Notes

Status: compatibility evidence pending

These notes describe rollback expectations for the compatibility and migration lane. They are not proof of successful rollback execution.

## Rollback Goals

- Preserve user-created manual provider settings.
- Remove or disable only the managed `Codex++ Cloud` behavior introduced by the release.
- Avoid deleting legacy relay profiles, API keys, base URLs, or provider selection history.
- Keep a manual recovery path for failed provider writes or bad config migration.

## Config Rollback

1. Restore the previous known-good desktop configuration snapshot when available.
2. If no full snapshot is available, remove only the managed `Codex++ Cloud` provider entry or disable its feature flag.
3. Confirm manual provider entries remain present after rollback.
4. Confirm provider sync still reads legacy settings.

Evidence status: compatibility evidence pending.

## Desktop Rollback

1. Reinstall or relaunch the previous desktop build.
2. Start Codex++ Manager with network calls disabled or pointed at the previous backend when needed.
3. Verify advanced provider settings can still open.
4. Verify manual provider selection remains possible.

Evidence status: compatibility evidence pending.

## Backend/Gateway Rollback

1. Roll back backend and gateway services according to the release runbook.
2. Ensure desktop clients do not receive new managed-provider-only assumptions.
3. Confirm old manual provider behavior remains valid for users who never completed cloud login.

Evidence status: compatibility evidence pending.

## Manual Recovery

- If a managed provider write fails, restore the last provider settings snapshot.
- If cloud logout removes unexpected state, restore the provider settings snapshot and collect logs.
- If a user-side key is exposed, rotate the affected key and preserve a redacted incident record.
- If provider sync corrupts legacy entries, disable managed provider sync and restore legacy settings from snapshot.

## Known Limits

- No old-version sample settings were provided.
- No runnable desktop environment was provided.
- No rollback command was executed by this replacement worker lane.
- All rollback verification remains compatibility evidence pending.
- Final compatibility evidence must be generated with `tools/new-07-compatibility-evidence.ps1` or an equivalent 8-file structure, optionally filled with provider snapshot comparisons from `tools/inspect-07-compatibility-snapshots.ps1`, then pass `tools/verify-07-compatibility-evidence.ps1`.
