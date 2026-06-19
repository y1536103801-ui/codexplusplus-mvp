# 01 Test Accounts

Run folder: 20260619-1940-e2e
Status: pass

## Sanitized Account Matrix

| Scenario | Expected entitlement/device state | Result |
| --- | --- | --- |
| admin_test | local admin audit token accepted; safe fields only | pass |
| user_active | service available, device active, model allowed | pass |
| user_not_purchased | entitlement rejected with structured policy code | pass |
| user_expired | expired entitlement rejected with structured policy code | pass |
| user_low_balance | quota/balance rejection returned with request id | pass |
| user_device_revoked | revoked device rejected with structured policy code | pass |
| user_model_denied | unauthorized model rejected with structured policy code | pass |

## Notes

- Test accounts are seeded local test accounts only.
- The visible local account identity is `codexplus-e2e-active@local.test`; no production user identity is used.
- Missing setup blockers: none for the Windows-only local MVP run.
