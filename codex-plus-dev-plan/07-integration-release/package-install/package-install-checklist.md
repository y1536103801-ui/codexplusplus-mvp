# Package Install Check Checklist

Worker lane: Package
Date: 2026-06-17
Scope: CodexPlusPlus-main package, installer, entrypoint, and install-assistant readiness.
Status: package evidence pending

## Evidence Sources Read

- `codex-plus-dev-plan/07-integration-release/README.md`
- `codex-plus-dev-plan/07-integration-release/task-package-install-check.md`
- `codex-plus-dev-plan/INTEGRATION-VERIFICATION-CHECKLIST.md`
- `CodexPlusPlus-main/README.md`
- `CodexPlusPlus-main/README_EN.md`
- `CodexPlusPlus-main/Cargo.toml`
- `CodexPlusPlus-main/apps/codex-plus-manager/package.json`
- `CodexPlusPlus-main/apps/codex-plus-manager/package-lock.json`
- `CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/tauri.conf.json`
- `CodexPlusPlus-main/apps/codex-plus-launcher/Cargo.toml`
- `CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/Cargo.toml`
- `CodexPlusPlus-main/scripts/installer/windows/CodexPlusPlus.nsi`
- `CodexPlusPlus-main/scripts/installer/macos/package-dmg.sh`
- `CodexPlusPlus-main/.github/workflows/release-assets.yml`
- `CodexPlusPlus-main/.github/workflows/pr-build.yml`
- `CodexPlusPlus-main/docs/superpowers/specs/2026-06-02-pr-build-actions-design.md`
- `CodexPlusPlus-main/crates/codex-plus-core/src/install/mod.rs`
- `CodexPlusPlus-main/crates/codex-plus-core/src/install/windows.rs`
- `CodexPlusPlus-main/crates/codex-plus-core/src/install/macos.rs`
- `CodexPlusPlus-main/crates/codex-plus-core/tests/installers.rs`
- `CodexPlusPlus-main/apps/codex-plus-launcher/src/main.rs`
- `CodexPlusPlus-main/apps/codex-plus-manager/src-tauri/src/install.rs`
- `CodexPlusPlus-main/apps/codex-plus-manager/src/cloud/CloudInstallAssistant.tsx`

## Static Packaging Findings

- Windows installer path is custom NSIS, not Tauri bundling. `src-tauri/tauri.conf.json` has `bundle.active: false`; package workflows stage binaries and call `scripts/installer/windows/CodexPlusPlus.nsi`.
- Windows NSIS output is `dist/windows/CodexPlusPlus-${VERSION}-windows-x64-setup.exe`.
- Windows NSIS installs `codex-plus-plus.exe` and `codex-plus-plus-manager.exe` into `$LOCALAPPDATA\Programs\Codex++`.
- Windows NSIS creates desktop shortcuts for `Codex++` and `Codex++ 管理工具`.
- Windows NSIS creates Start Menu shortcuts for `Codex++`, `Codex++ 管理工具`, and uninstall.
- Windows NSIS has uninstall cleanup for installed binaries, desktop shortcuts, Start Menu shortcuts, and registry uninstall keys.
- macOS package path is custom shell script `scripts/installer/macos/package-dmg.sh`.
- macOS DMG script creates `Codex++.app` with `LSUIElement` true and `Codex++ 管理工具.app` with `LSUIElement` false, links `/Applications`, signs ad-hoc, verifies bundle structure, then writes `dist/macos/CodexPlusPlus-${VERSION}-macos-${ARCH}.dmg`.
- GitHub workflows build macOS matrix artifacts for `x64` on `macos-15-intel` targeting `x86_64-apple-darwin` and `arm64` on `macos-14` targeting `aarch64-apple-darwin`.
- Manager package `npm run build` exists and maps to `cargo build -p codex-plus-launcher --release && tauri build`, but the release workflow uses explicit frontend, Rust, staging, NSIS, and DMG steps.
- `crates/codex-plus-core/tests/installers.rs` statically covers silent and manager entrypoint names, Windows shortcut plan, macOS app bundle names, and macOS companion binary resolution.
- `CloudInstallAssistant.tsx` exposes local checks for Codex install state, saved path, silent shortcut, management shortcut, Watcher state, repair entrypoints, backend repair, and manual Codex path selection.
- `tools/inspect-07-package-artifacts.ps1` now provides a read-only artifact inspection runner for generated Windows setup and macOS x64/arm64 DMG files. It records file names, SHA256 hashes, expected artifact coverage, high-confidence secret/policy scanner results and installer-script credential-write checks without printing matched values.

