# Codex++ Windows MVP Handoff

This repository contains the Codex++ Windows-only local MVP development workspace.

## Clone On Another Windows PC

```powershell
git clone https://github.com/y1536103801-ui/codexplusplus-mvp.git
cd codexplusplus-mvp
powershell -ExecutionPolicy Bypass -File .\CLONE-RUN-WINDOWS.ps1 -ReplaceExisting
```

The startup script builds and starts the local Sub2API backend with Docker Compose, waits for `http://127.0.0.1:8081/health`, then opens Codex++ Manager from `_handoff-artifacts/windows`.

## Required Runtime

- Windows 10/11
- Docker Desktop
- PowerShell
- Microsoft Edge WebView2 Runtime

For active development:

- Node.js/npm
- pnpm
- Go
- Rust/MSVC build tools
- NSIS

## Handoff Docs

- `GITHUB-CLONE-HANDOFF-WINDOWS.md`
- `FULL-PORTABLE-HANDOFF-WINDOWS.md`
- `DEV-HANDOFF-WINDOWS-MVP.md`

## Current MVP Status

- Scope: Windows-only local MVP
- Final evidence stamp: `20260619-1940`
- Final recommendation: `go with accepted risks`
- Production release remains separately gated.

