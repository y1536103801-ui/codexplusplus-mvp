# Package Pre-Release Blockers And Risks

Worker lane: Package
Date: 2026-06-17
Status: Windows package evidence partially complete; Windows-only MVP package evidence still blocked

## 2026-06-19 Windows-Only MVP Scope Update

Owner decision: MVP package scope is Windows x64 only. macOS x64 and macOS arm64 package evidence is deferred post-MVP and must not block Windows-only MVP readiness. Full cross-platform release remains blocked until macOS artifacts and install evidence exist.

Current Windows package baseline:

- Windows setup artifact exists at `CodexPlusPlus-main/dist/windows/CodexPlusPlus-1.2.9-windows-x64-setup.exe`.
- Windows setup SHA256: `20ebf88d1bd2ebeca903ffedf39fdd175c9b8edfdc6fbca7e8764b5278a031ac`.
- Windows clean install, overwrite install, uninstall, and reinstall have command-level evidence.
- Windows desktop shortcuts, Start Menu entries, Apps and Features metadata, and silent launcher behavior have been fixed and verified.
- Windows package evidence still lacks page-level Manager UI proof for login, install assistant, diagnostics, and advanced configuration.
- Windows package evidence still lacks isolated Missing-Codex first-run assistant proof.
- `verify-07-package-evidence.ps1 -WindowsOnlyMvp` is the Windows-only MVP package verifier mode; the default verifier remains the full cross-platform gate.

Windows evidence must use explicit pass lines in `01-windows-x64-install.md`:

- `Manager login: pass`
- `Manager install assistant: pass`
- `Manager diagnostics: pass`
- `Manager advanced configuration: pass`
- `Missing-Codex first-run: pass`

## Blocking For Package Signoff

- Windows-only MVP package signoff is blocked by missing page-level Manager UI proof.
- Windows-only MVP package signoff is blocked by missing isolated Missing-Codex first-run assistant proof.
- Full cross-platform package signoff is additionally blocked by missing macOS x64 and macOS arm64 DMG artifacts and install evidence.

## Important Static Risks

- `src-tauri/tauri.conf.json` has `bundle.active: false`; package release depends on custom workflows and installer scripts, not Tauri bundle output.
- Windows NSIS uses `RequestExecutionLevel admin` while the install dir is under `$LOCALAPPDATA`; clean Windows install testing should confirm UAC behavior and HKCU registry writes under elevated context.
- NSIS contains cleanup for mojibake legacy shortcut names (`Codex++ 绠＄悊宸ュ叿.lnk`), which is useful for legacy cleanup but should be checked on localized Windows desktops.
- Rust Windows entrypoint registration uses uninstall key `CodexPlusPlus`, while NSIS writes `Codex++`; release testing should confirm final shipped installer path and uninstall entry naming are intentional.
- macOS script signs ad-hoc and README notes unsigned/unnotarized Gatekeeper handling. Release readiness should explicitly decide whether ad-hoc unsigned DMGs are acceptable for this release.
- Local Node is v20.19.0, while workflows use Node 22; CI remains the authoritative package build environment.
- Workspace-local Rust targeted checks passed after the original package worker report, but full release build and installer artifact tests remain outstanding.
- Full Rust workspace testing was attempted with the workspace-local GNU toolchain and local `w64devkit`; it did not reach a pass/fail test result because linking/writing failed when the host ran out of disk space. Generated `target` artifacts were cleaned afterward.

## Package Evidence Required Before Marking Passed

For Windows-only MVP package signoff, only the Windows x64 setup artifact and Windows install evidence are in scope. The macOS items below remain required for full cross-platform package signoff.

- CI artifact or release asset names:
  - `CodexPlusPlus-<version>-windows-x64-setup.exe`
  - `CodexPlusPlus-<version>-macos-x64.dmg`
  - `CodexPlusPlus-<version>-macos-arm64.dmg`
- Windows install record:
  - clean install
  - desktop shortcuts
  - Start Menu shortcuts
  - silent launcher starts Codex without Manager UI
  - Manager opens login, install assistant, diagnostics, and advanced configuration
  - missing Codex first-run assistant path
  - overwrite install
  - uninstall
  - reinstall after uninstall
- macOS install record for both x64 and arm64:
  - DMG mounts
  - both apps are present
  - both apps copy or open from `/Applications`
  - silent app hides Dock icon and launches Codex path
  - Manager app opens UI
  - missing Codex first-run assistant path
  - overwrite install by replacing apps
  - uninstall by removing apps
  - reinstall after removal
- Artifact inspection:
  - no shared Key
  - no packaged user credential
  - no embedded price, plan, or fixed model policy
  - install scripts do not write Codex credentials

## 2026-06-18 Worker 2B Blocker Confirmation

Current package evidence folder: `codex-plus-dev-plan/test-runs/20260618-2103-package`.

Confirmed blockers:

