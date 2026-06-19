# 07 Integration Release Reports

本目录记录 `07-integration-release` 阶段的并行验收报告、E2E 证据索引和 coordinator 发布裁决。

Expected reports:

- `coordinator-integration-release-verification.md`
- `module-j-final-report-template.md`
- `module-j-final-report.md` once real release evidence exists
- `worker-e2e-buy-login-launch-final.md`
- `worker-compatibility-migration-final.md`
- `worker-package-install-final.md`
- `worker-docs-product-copy-final.md`
- `../release-local-verification.md`

同阶段 worker 可以并行执行，但必须保持写入范围互不重叠。最终发布裁决只在 E2E、兼容迁移、安装包检查、文档同步和总门禁全部有证据后生成。

Docs product copy now has its own release evidence lane. The final evidence set must include `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs`, verified with `tools/verify-07-docs-product-copy-evidence.ps1`, before aggregate evidence, coverage, readiness, handoff or Module J final-report checks can be treated as complete. Current static docs and worker reports may still contain draft/pending boundaries; they are not a substitute for finalized release docs evidence.

Final Module J reports must pass:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-module-j-report.ps1 -ReportFile codex-plus-dev-plan/07-integration-release/reports/module-j-final-report.md -CoverageSummaryFile codex-plus-dev-plan/07-integration-release/reports/release-coverage-summary.md -ReadinessSummaryFile codex-plus-dev-plan/07-integration-release/reports/release-readiness-summary.md
```

To prepare a full matched release evidence workspace before execution:

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/new-07-release-evidence-set.ps1
```

The final handoff lives in the timestamped `codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release` directory, together with its sibling `YYYYMMDD-HHMM-e2e`, `YYYYMMDD-HHMM-package`, `YYYYMMDD-HHMM-compatibility`, `YYYYMMDD-HHMM-docs`, and `YYYYMMDD-HHMM-business` evidence folders. `07-integration-release/reports/module-j-final-report.md` can mirror the final report for stage reporting, but the release handoff verifier treats the timestamped `*-release` directory as the authoritative package.

Before a timestamped `*-release` workspace can be treated as the final handoff, its `00-release-evidence-index.md` must be changed to `Status: final`, every scaffold/pending marker must be removed, and `Final Verification Results` must record aggregate evidence, Docs product copy evidence, business readiness, coverage status, readiness posture, Module J report verification and final recommendation values that match the stored summaries and final report.

For no-go or scaffold checks, generate the readiness summary and run handoff verification without `-AllowGoCandidate`. For any go-candidate readiness posture, regenerate `release-readiness-summary.md` with `-AllowGoCandidate` so it records `Allow go candidate: true`, then run the handoff verifier with the same allowance.

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/summarize-07-release-readiness.ps1 -E2EEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-e2e -PackageEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-package -CompatibilityEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-compatibility -DocsEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-docs -BusinessEvidenceDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-business -CoverageSummaryFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-coverage-summary.md -OutputFile codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release/release-readiness-summary.md -AllowGoCandidate
```

```text
powershell -ExecutionPolicy Bypass -File codex-plus-dev-plan/tools/verify-07-release-handoff.ps1 -ReleaseDir codex-plus-dev-plan/test-runs/YYYYMMDD-HHMM-release -AllowGoCandidate
```

Use `-AllowGoCandidate` only after real production-equivalent E2E, package, compatibility, Docs product copy and business readiness evidence exists. Without it, the readiness summary must remain `no-go`, and a go/go-with-accepted-risks Module J recommendation must not pass final verification.
