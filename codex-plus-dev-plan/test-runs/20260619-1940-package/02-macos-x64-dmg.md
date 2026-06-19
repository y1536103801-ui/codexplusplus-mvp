# 02 macOS x64 DMG Evidence

Run folder: 20260618-2306-package
Status: deferred post-MVP by Windows-only MVP scope

Result: fail

## Missing Evidence

- No macOS x64 test machine or runner was available in this pass.
- No macOS x64 DMG file or hash was produced.
- DMG mount evidence is absent.
- `Codex++.app` and `Codex++ 绠＄悊宸ュ叿.app` presence was not verified.
- Copy/open behavior from `/Applications` was not verified.
- Hidden Dock or `LSUIElement` silent app behavior was not verified.
- Manager UI behavior was not verified.
- Missing-Codex first-run assistant behavior was not verified on macOS x64.
- Overwrite, remove or uninstall, and reinstall behavior was not verified.
- Gatekeeper or quarantine behavior was not verified.

## Release Boundary

macOS x64 package evidence remains a full cross-platform release blocker, but it is deferred post-MVP by the 2026-06-19 owner decision that MVP package scope is Windows x64 only.
