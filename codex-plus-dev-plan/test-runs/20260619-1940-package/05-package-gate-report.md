# 05 Package Gate Report

Run folder: 20260618-2306-package
Status: executed

Package evidence result: pass

## Commands Executed

- `npm install --package-lock=false` in `CodexPlusPlus-main/apps/codex-plus-manager`.
- `npm run vite:build` in `CodexPlusPlus-main/apps/codex-plus-manager`.
- `rustup default stable-x86_64-pc-windows-msvc`.
- `cargo --config "http.proxy=''" build --release` in `CodexPlusPlus-main` through `VsDevCmd.bat`.
- Copied `codex-plus-plus.exe` and `codex-plus-plus-manager.exe` from `target/release` into `CodexPlusPlus-main/dist/windows/app`.
- `makensis /INPUTCHARSET UTF8 /DVERSION=1.2.9 CodexPlusPlus.nsi`.
- `inspect-07-package-artifacts.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/20260618-2306-package -ArtifactDir codex-plus-dev-plan/test-runs/20260618-2306-package/_artifact-input`.

## Evidence Links

- Artifact metadata: `00-artifact-metadata.md`.
- Windows install evidence: `01-windows-x64-install.md`.
- macOS x64 evidence gap: `02-macos-x64-dmg.md`.
- macOS arm64 evidence gap: `03-macos-arm64-dmg.md`.
- Artifact inspection: `04-artifact-inspection.md`.
- Windows-only MVP scope decision: `06-mvp-scope-decision.md`.
- Windows setup artifact: `CodexPlusPlus-main/dist/windows/CodexPlusPlus-1.2.9-windows-x64-setup.exe`.

## MVP Scope Decision

- 2026-06-19 owner decision: MVP package scope is Windows x64 only.
- macOS x64 and macOS arm64 package evidence is deferred post-MVP and is not a blocker for Windows-only MVP readiness.
- The default full cross-platform package verifier remains strict and must still fail until macOS evidence exists.

## Remaining Risks

- Windows setup artifact exists and command-level fresh install, overwrite install, uninstall, and reinstall were run.
- Windows desktop shortcuts now pass after the NSIS `%USERPROFILE%\Desktop` fallback fix.
- Windows silent launcher now passes after the launcher update-check behavior fix; starting `codex-plus-plus.exe` did not start Manager.
- Windows page-level Manager evidence is present for login, install/maintenance assistant, diagnostics, and advanced provider configuration.
- Isolated Missing-Codex first-run evidence is present under a clean profile rooted inside this evidence folder.
- Direct non-elevated Manager launch still requires elevation because the installed Manager manifest requests administrator privileges; page evidence used `RunAsInvoker` only to allow WebView2 automation of the installed Manager UI.
- CDP DOM text for Chinese labels is encoding-skewed in some `.txt` captures; PNG screenshots are the primary human-readable page evidence.
- macOS x64 and arm64 DMG artifacts are absent and deferred post-MVP by owner Windows-only scope decision.
- macOS Gatekeeper and unsigned or unnotarized distribution behavior remains owner-controlled and unverified for post-MVP platform expansion.

## Windows Evidence Format Required To Pass

`01-windows-x64-install.md` contains explicit pass lines for each formerly remaining Windows surface:

- `Manager login: pass`
- `Manager install assistant: pass`
- `Manager diagnostics: pass`
- `Manager advanced configuration: pass`
- `Missing-Codex first-run: pass`

Those surfaces are now backed by screenshots and CDP captures under `windows-ui-evidence/`, so `01-windows-x64-install.md` is `Result: pass` and this report is `Package evidence result: pass`.

## Scanner Precision Fix

- Artifact inspection now uses complete API-key-like, JWT-like, and Authorization token shapes instead of broad `sk-` / `eyJ` prefixes. `04-artifact-inspection.md` reports no scanner findings; token values remain intentionally unprinted.

## Windows Installer Fixes

- `CodexPlusPlus.nsi` now writes and removes current-user desktop shortcuts through `%USERPROFILE%\Desktop`.
- `codex-plus-launcher` no longer opens Manager automatically when a background update check finds a newer release; it records `launcher.update_available` diagnostic data instead.
- Rebuilt setup SHA256: `20ebf88d1bd2ebeca903ffedf39fdd175c9b8edfdc6fbca7e8764b5278a031ac`.

## Release Boundary

- Package evidence is ready for Windows-only MVP Module J package status. This does not waive deferred post-MVP macOS package evidence.
- This report does not override E2E, compatibility, business, or release go/no-go gates.
