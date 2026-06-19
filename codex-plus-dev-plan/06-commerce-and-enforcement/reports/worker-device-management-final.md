Report status: final
Worker lane: Device
Forbidden edits: none

## Changed files

- `sub2api-main/backend/internal/service/codexplus_foundation.go`
- `sub2api-main/backend/internal/service/codexplus_device_management.go`
- `sub2api-main/backend/internal/service/codexplus_device_management_test.go`
- `sub2api-main/backend/internal/repository/codexplus_foundation_repo.go`
- `sub2api-main/backend/internal/repository/codexplus_foundation_repo_test.go`
- `sub2api-main/backend/internal/handler/admin/codexplus_handler.go`
- `sub2api-main/backend/internal/server/routes/codexplus_admin.go`
- `codex-plus-dev-plan/06-commerce-and-enforcement/reports/worker-device-management-final.md`

## Implementation

- Added admin device management service methods for listing, revoking, and restoring Codex++ devices.
- Added admin device DTO fields for user, device id, status, last seen time, revoked time, platform, app version, and Codex version metadata.
- Added repository support for:
  - listing devices by user and optional statuses;
  - setting device status with `RETURNING` of the updated row;
  - mapping missing devices to `CODEXPLUS_DEVICE_NOT_FOUND`.
- Added admin endpoints:
  - `GET /admin/codex-plus/users/:id/devices`
  - `POST /admin/codex-plus/users/:id/devices/:device_id/revoke`
  - `POST /admin/codex-plus/users/:id/devices/:device_id/restore`
- Revoke sets status to `revoked` and records `revoked_at`.
- Restore sets status to `active` and clears `revoked_at`.
- Admin write handlers require an admin actor from context and reject missing admin context with 403.
- Admin revoke/restore emit best-effort Codex++ events when the event repository is available.
- Existing bootstrap behavior already consumes device status through `CodexPlusClientService.loadDevice`; tests now cover both revoked suppression and restored/active availability.
- Existing gateway policy service already reads the device repository and rejects `revoked` / `blocked` devices when policy evaluation compiles.

## Verification

- Ran `gofmt -w` on all changed Go files.
- Ran `gofmt -l` on changed Go files: no output.
- Passed:
  - `go test ./internal/service -run "TestCodexPlusAdminDeviceManagement|TestCodexPlusClientBootstrapRevokedDeviceSuppressesKey|TestCodexPlusClientBootstrapRestoredDeviceIsAvailable"`
  - `go test ./internal/repository -run "TestCodexPlusDevice"`
- Broader targeted test command was attempted:
  - `go test ./internal/repository ./internal/service ./internal/handler/admin ./internal/server/routes -run "CodexPlus|DeviceManagement"`
  - Blocked by concurrent non-Device compile errors in `internal/service/codexplus_gateway_policy_service.go`: missing `resolveCodexPlusGatewayPolicy`, `checkUsagePolicy`, and `allow`.
- Handler permission/parameter behavior is implemented in `codexplus_handler.go`, but handler `_test.go` files are outside this worker's declared write scope, so dedicated handler tests were not added.

## Coordinator follow-up

- Resolve concurrent gateway-policy compile errors before rerunning full targeted tests.
- Decide whether Device lane may add handler admin tests, or have coordinator/handler owner add coverage for:
  - invalid user id;
  - invalid device id;
  - missing admin actor returns 403;
  - revoke/restore request body strict JSON behavior.
- `ClientUsage` still uses an always-active device snapshot in `internal/service/codexplus_client.go`; changing it to load `input.DeviceID` is outside this worker's write scope and should be handled by coordinator or the client API owner.
- After the gateway-policy compile blocker is fixed, rerun:
  - `go test ./internal/repository ./internal/service ./internal/handler/admin ./internal/server/routes -run "CodexPlus|DeviceManagement"`

## Remaining risks

- Admin UI wiring is not included in this worker lane.
- Gateway device enforcement depends on the gateway-policy lane compiling successfully.
- Usage endpoint device-revoked behavior is not fully enforced until the `codexplus_client.go` follow-up is applied.
