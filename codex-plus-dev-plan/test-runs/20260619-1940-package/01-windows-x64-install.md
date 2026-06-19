# 01 Windows x64 Install Evidence

Run folder: 20260618-2306-package
Status: executed

Result: pass

## Artifact Under Test

- Test host: local Windows package-build host, user `DESKTOP-894L06I\20622`.
- Installer file: `CodexPlusPlus-1.2.9-windows-x64-setup.exe`.
- SHA256: `20ebf88d1bd2ebeca903ffedf39fdd175c9b8edfdc6fbca7e8764b5278a031ac`.
- Size: `8685048` bytes.
- Installer command: `Start-Process <setup.exe> -ArgumentList '/S' -Wait -PassThru`.
- Install directory: `C:\Users\20622\AppData\Local\Programs\Codex++`.

## Fresh Install

- Fresh install command returned exit code `0`.
- Installed files were present: `codex-plus-plus.exe`, `codex-plus-plus-manager.exe`, `uninstall.exe`.
- HKCU `Software\Codex++` install dir was written.
- Apps and Features uninstall metadata was present under HKCU `Software\Microsoft\Windows\CurrentVersion\Uninstall\Codex++`.
- Display name: `Codex++`.
- Display version: `1.2.9`.
- Publisher: `BigPizzaV3`.
- Start Menu entries were present for launcher, Manager, and uninstall.

## Shortcut Evidence

- Desktop Codex++.lnk: pass after installer fix.
- Desktop Codex++ 管理工具.lnk: pass after installer fix.
- Start Menu `Codex++.lnk`: pass.
- Start Menu `Codex++ 管理工具.lnk`: pass.
- Start Menu `卸载 Codex++.lnk`: pass.

## Launch Evidence

- Silent launcher command: `Start-Process codex-plus-plus.exe`.
- Silent launcher result: pass after launcher update-check fix. Starting `codex-plus-plus.exe` left only one `codex-plus-plus` process and `ManagerProcessCount` was `0` after 8 seconds.
- Manager launch command: `Start-Process codex-plus-plus-manager.exe`.
- Manager launch result: partial. `codex-plus-plus-manager.exe` stayed running and exposed a visible window titled `Codex++ 管理工具`.
- Manager page coverage result: pass with page-level screenshots and CDP text captures from the installed Manager executable.
- Manager login: pass. Evidence: `windows-ui-evidence/manager-login-panel.png` and `windows-ui-evidence/manager-login-panel-dom.txt` show the `Codex++ Cloud` login binding page with browser login, compatible account/password login, email, and password controls.
- Manager install assistant: pass. Evidence: `windows-ui-evidence/manager-maintenance-diagnostics.png` and `windows-ui-evidence/manager-maintenance-dom.txt` show the install/maintenance page with Codex app status, silent launcher entry, manager entry, Watcher status, entry repair, app path chooser, and manual launch controls.
- Manager diagnostics: pass. Evidence: `windows-ui-evidence/manager-diagnostics-report-panel.png` and `windows-ui-evidence/manager-about-diagnostics-dom.txt` show the diagnostics report panel with regenerate and copy-report actions.
- Manager advanced configuration: pass. Evidence: `windows-ui-evidence/manager-advanced-provider.png` and `windows-ui-evidence/manager-advanced-provider-dom.txt` show the advanced providers page with provider switching, provider list, protocol/Key/config-file scope, and a default provider using official login with Responses API.
- Missing-Codex first-run: pass. Evidence: `windows-ui-evidence/missing-codex-maintenance.png`, `windows-ui-evidence/missing-codex-maintenance-dom.txt`, and `windows-ui-evidence/missing-codex-summary.json` show an isolated first-run profile where Codex app detection is missing and the user is offered app directory / `Codex.exe` selection actions.

## Overwrite Install

## Manager UI Evidence Method

- Direct non-elevated Manager launch returned `The requested operation requires elevation`; the installed Manager manifest requests administrator privileges.
- Page evidence was captured by launching the installed `C:\Users\20622\AppData\Local\Programs\Codex++\codex-plus-plus-manager.exe` with `__COMPAT_LAYER=RunAsInvoker` and `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=<port> --remote-allow-origins=*`.
- This method did not modify runtime source, the installer, user credentials, or Codex configuration. It proves the installed Manager UI pages render and expose the required page-level controls under WebView2 automation; it does not change the package privilege manifest.
- CDP DOM text for Chinese labels is encoding-skewed in some `.txt` captures, so the PNG screenshots are the primary human-readable page evidence. `windows-ui-evidence/windows-ui-evidence-summary.json` records the screenshot-to-surface mapping.

## Missing-Codex Isolation Method

- Started a second Manager instance with `LOCALAPPDATA`, `APPDATA`, `USERPROFILE`, and `CODEX_HOME` pointed under `codex-plus-dev-plan/test-runs/20260618-2306-package/missing-codex-profile/`.
- The isolated profile summary records `local_openai_codex_exists: false` and `local_programs_openai_codex_exists: false`.
- No real Codex install directory was moved, renamed, or deleted.

- Overwrite install command returned exit code `0`.
- The installer taskkill step cleared test `codex-plus-plus.exe` and `codex-plus-plus-manager.exe` processes.
- Install directory and installed files remained present after overwrite.

## Uninstall and Reinstall

- Uninstall command: `Start-Process uninstall.exe -ArgumentList '/S' -Wait -PassThru`.
- Uninstall command returned exit code `0`.
- After uninstall, install directory was absent.
- After uninstall, Start Menu folder was absent.
- After uninstall, HKCU `Software\Codex++` and HKCU uninstall registry keys were absent.
- First reinstall attempt was cancelled at the Windows elevation prompt and produced `The operation was canceled by the user`.
- Second reinstall command returned exit code `0`.
- After reinstall, install directory, installed files, Start Menu folder, HKCU app registry key, and HKCU uninstall registry key were present.

## Fixes Applied During This Pass

- `CodexPlusPlus.nsi` now creates desktop shortcuts through the explicit `%USERPROFILE%\Desktop` path in addition to NSIS `$DESKTOP`.
- `codex-plus-launcher` now records available updates in diagnostics instead of opening Manager UI during default silent launcher startup.
- Rebuilt release binaries and regenerated `CodexPlusPlus-1.2.9-windows-x64-setup.exe`.

## Final Installed State

- A final overwrite install returned exit code `0` and cleared test processes.
- Final install directory exists.
- No `codex-plus-plus.exe` or `codex-plus-plus-manager.exe` test process remained after cleanup.

## Release Boundary

Windows package evidence is passable for the Windows-only MVP package gate. macOS x64 and macOS arm64 package evidence remains deferred post-MVP by owner scope decision and is not treated as a Windows-only MVP blocker.
