# Codex++ Windows MVP Development Handoff

Created: 2026-06-19

This package is for continuing development on another Windows PC. It is not a production release package.

## Current Status

- MVP scope: Windows-only local MVP.
- Final release evidence stamp: `20260619-1940`.
- Final recommendation: `go with accepted risks` for the owner-approved local Windows MVP gate.
- Production release, public paid traffic, real customer rollout and macOS packaging remain separate post-MVP gates.

## Important Paths

- Desktop app source: `CodexPlusPlus-main`
- Backend/admin source: `sub2api-main`
- Planning, stage gates and evidence: `codex-plus-dev-plan`
- Final release evidence: `codex-plus-dev-plan/test-runs/20260619-1940-release`
- Windows package evidence: `codex-plus-dev-plan/test-runs/20260619-1940-package`
- Product spec: `codex-plus-product-spec.html`

## Included Binary Artifacts

The migration package may include `_handoff-artifacts/windows/` with:

- latest locally rebuilt Manager executable
- Windows x64 setup artifact used by the package evidence lane

These artifacts are for reference and smoke testing. For continued development, rebuild from source on the new PC.

## Excluded From The Migration Package

The package intentionally excludes generated or machine-local files:

- Rust `target/`
- frontend `node_modules/`
- large generated build outputs that are not required for source handoff
- Docker local data volumes
- browser/WebView test profiles
- scratch logs
- local auth files such as `auth.json`
- local env files that may contain credentials

Regenerate dependencies and local runtime state on the new PC.

## New PC Prerequisites

Install these before resuming development:

- Windows 10/11
- PowerShell
- Docker Desktop
- Node.js with npm
- pnpm for `sub2api-main/frontend`
- Go toolchain matching the backend module requirements
- Rust toolchain with Windows/MSVC build tools
- NSIS if you need to rebuild the Windows installer

## Restore And Start Local Backend

From the extracted project root:

```powershell
cd '<extracted-root>'
powershell -ExecutionPolicy Bypass -File sub2api-main\tools\e2e\codexplus\start-local-dev-compose.ps1 -InitEnv
powershell -ExecutionPolicy Bypass -File sub2api-main\tools\e2e\codexplus\start-local-dev-compose.ps1
```

Then verify:

```powershell
Invoke-RestMethod http://127.0.0.1:8081/health
```

Expected result:

```json
{"status":"ok"}
```

## Reinstall Frontend Dependencies

Manager:

```powershell
cd '<extracted-root>\CodexPlusPlus-main\apps\codex-plus-manager'
npm install
npm run check
```

Sub2API admin frontend:

```powershell
cd '<extracted-root>\sub2api-main\frontend'
pnpm install
pnpm build
```

## Rebuild Desktop Manager

```powershell
cd '<extracted-root>\CodexPlusPlus-main'
cargo build -p codex-plus-manager --release --bin codex-plus-plus-manager
```

Output:

```text
CodexPlusPlus-main\target\release\codex-plus-plus-manager.exe
```

## Local MVP Verification Commands

From the extracted project root:

```powershell
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-07-static.ps1
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\validate-stage-gate.ps1 -Stage 07-integration-release
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan\tools\verify-07-release-handoff.ps1 -Root '<extracted-root>' -ReleaseDir codex-plus-dev-plan/test-runs/20260619-1940-release -WindowsOnlyMvp -AllowGoCandidate
```

## Next Development Priorities

Before giving the product to a non-technical Windows customer, finish these items:

1. Make the main Manager action a clear `登录 Codex++ Cloud` button.
2. Default the service address so normal users do not type `http://127.0.0.1:8081`.
3. Rebuild the Windows installer using the latest Manager binary.
4. Re-run the Windows package install evidence after the installer rebuild.
5. Run one no-command-line customer path: install, login, auto-configure, launch Codex, send one request.
