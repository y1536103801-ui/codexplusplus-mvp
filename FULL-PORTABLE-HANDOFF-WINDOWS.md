# Codex++ Full Portable Windows Handoff

Created: 2026-06-19

This full handoff package is intended for moving the current local development state to another personal Windows PC with minimal setup.

## What This Full Package Keeps

- source code
- `target/` Rust build outputs
- frontend `node_modules/`
- `.env` and local E2E env files
- Docker bind data under `sub2api-main/deploy/.codexplus-local`
- current Docker images exported under `_portable-env/docker-images`
- installed Codex++ app files under `_portable-env/installed-codexpp`
- optional current Codex profile snapshot under `_portable-env/codex-home`
- final MVP evidence under `codex-plus-dev-plan/test-runs`

## What May Still Be Needed On The New PC

The package cannot replace Windows system services. The new PC should have:

- Docker Desktop
- Microsoft Edge WebView2 Runtime
- PowerShell

For continued development instead of just running:

- Node.js/npm
- pnpm
- Go
- Rust/MSVC build tools
- NSIS

## First Run On The New PC

Extract the full package, then run:

```powershell
cd '<extracted-root>\codex+++'
powershell -ExecutionPolicy Bypass -File .\PORTABLE-RUN-WINDOWS.ps1
```

The script will:

1. load bundled Docker images if missing,
2. start the local Sub2API Codex++ backend with `docker compose`,
3. wait for `http://127.0.0.1:8081/health`,
4. open Codex++ Manager.

## Optional Codex Profile Restore

Only run this if you want the new PC to use the copied `.codex` profile snapshot:

```powershell
cd '<extracted-root>\codex+++'
powershell -ExecutionPolicy Bypass -File .\RESTORE-CODEX-PROFILE-OPTIONAL.ps1
```

The script backs up an existing `%USERPROFILE%\.codex` before copying the portable profile. Use `-Force` only if you intentionally want to replace the target profile.

## Local URLs

- Backend health: `http://127.0.0.1:8081/health`
- Manager expected service address: `http://127.0.0.1:8081`

## Current MVP State

- Windows-only local MVP gate: passed with accepted risks.
- Final release evidence stamp: `20260619-1940`.
- Final release evidence folder: `codex-plus-dev-plan/test-runs/20260619-1940-release`.
- Production release remains a separate gate.