- Windows artifact blocker: no `CodexPlusPlus-<version>-windows-x64-setup.exe` was found or generated.
- Windows build blocker: `npm run build` failed because `cargo` is not on PATH.
- Windows installer blocker: `makensis` is not on PATH, so NSIS cannot build the setup exe on this host even after Rust is supplied.
- macOS x64 blocker: current host is Windows and has no macOS DMG tooling or Intel macOS install surface.
- macOS arm64 blocker: current host is Windows and has no Apple Silicon macOS install surface.
- Artifact inspection blocker: `CodexPlusPlus-main/dist` lacks all three required distribution artifacts.
- Package verifier blocker: `verify-07-package-evidence.ps1` fails by design until metadata, platform install files, artifact inspection, and package report all contain real pass evidence.

Minimum unblock set:

- Windows packaging host or CI run with Node 22, Rust stable, NSIS, and enough disk to run frontend build, Rust release build, staging, and `makensis`.
- macOS x64 host or `macos-15-intel` CI run with Node 22, Rust stable target `x86_64-apple-darwin`, and DMG tools.
- macOS arm64 host or `macos-14` CI run with Node 22, Rust stable target `aarch64-apple-darwin`, and DMG tools.
- Real install/open/overwrite/uninstall/reinstall evidence for Windows, macOS x64, and macOS arm64.
- Owner decision on whether unsigned or unnotarized macOS artifacts are acceptable for MVP.

## 2026-06-18 Worker 4A Windows Package Unblock

Current package-unblock evidence folder: `codex-plus-dev-plan/test-runs/20260618-2201-package-unblock`.

Confirmed current host state:

- Node/npm/npx/corepack/corepack pnpm and `npx tauri` are available.
- `rustc`, `cargo`, `rustup`, `cl`, `link`, and `makensis` are missing on PATH.
- `vswhere` exists, but no Visual Studio installation with `Microsoft.VisualStudio.Component.VC.Tools.x86.x64` was detected.
- No Windows SDK root was detected under the checked Windows Kits paths.
- Local executable search found no workspace-local Rust/Cargo, NSIS, MSVC, or VS developer command prompt executable in the checked user, Program Files, and workspace paths.
- `CodexPlusPlus-main/dist` does not exist; no Windows setup artifact is present.

Build attempt:

- `npm run build` from `CodexPlusPlus-main/apps/codex-plus-manager` failed with exit code 1.
- Exact blocker: `'cargo' is not recognized as an internal or external command, operable program or batch file.`
- No Windows installer artifact or SHA256 was generated.

Minimum Windows unblock commands, pending owner approval before execution:

```powershell
winget install --id Microsoft.VisualStudio.2022.BuildTools -e --source winget --override "--passive --wait --add Microsoft.VisualStudio.Workload.VCTools --add Microsoft.VisualStudio.Component.Windows11SDK.26100 --includeRecommended"
winget install --id Rustlang.Rustup -e --source winget --scope user
winget install --id NSIS.NSIS -e --source winget
```

After installation, open a new PowerShell session and verify `rustup default stable-x86_64-pc-windows-msvc`, `rustc --version`, `cargo --version`, `cl /?`, and `makensis /VERSION`, then rerun the CI-equivalent package recipe.

## 2026-06-18 Toolchain Install Follow-up

Evidence folder: `codex-plus-dev-plan/test-runs/20260618-2245-package-unblock`.

The Windows package toolchain was installed after user approval:

- `rustup`: installed, `rustup 1.29.0`.
- `rustc`: installed, `rustc 1.96.0`.
- `cargo`: installed, `cargo 1.96.0`.
- `makensis`: installed at `C:\Program Files (x86)\NSIS\makensis.exe`, version `v3.12`.
- MSVC C++ tools: installed under `C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools`.
- `cl`: available through `VsDevCmd.bat`, compiler version `19.44.35228`.
- `link`: available through `VsDevCmd.bat`, linker version `14.44.35228.0`.

The previous host-level toolchain blocker is resolved. Package evidence is still blocked until a Windows installer is built, hashed, inspected, and install-tested.

## 2026-06-18 Windows Build Retry Follow-up

Evidence folders:

- Toolchain/build retry: `codex-plus-dev-plan/test-runs/20260618-2245-package-unblock`
- Package artifact inspection: `codex-plus-dev-plan/test-runs/20260618-2306-package`

Resolved in this follow-up:

- Manager frontend build passed.
- Rust MSVC release build passed.
- NSIS Windows setup build passed.
- Windows setup artifact exists at `CodexPlusPlus-main/dist/windows/CodexPlusPlus-1.2.9-windows-x64-setup.exe`.
- Windows setup SHA256: `20ebf88d1bd2ebeca903ffedf39fdd175c9b8edfdc6fbca7e8764b5278a031ac`.

Still blocked:

- Windows command-level fresh install, overwrite install, uninstall, and reinstall evidence has now been run.
- Windows desktop shortcuts were fixed and verified on the current user desktop.
- Windows silent launcher was fixed and verified: launching `codex-plus-plus.exe` no longer started `codex-plus-plus-manager.exe`.
- Windows package evidence still lacks page-level Manager UI proof and isolated Missing-Codex first-run proof.
- macOS x64 and macOS arm64 DMGs are still absent.
- Artifact inspector still fails because macOS coverage is missing.
- Artifact scanner precision was fixed so complete API-key-like, JWT-like, and Authorization token shapes are scanned instead of broad `sk-` / `eyJ` prefixes; current scanner findings are none.
