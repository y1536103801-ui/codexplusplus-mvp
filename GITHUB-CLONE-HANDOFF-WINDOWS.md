# GitHub Clone Handoff For Codex++

Created: 2026-06-19

This repository is intended for continuing Codex++ MVP development from another Windows PC.

## Repository Scope

Included:

- Codex++ desktop source
- Sub2API backend/admin source
- local development env file: `sub2api-main/deploy/.env.codexplus-local`
- final Windows-only MVP evidence: `codex-plus-dev-plan/test-runs/20260619-1940-*`
- small handoff binaries under `_handoff-artifacts/windows`
- clone/start helper: `CLONE-RUN-WINDOWS.ps1`

Excluded:

- `node_modules/`
- Rust `target/`
- Docker bind data and local databases
- WebView/Edge profiles
- `.codex` profile and auth snapshots
- generated local E2E token files

This keeps the repo cloneable and avoids GitHub file-size limits.

## New PC Prerequisites

Minimum to run backend + Manager:

- Docker Desktop
- PowerShell
- Microsoft Edge WebView2 Runtime

For active development:

- Git
- Node.js/npm
- pnpm
- Go
- Rust/MSVC build tools
- NSIS for installer rebuilds

## Clone And Run

```powershell
git clone https://github.com/y1536103801-ui/codexplusplus-mvp.git
cd codexplusplus-mvp
powershell -ExecutionPolicy Bypass -File .\CLONE-RUN-WINDOWS.ps1 -ReplaceExisting
```

The script will:

1. create/use `sub2api-main/deploy/.env.codexplus-local`,
2. build and start the local Sub2API stack with Docker Compose,
3. wait for `http://127.0.0.1:8081/health`,
4. open Codex++ Manager from `_handoff-artifacts/windows`.

## Rebuild Manager From Source

```powershell
cd CodexPlusPlus-main\apps\codex-plus-manager
npm install
npm run check

cd ..\..
cargo build -p codex-plus-manager --release --bin codex-plus-plus-manager
```

## Rebuild Sub2API Admin Frontend

```powershell
cd sub2api-main\frontend
pnpm install
pnpm build
```

## Current MVP Evidence

Final evidence folder:

```text
codex-plus-dev-plan/test-runs/20260619-1940-release
```

Final recommendation:

```text
go with accepted risks
```

Scope:

```text
Windows-only local MVP. Production release remains separately gated.
```

