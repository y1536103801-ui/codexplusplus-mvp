# Platform Evidence Template

Worker lane: Package
Status: package evidence pending

Use this template only after real package artifacts are available. Static source evidence is not enough to mark an installer as passed.

## Scaffold And Verification

Generate a timestamped package evidence folder with:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-package-evidence.ps1
```

After Windows setup and macOS x64/arm64 DMG artifacts are available, run the read-only artifact inspection runner against the artifact directory:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1 -ArtifactDir CodexPlusPlus-main/dist -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package
```

The runner updates `00-artifact-metadata.md` and `04-artifact-inspection.md` with artifact names, SHA256 hashes, expected three-platform coverage, the artifact inspector command, high-confidence secret/policy scanner results, and installer-script credential scan results. It does not perform platform installs, and it does not replace the Windows/macOS evidence sections below.

After replacing every placeholder with sanitized platform evidence, run:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-package-evidence.ps1 -EvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package
```

After E2E, compatibility and Docs product copy evidence also exist, Module J must include this package folder in the aggregate release evidence gate:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-evidence.ps1 -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs
```

Generated scaffolds intentionally contain `TODO` and `pending` placeholders. The verifier must fail until Windows, macOS x64, macOS arm64, artifact inspection, and package gate evidence are filled with real sanitized results.

The latest package verifier requires the package metadata and artifact inspection files to be final, not merely descriptive. Before Module J consumes the folder:

- `00-artifact-metadata.md` must contain `Result: pass`.
- `00-artifact-metadata.md` must include `## Expected Artifact Coverage`.
- The expected coverage entries must resolve to:
  - `windows-x64-setup: present`
  - `macos-x64-dmg: present`
  - `macos-arm64-dmg: present`
- `04-artifact-inspection.md` must contain `Result: pass`.
- `04-artifact-inspection.md` must record the `inspect-07-package-artifacts.ps1` command that produced the inspection subset.
- `04-artifact-inspection.md` must record scanner and installer-script outcomes, including `High Confidence Scanner Findings` with `- none` and `Installer scripts do not write Codex credentials: pass`.

## Artifact Metadata

Result: pending / pass / fail

- Version/tag:
- Build source:
- Build URL or CI run:
- Artifact names:
  - Windows x64 setup:
  - macOS x64 DMG:
  - macOS arm64 DMG:
- Artifact hashes:
  - Windows x64 setup:
  - macOS x64 DMG:
  - macOS arm64 DMG:

## Expected Artifact Coverage

- windows-x64-setup: pending / present / missing
- macos-x64-dmg: pending / present / missing
- macos-arm64-dmg: pending / present / missing

Final evidence must replace the coverage values with exactly `present` for all three entries.

## Windows x64 Install Evidence

- Test machine / VM:
- Windows version:
- Existing Codex state: clean / Codex installed / upgrade from version:
- Installer file:

Result: pending / pass / fail

Checks:

- [ ] Fresh install completes.
- [ ] Desktop `Codex++.lnk` exists and targets `codex-plus-plus.exe`.
- [ ] Desktop `Codex++ 管理工具.lnk` exists and targets `codex-plus-plus-manager.exe`.
- [ ] Start Menu `Codex++` folder exists.
- [ ] Start Menu silent launcher entry exists.
- [ ] Start Menu Manager entry exists.
- [ ] Start Menu uninstall entry exists.
- [ ] Apps and Features uninstall metadata is present.
- [ ] Silent launcher starts Codex and does not open Manager UI by default.
- [ ] Manager opens login, install assistant, diagnostics, and advanced configuration.
- [ ] Missing-Codex first-run/install assistant shows actionable guidance.
- [ ] Overwrite install preserves valid entries and replaces binaries.
- [ ] Uninstall removes installed binaries and shortcuts.
- [ ] Reinstall after uninstall recreates entries.

Evidence links or notes:

-

## macOS x64 DMG Evidence

- Test machine / runner:
- macOS version:
- Architecture:
- Existing Codex state: clean / Codex installed / upgrade from version:
- DMG file:

Result: pending / pass / fail

Checks:

- [ ] DMG mounts.
- [ ] `Codex++.app` is present.
- [ ] `Codex++ 管理工具.app` is present.
- [ ] Apps can be copied to or opened from `/Applications`.
- [ ] `Codex++.app` launches the silent path and hides Dock UI.
- [ ] `Codex++ 管理工具.app` opens Manager UI.
- [ ] Missing-Codex first-run/install assistant shows actionable guidance.
- [ ] Overwrite install by replacing apps works.
- [ ] Removing apps uninstalls package-owned app bundles.
- [ ] Reinstall after removal works.
- [ ] Gatekeeper/quarantine behavior is documented.

Evidence links or notes:

-

## macOS arm64 DMG Evidence

- Test machine / runner:
- macOS version:
- Architecture:
- Existing Codex state: clean / Codex installed / upgrade from version:
- DMG file:

Result: pending / pass / fail

Checks:

- [ ] DMG mounts.
- [ ] `Codex++.app` is present.
- [ ] `Codex++ 管理工具.app` is present.
- [ ] Apps can be copied to or opened from `/Applications`.
- [ ] `Codex++.app` launches the silent path and hides Dock UI.
- [ ] `Codex++ 管理工具.app` opens Manager UI.
- [ ] Missing-Codex first-run/install assistant shows actionable guidance.
- [ ] Overwrite install by replacing apps works.
- [ ] Removing apps uninstalls package-owned app bundles.
- [ ] Reinstall after removal works.
- [ ] Gatekeeper/quarantine behavior is documented.

Evidence links or notes:

-

## Artifact Inspection

Result: pending / pass / fail

- Artifact inspector command:
  - `powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/inspect-07-package-artifacts.ps1 -ArtifactDir <artifact-dir> -EvidenceDir <package-evidence-dir>`
- Package does not embed a shared Key: pending / pass / fail
- Package does not embed user credentials: pending / pass / fail
- Package does not embed fixed price, plan, or model policy: pending / pass / fail
- Installer scripts do not write Codex credentials: pending / pass / fail
- Package install does not overwrite existing manual provider configuration: covered by platform install and compatibility evidence; artifact inspection records package hygiene only.

## Artifact Coverage

- windows-x64-setup: pending / present / missing
- macos-x64-dmg: pending / present / missing
- macos-arm64-dmg: pending / present / missing

## High Confidence Scanner Findings

- pending / none / describe rule names only, with matched values intentionally omitted

Final evidence must reduce this section to `- none` when no high-confidence scanner findings remain.

## Installer Script Scan

- Windows NSIS script:
- macOS DMG script:

Notes:

-
