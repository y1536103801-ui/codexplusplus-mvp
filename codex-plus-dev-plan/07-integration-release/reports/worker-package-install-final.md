# Package Install Final Report

Report status: final
Worker lane: Package
Forbidden edits: none
Date: 2026-06-17

## Summary

This lane produced the package install checklist, local command evidence, and pre-release blocker/risk record for the `07-integration-release` package install check. No installer package was built or installed in this lane, because the worker environment had no Rust toolchain or NSIS in PATH and was not a macOS packaging host. Coordinator follow-up later found a workspace-local Rust toolchain and passed targeted desktop Rust checks, but package status is still `package evidence pending`, not passed.

2026-06-18 template sync note: the package evidence template now reflects the latest 07 verifier requirements for package metadata `Result: pass`, three-platform Expected Artifact Coverage, artifact inspection `Result: pass`, recorded `inspect-07-package-artifacts.ps1` command, clear high-confidence scanner findings, no shared key/user credentials/fixed commercial policy, installer-script credential scan pass, and the manual-provider preservation boundary. This is a documentation sync only and does not claim real package evidence has passed.

## Changed files

- `codex-plus-dev-plan/07-integration-release/package-install/package-install-checklist.md`
- `codex-plus-dev-plan/07-integration-release/package-install/local-command-evidence.md`
- `codex-plus-dev-plan/07-integration-release/package-install/platform-evidence-template.md`
- `codex-plus-dev-plan/07-integration-release/package-install/pre-release-blockers.md`
- `codex-plus-dev-plan/tools/new-07-package-evidence.ps1`
- `codex-plus-dev-plan/tools/verify-07-package-evidence.ps1`
- `codex-plus-dev-plan/07-integration-release/reports/worker-package-install-final.md`

## Verification

- Read required stage docs:
  - `07-integration-release/README.md`
  - `07-integration-release/task-package-install-check.md`
  - `INTEGRATION-VERIFICATION-CHECKLIST.md`
- Read package/build/release sources without editing manifests:
  - `CodexPlusPlus-main` README files, package manifests, Tauri config, NSIS script, macOS DMG script, release and PR workflows, install modules, installer tests, launcher entry, Manager install assistant.
- Confirmed available npm scripts with `npm run` in `apps/codex-plus-manager`.
- Ran `npm run check` in `apps/codex-plus-manager`: passed.
- Checked local tool availability:
  - `node --version`: `v20.19.0`
  - `npm --version`: `10.8.2` printed before probe timeout
  - `npx tsc --version`: `Version 5.9.3`
  - `npx vite --version`: `vite/6.4.2 win32-x64 node-v20.19.0`
  - `npx tauri --version`: `tauri-cli 2.11.2`
  - `cargo --version`: failed, command not found
  - `makensis`: not found
- Confirmed no local `CodexPlusPlus-main/dist` directory and no local `target/release` directory were present.
- Coordinator follow-up found a workspace-local Rust toolchain outside PATH and passed targeted desktop Rust checks:
  - `cargo fmt --check -p codex-plus-core`
  - `cargo test -p codex-plus-core codexplus_cloud`
  - `cargo test -p codex-plus-core relay_config`
  - `cargo test -p codex-plus-core protocol_proxy`

## Package command evidence

- Manifest local build command: `npm run build` from `apps/codex-plus-manager`, mapped to `cargo build -p codex-plus-launcher --release && tauri build`.
- CI Windows package path: install deps, `npm run vite:build`, `cargo build --release`, stage `target/release/codex-plus-plus.exe` and `codex-plus-plus-manager.exe`, then run `makensis /INPUTCHARSET UTF8 /DVERSION=<version> CodexPlusPlus.nsi`.
- CI macOS package path: install deps, `npm run vite:build`, `cargo build --release --target <mac-target>`, then `BINARY_DIR=<target-release> bash scripts/installer/macos/package-dmg.sh <version> <x64|arm64>`.
- Tauri config has `bundle.active: false`; release packaging depends on custom NSIS and DMG scripts.

## Checklist coverage

The generated checklist covers:

- Windows desktop entry.
- Windows Start Menu entry.
- macOS x64 DMG.
- macOS arm64 DMG.
- Silent launcher behavior.
- Manager entry behavior.
- Missing-Codex first-run/install assistant path.
- Overwrite install.
- Uninstall and reinstall.
- Package exclusion of shared Key, user credentials, price, plan, or fixed model policy.

## Remaining risks