## Checklist

Legend:
- Static evidence: source/config/workflow evidence exists.
- Package evidence pending: real installer artifact and platform install run are still required.
- Blocked locally: current host lacks required package environment or artifact.

| Area | Check | Static evidence | Local result | Required release evidence |
| --- | --- | --- | --- | --- |
| Windows desktop entry | Fresh install creates desktop `Codex++.lnk` targeting `codex-plus-plus.exe`. | NSIS `CreateShortcut "$DESKTOP\Codex++.lnk"` and install tests for `Codex++.lnk`. | Package evidence pending. No installer artifact present. | Install Windows x64 setup on clean Windows profile and verify shortcut target. |
| Windows desktop entry | Fresh install creates desktop `Codex++ 管理工具.lnk` targeting `codex-plus-plus-manager.exe`. | NSIS `CreateShortcut "$DESKTOP\Codex++ 管理工具.lnk"` and install tests for manager shortcut. | Package evidence pending. No installer artifact present. | Install Windows x64 setup on clean Windows profile and verify shortcut target. |
| Windows Start Menu entry | Fresh install creates Start Menu `Codex++` folder with silent launcher, manager, and uninstall entries. | NSIS `CreateDirectory "$SMPROGRAMS\Codex++"` plus three `CreateShortcut` calls. | Package evidence pending. No installer artifact present. | Verify Start Menu entries and uninstall link after install. |
| Windows uninstall metadata | Installer registers uninstall metadata. | NSIS writes HKCU uninstall values under `Software\Microsoft\Windows\CurrentVersion\Uninstall\Codex++`; Rust install plan writes `CodexPlusPlus` and removes legacy key. | Package evidence pending. | Verify Apps and Features entry, display version, display icon, install location, uninstall command. |
| Windows overwrite install | Reinstall or upgrade kills running `codex-plus-plus.exe` and `codex-plus-plus-manager.exe`, then replaces binaries. | NSIS `taskkill` and repeated `File` copy into `$INSTDIR`. | Package evidence pending. | Install vN, launch both binaries, run vN+1 installer, confirm app restarts/shortcuts still target new binaries. |
| Windows uninstall then reinstall | Uninstall removes app files, shortcuts, and registry keys, then reinstall recreates both entries. | NSIS `Section "Uninstall"` deletes shortcuts, files, and uninstall registry keys. | Package evidence pending. | Uninstall, verify cleanup, reinstall, verify entries and launch behavior. |
| macOS x64 DMG | Intel DMG builds and contains `Codex++.app` and `Codex++ 管理工具.app`. | Release and PR workflows use `macos-15-intel`, target `x86_64-apple-darwin`, arch `x64`; DMG script creates two apps. | Blocked locally on Windows; package evidence pending. | Run GitHub Actions or Intel macOS host, mount DMG, drag apps to `/Applications`, launch both. |
| macOS arm64 DMG | Apple Silicon DMG builds and contains `Codex++.app` and `Codex++ 管理工具.app`. | Release and PR workflows use `macos-14`, target `aarch64-apple-darwin`, arch `arm64`; DMG script creates two apps. | Blocked locally on Windows; package evidence pending. | Run GitHub Actions or arm64 macOS host, mount DMG, drag apps to `/Applications`, launch both. |
| macOS bundle metadata | Both apps have valid Info.plist, PkgInfo, executable, and codesign verification. | DMG script writes Info.plist and PkgInfo, ad-hoc signs, verifies with `plutil` or `PlistBuddy` and `codesign`; workflows repeat structure verification. | Blocked locally on Windows; package evidence pending. | Collect workflow logs for both matrix entries and manual mount/open results. |
| Silent launcher | `Codex++` entry starts only the silent launcher and does not open Manager UI by default. | README defines silent launcher behavior; NSIS target is `codex-plus-plus.exe`; macOS silent app uses `CodexPlusPlus`; launcher `main.rs` starts Codex/injection and only opens Manager when update prompt is needed. | Static evidence only. | Launch from desktop/Start Menu and macOS app; confirm Codex opens and Manager window does not appear except update prompt. |
| Manager entry | `Codex++ 管理工具` opens Manager UI and exposes login, install assistant, diagnostics, and advanced config paths. | README describes Manager role; NSIS target is `codex-plus-plus-manager.exe`; CloudInstallAssistant includes install repair/status actions. | Static evidence only; no packaged UI launch. | Open Manager from Windows desktop/Start Menu and macOS app, verify login, install assistant, diagnostics, advanced config navigation. |
| Missing Codex first-run | If Codex is not installed, Manager first-run/install-assistant gives actionable repair guidance and does not silently install Codex. | CloudInstallAssistant renders `未识别到 Codex`, prompts official install or manual app path selection, and says the page will not automatically download or silently install third-party software. | Static evidence only. | On clean machine without Codex, open Manager and verify missing-Codex state, choose path flow, and repair controls. |
| Package excludes shared Key | Installer does not embed shared Key, price, plan, or fixed model policy. | NSIS and DMG scripts copy binaries/icons only; no package script evidence of credentials, price, plan, or model policy. | Static evidence only. | Inspect generated artifacts and release notes before shipping. |
| Existing manual provider preservation | Install path should not overwrite user provider configuration. | Package scripts do not write `~/.codex/config.toml`; runtime provider behavior is covered by other desktop checks. | Package install scope static only. | Combine with migration/E2E evidence confirming manual providers survive login, refresh, logout, overwrite install, and reinstall. |

