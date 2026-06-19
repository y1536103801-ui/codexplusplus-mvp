# 03 Admin Setup

Run folder: 20260619-1940-e2e
Status: pass

Result: pass

## Admin Configuration Evidence

- Test plan: local `20260619-1940-e2e` runner executed client API, browser handoff, gateway policy, and admin audit subsets.
- Model policy: allowed model path succeeded; denied model path returned `GATEWAY_POLICY_MODEL_NOT_ALLOWED`.
- Default model: Manager UI showed `Codex Standard`; local gateway allowed configured test model.
- Usage policy: active user succeeded; not purchased, expired, low balance, revoked device, and denied model scenarios returned expected structured rejections.
- Feature flags/config: config version `codexplus-mvp-1`, snapshot version `snap_20260619_114119_available`.
- Admin-visible entitlement/device/managed-key summaries: admin audit returned safe event fields and matched gateway `request_id` values; secrets were not printed.
