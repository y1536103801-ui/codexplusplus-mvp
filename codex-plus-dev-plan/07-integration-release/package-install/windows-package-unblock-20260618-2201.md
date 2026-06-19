# Windows Package Unblock Note

Run stamp: 20260618-2201-package-unblock
Status: blocked; no Windows artifact generated
Evidence: `codex-plus-dev-plan\test-runs\20260618-2201-package-unblock`

## Current Host

- Windows 11 x64 host.
- Node/npm/npx/corepack/corepack pnpm are available.
- Tauri CLI is available through `npx tauri --version`.
- `rustc`, `cargo`, `rustup`, `cl`, `link`, and `makensis` are missing on PATH.
- `vswhere` exists, but no Visual Studio installation with `Microsoft.VisualStudio.Component.VC.Tools.x86.x64` was detected.
- No Windows SDK root was detected under the checked Windows Kits paths.
- `winget` is available; `choco` is missing.
- `CodexPlusPlus-main\dist` does not exist, so no local installer artifact is present.

## Actual Windows Packaging Path

The release workflow builds the Windows installer with the custom NSIS path, not with Tauri bundling:

1. Node 22.
2. Rust stable.
3. NSIS.
4. `npm install --package-lock=false` in `apps/codex-plus-manager`.
5. `npm run vite:build` in `apps/codex-plus-manager`.
6. `cargo build --release` at `CodexPlusPlus-main`.
7. Stage `target\release\codex-plus-plus.exe` and `target\release\codex-plus-plus-manager.exe` into `dist\windows\app`.
8. Run `makensis /INPUTCHARSET UTF8 /DVERSION=<version> scripts\installer\windows\CodexPlusPlus.nsi`.
9. Output `dist\windows\CodexPlusPlus-<version>-windows-x64-setup.exe`.

## Current Build Result

- `npm run build` from `apps\codex-plus-manager` failed with exit code 1.
- Exact blocker: `'cargo' is not recognized as an internal or external command, operable program or batch file.`
- The build did not reach Rust compilation, binary staging, NSIS, installer hashing, or install checks.

## Minimum Install Commands Awaiting Approval

```powershell
winget install --id Microsoft.VisualStudio.2022.BuildTools -e --source winget --override "--passive --wait --add Microsoft.VisualStudio.Workload.VCTools --add Microsoft.VisualStudio.Component.Windows11SDK.26100 --includeRecommended"
winget install --id Rustlang.Rustup -e --source winget --scope user
winget install --id NSIS.NSIS -e --source winget
```

After installation, open a new PowerShell session and verify:

```powershell
rustup default stable-x86_64-pc-windows-msvc
rustc --version
cargo --version
cl /?
makensis /VERSION
```

Then rerun the CI-equivalent Windows package recipe and collect the setup exe hash.