- Package evidence pending: no Windows installer or macOS DMG was built or installed in this lane.
- Windows clean install, overwrite install, uninstall, and reinstall still need real machine evidence.
- macOS x64 and arm64 DMGs still need CI logs and manual mount/install/launch evidence on matching hardware or runners.
- Missing-Codex first-run assistant still needs clean-machine UI evidence.
- PATH `cargo`/`rustc` and `makensis` remain unavailable; coordinator direct-toolchain Rust tests passed, but Rust release builds and NSIS installer generation were not executed.
- macOS Gatekeeper/signing behavior remains unverified; scripts use ad-hoc signing and README documents unsigned/unnotarized handling.
- Package evidence verifier now exists, but no real timestamped package evidence folder has passed it yet.
- Latest package artifact inspection tooling can fill `00-artifact-metadata.md` and `04-artifact-inspection.md`, but real Windows/macOS platform install evidence is still required before Module J can treat the package lane as release-ready.

## 2026-06-18 Worker 2B Current Run Addendum

Current evidence folder: `codex-plus-dev-plan/test-runs/20260618-2103-package`.

Files produced or updated in the evidence folder:

- `00-artifact-metadata.md`
- `01-windows-x64-install.md`
- `02-macos-x64-dmg.md`
- `03-macos-arm64-dmg.md`
- `04-artifact-inspection.md`
- `05-package-gate-report.md`
- `host-prerequisites.json`
- `host-prerequisites.txt`
- `inspect-default-artifacts.log`
- `windows-package-build-npm-run-build.log`
- `tauri-cli-preflight.log`
- `verify-package-evidence.log`

Current command results:

- `inspect-07-package-artifacts.ps1`: failed because all three required artifacts are missing under `CodexPlusPlus-main/dist`.
- `npm run build`: failed because `cargo` is not available from PATH.
- `npm exec tauri -- --version`: passed, `tauri-cli 2.11.2`.
- `verify-07-package-evidence.ps1`: failed with 12 real gate failures for missing artifact coverage and `Result: fail` platform evidence.

Current package lane status: blocked. No Windows or macOS package artifact was generated in this run, and no install/open/overwrite/uninstall/reinstall evidence was produced.

## 2026-06-18 Worker 4A Package Unblock Addendum

Current evidence folder: `codex-plus-dev-plan/test-runs/20260618-2201-package-unblock`.

Files produced in the package-unblock evidence folder:

- `package-unblock-report.md`
- `host-tool-version-checks.md`
- `host-tool-version-checks.json`
- `host-toolchain-status.md`
- `host-toolchain-status.json`
- `local-tool-executable-search.txt`
- `manager-packaging-script-summary.md`
- `manager-packaging-script-summary.json`
- `workspace-packaging-files.txt`
- `workflow-files.txt`
- `release-assets-yml.snapshot.yml`
- `pr-build-yml.snapshot.yml`
- `manager-package-json.snapshot.json`
- `tauri-conf.snapshot.json`
- `npm-run-build-current.normalized.log`
- `npx-tauri-version.log`
- `artifact-dir-current.txt`
- `winget-search-rustup.log`
- `winget-search-nsis.log`
- `winget-search-vs-buildtools.log`

Current command results:

- Host toolchain check: Node `v24.14.1`, npm/npx `11.11.0`, corepack `0.34.6`, corepack pnpm `11.4.0`, Tauri CLI `2.11.2`, git `2.49.0.windows.1`, Docker Desktop `29.3.1`; `rustc`, `cargo`, `rustup`, `cl`, `link`, and `makensis` are missing.
- MSVC/SDK check: `vswhere` exists, but no VC Tools installation was detected and no Windows SDK root was detected.
- Local executable search: no local Rust/Cargo/NSIS/MSVC executables were found in the checked user, Program Files, and workspace paths.
- Packaging recipe review: Windows release artifact is produced by Node 22, Rust stable, NSIS, `npm run vite:build`, root `cargo build --release`, binary staging, then `makensis`; output path is `dist/windows/CodexPlusPlus-<version>-windows-x64-setup.exe`.
- `npm run build` from `apps/codex-plus-manager`: failed with exit code 1 because `cargo` is not recognized.
- `npx tauri --version`: passed, `tauri-cli 2.11.2`.
- `CodexPlusPlus-main/dist`: missing; no setup exe exists locally.

Inspector/verifier disposition:

- `inspect-07-package-artifacts.ps1` was not run against the `*-package-unblock` folder because the script writes package evidence and requires a `YYYYMMDD-HHMM-package` folder name.
- Creating or mutating a sibling package evidence folder was outside Worker 4A write scope.
- Because no Windows artifact was generated, artifact inspection, SHA256 capture, and Windows install/open/overwrite/uninstall/reinstall evidence remain blocked.

Minimum Windows unblock set:

- Install Visual Studio Build Tools 2022 with VC Tools and Windows 11 SDK.
- Install Rustup and set `stable-x86_64-pc-windows-msvc` as default.
- Install NSIS.
- Open a new PowerShell session and verify `rustc`, `cargo`, `cl`, and `makensis`.
- Rerun the CI-equivalent Windows package recipe and then run package artifact inspection on a proper `YYYYMMDD-HHMM-package` evidence folder.
