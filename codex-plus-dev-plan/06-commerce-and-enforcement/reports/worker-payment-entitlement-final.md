Report status: final
Worker lane: Payment
Forbidden edits: none

## Changed files
- `sub2api-main/backend/internal/service/codexplus_commerce_entitlement.go`
- `sub2api-main/backend/internal/service/payment_fulfillment.go`
- `sub2api-main/backend/internal/service/codexplus_commerce_entitlement_test.go`
- `codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-payment-entitlement-final.md`

## Implementation
- Added `CodexPlusCommerceEntitlementService` for payment fulfillment to resolve subscription orders into Codex++ entitlements using payment order fields, subscription plan fallback, subscription group binding, and PlanCatalog entitlement sources.
- Added Codex++ commerce audit events for granted, already-granted, unmapped, and config-error cases.
- Updated subscription payment fulfillment so successful paid/recovered orders attach a Codex++ entitlement note before assigning/extending a subscription and record the entitlement grant after assignment.
- Hardened idempotency for retry paths by checking both audit logs and subscription notes before assigning/extending, covering the window where subscription mutation succeeds but completion/audit recording is retried.
- Verified that existing bootstrap/usage behavior reflects the new state after fulfillment through active `UserSubscription` records matched by PlanCatalog group entitlement.

## Verification
- Ran `gofmt` on changed Go files.
- Passed targeted tests:
  - `go test -tags unit ./internal/service -run "TestCodexPlusCommerce|TestPaymentSubscriptionFulfillmentSkipsAlreadyGrantedCodexPlusOrder|TestPaymentExpiredGraceOrderGrantsCodexPlusEntitlementAndRefreshesClientState|TestHandlePaymentNotification|Test.*Fulfillment"`
- Added tests for:
  - payment plan/group binding to Codex++ PlanCatalog entitlement
  - repeat subscription fulfillment skipping an already-granted order
  - expired-within-grace payment recovery granting entitlement and making bootstrap/usage available

## Coordinator follow-up
- Refund/compensation wiring remains follow-up because `payment_refund.go` is outside this worker write scope. Coordinator should wire subscription refund deduction/compensation into an idempotent Codex++ audit path or approve a scoped Payment worker edit there.
- API-key/gateway auth cache invalidation was not wired because `PaymentService` currently has no direct `APIKeyAuthCacheInvalidator` dependency. Subscription caches are invalidated by `SubscriptionService`; coordinator should expose/route an auth cache invalidator if immediate gateway auth refresh is required.
- Legacy subscription orders missing `subscription_group_id`/`subscription_days` still fail the existing `ExecuteSubscriptionFulfillment` precheck before PlanCatalog fallback can help. Current order creation writes these fields; coordinator should decide whether legacy repair is needed.

## Remaining risks
- Codex++ config load errors and unmapped PlanCatalog bindings are fail-open for legacy subscription fulfillment and are recorded as audits rather than blocking payment completion.
- Verification used targeted unit tests with sqlite/enttest; full backend suite was not run.
