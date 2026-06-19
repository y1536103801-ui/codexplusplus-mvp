# 06 MVP Scope Decision

Run folder: 20260618-2306-package
Status: owner-approved scope change
Result: pass

Decision date: 2026-06-19

MVP package scope: Windows x64 only.

Owner decision: user stated "MVP 先只认 Windows。" in the coordinator session.

## In Scope For MVP

- Windows x64 setup artifact.
- Windows clean install, overwrite install, uninstall, and reinstall evidence.
- Windows desktop shortcut, Start Menu, Apps and Features, silent launcher, Manager UI, and Missing-Codex first-run evidence.
- Artifact inspection for the Windows setup artifact.

## Deferred Post-MVP

- macOS x64 DMG artifact, install evidence, Gatekeeper behavior, and first-run evidence.
- macOS arm64 DMG artifact, install evidence, Gatekeeper behavior, and first-run evidence.
- macOS unsigned or unnotarized distribution acceptance decision.

## Release Boundary

- This Windows-only MVP scope decision does not waive Windows evidence requirements.
- This Windows-only MVP scope decision does not waive E2E, compatibility, docs, business readiness, Module J, or release go/no-go gates.
- The full cross-platform package verifier without the Windows-only switch must still fail until macOS package evidence exists.