## Blocking Issues Before Release Signoff

- No generated installer artifacts were present locally under `CodexPlusPlus-main/dist`.
- No release binaries were present locally under `CodexPlusPlus-main/target/release`.
- Local Windows host did not have `cargo`, `rustc`, or `makensis` available in PATH.
- Local host was Windows, so macOS DMG tooling (`sips`, `iconutil`, `codesign`, `hdiutil`, `/usr/libexec/PlistBuddy`) could not be executed.
- Local `node` was v20.19.0 while workflows use Node 22. Local TypeScript check passed, but this does not prove Node 22 package builds.

## Release Evidence Still Needed

- Windows x64 installer build log from CI or a Windows packaging host with Rust and NSIS.
- Windows clean install screenshots or logs verifying desktop shortcuts, Start Menu shortcuts, uninstall metadata, silent launcher behavior, Manager launch, overwrite install, uninstall, and reinstall.
- macOS x64 workflow log plus manual mount/install/launch evidence on Intel macOS.
- macOS arm64 workflow log plus manual mount/install/launch evidence on Apple Silicon macOS.
- Missing-Codex first-run/assistant evidence from a clean machine without Codex installed.
- Confirmation that generated installer artifacts contain no packaged credentials or fixed commercial/model policy.
- Before handing off package lane evidence for a run stamp, run `tools/report-07-release-gaps.ps1 -RunStamp YYYYMMDD-HHMM` and confirm the package sibling directory/key files are no longer reported missing.
- `tools/inspect-07-package-artifacts.ps1` must pass against the generated artifact directory and write sanitized `00-artifact-metadata.md` / `04-artifact-inspection.md` before package evidence can be treated as complete.
- `tools/verify-07-package-evidence.ps1` must pass against the final timestamped package evidence folder before package evidence can be treated as hygienic.
