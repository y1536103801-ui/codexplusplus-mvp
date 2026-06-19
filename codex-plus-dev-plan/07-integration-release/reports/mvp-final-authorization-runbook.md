# MVP 最终授权后本地收口 Runbook

Status: execution runbook only
Owner: 07 integration release coordinator
Last updated: 2026-06-19

本 runbook 只说明用户授权后 coordinator 应按什么顺序执行本地收口验证。它不是通过结果，也不是证据本身。执行者必须在 `F:\codex++\codex+++(2)\codex+++` 作为 `$Root` 运行命令，并在任何会发起本地登录、写入本地授权状态、启动 Desktop Manager/Codex、写入 provider 状态、发起 gateway 请求或使用 `-Allow*` 开关的步骤前，先取得对应范围的精确授权短语。

## 授权短语和硬边界

只接受以下两个精确授权短语。两个短语不互相解锁；只输入其中一个时，另一个范围仍然禁止执行。

| 授权短语 | 只解锁的范围 | 仍然禁止 |
| --- | --- | --- |
| `授权本地测试合规确认` | 只允许运行本地 admin compliance accept helper：`accept-07-local-admin-compliance.ps1 -AllowLocalComplianceAccept`。 | 不允许启动 Desktop Manager/Codex，不允许 browser handoff，不允许 gateway/admin audit，不允许 provider snapshot/runtime compatibility，不允许 production。 |
| `授权在隔离 profile 下执行 Desktop Manager/Codex 本地 E2E` | 只允许在 `new-07-desktop-compatibility-harness.ps1` 生成的隔离 profile 下执行本地 E2E：Desktop Manager/Codex 启动、browser handoff、client API/bootstrap、gateway policy、admin audit、compat snapshot capture/inspection。 | 不允许运行 admin compliance accept helper，不允许使用真实用户 profile，不允许写真实 production provider config，不允许 production。 |

执行规则：

- 未收到匹配范围的授权短语前，只允许做只读检查和 scaffold 准备；不得调用任何 `-Allow*` 开关。
- 不接受任何总括式、宽泛式或改写后的授权短语。
- `accept-07-local-admin-compliance.ps1` 只能在收到 `授权本地测试合规确认` 后运行，且必须带 `-AllowLocalComplianceAccept`，目标只能是 `http://localhost`、`http://127.0.0.1` 或 `http://[::1]`。
- Desktop Manager/Codex、本地 browser handoff、gateway/admin audit、compat snapshot runtime 只能在收到 `授权在隔离 profile 下执行 Desktop Manager/Codex 本地 E2E` 后运行，且只能使用 `new-07-desktop-compatibility-harness.ps1` 生成的隔离 profile；不得使用真实用户 profile。
- 任何步骤都不得使用 `-AllowProduction`，不得连接 production admin/backend/gateway/provider 目标。
- 不得把真实 JWT、refresh token、Authorization header、上游 provider key、desktop `poll_token`、`session_token` 或原始 provider config 复制进证据。
- 任何截图、日志、命令输出进入证据前必须先人工检查并脱敏。
- 如果某一步缺少真实环境、测试账户、构建产物或 owner approval，记录为 blocker/no-go，不得用 fixture、scaffold、placeholder 或 weaker check 代替。

## 证据目录约定

一次最终收口必须使用同一个 `$RunStamp`：

```powershell
$Root = 'F:\codex++\codex+++(2)\codex+++'
$RunStamp = 'YYYYMMDD-HHMM'
$TestRuns = Join-Path $Root 'codex-plus-dev-plan\test-runs'
$E2E = Join-Path $TestRuns "$RunStamp-e2e"
$Package = Join-Path $TestRuns "$RunStamp-package"
$Compatibility = Join-Path $TestRuns "$RunStamp-compatibility"
$Docs = Join-Path $TestRuns "$RunStamp-docs"
$Business = Join-Path $TestRuns "$RunStamp-business"
$Release = Join-Path $TestRuns "$RunStamp-release"
$DesktopHarnessRoot = Join-Path $TestRuns '_desktop-harness'
```

输出归档规则：

| 输出 | 目录 | 说明 |
| --- | --- | --- |
| 本地 admin compliance accept 结果 | `$E2E\03-admin-setup.md` | 只记录本地 URL、acknowledged 状态、执行时间；不记录 token。 |
| Client API/bootstrap/device/usage 子集 | `$E2E\02-contract-checks.md`、`$E2E\04-client-api-e2e.md`、`$E2E\09-usage-events-audit.md`、`$E2E\11-defects.md` | 由 E2E runner 或等价人工证据填充。 |
| Browser handoff | `$E2E\04-client-api-e2e.md`、`$E2E\06-desktop-manager-e2e.md` | 不得记录 `poll_token` 或 session token。 |
| Gateway policy 请求/拒绝 | `$E2E\05-gateway-policy-e2e.md`、`$E2E\09-usage-events-audit.md` | 记录 request id、policy code、状态；不记录 Authorization。 |
| Desktop Manager/Codex 隔离运行 | `$E2E\06-desktop-manager-e2e.md` | 记录隔离 profile、构建版本、登录/启动/请求结果和截图索引。 |
| provider snapshots | `$DesktopHarnessRoot\$RunStamp-desktop-harness\snapshots` | 原始 snapshot 仅保存在隔离 harness；只能含 provider 名称、URL/key hash 等脱敏字段。 |
| Compatibility snapshot inspection/runtime evidence | `$Compatibility` | inspector 输出加人工 runtime evidence；snapshot-only 不能算完整通过。 |
| Package install evidence | `$Package` | 由 package lane 填平台安装、artifact metadata、扫描结果。 |
| Docs product copy evidence | `$Docs` | 由 docs lane 填 public README/user/admin/release notes/HTML 证据。 |
| Business readiness evidence | `$Business` | 由 business/legal/owner approval lane 填。 |
| 聚合 verifier 日志、coverage、readiness、Module J final report | `$Release` | 最终 handoff authoritative package。 |

## 授权前允许的只读/安全命令

这些命令不启动 Manager/Codex，不调用 `-Allow*`，不接受 compliance，不写真实 provider config。若命令输出包含本地路径或 URL，可进入 release scratch log；若包含任何 secret，必须丢弃并重跑脱敏版本。

```powershell
Set-Location $Root

powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-static.ps1 -Root $Root

powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\validate-stage-gate.ps1 -Root $Root -Stage 07-integration-release
```

可选 scaffold 准备。它会新增 timestamped placeholder 目录，但不会执行 E2E、不会启动桌面运行时、不会接受 compliance。不要使用 `-Force` 覆盖别人已有输出。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\new-07-release-evidence-set.ps1 -Root $Root -OutputRoot $TestRuns -Timestamp $RunStamp
```

可选 E2E 输入模板准备。模板里的 token/key/model placeholder 必须由 owner 手动填写到本地未提交 env 文件；不要把填好值的 env 文件复制进报告。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\new-07-e2e-env-template.ps1 -Root $Root -OutputRoot $TestRuns -Timestamp $RunStamp
```

## 按授权范围执行顺序

### 1. 确认环境和输入

如果后续要执行本地 E2E，先在收到 `授权在隔离 profile 下执行 Desktop Manager/Codex 本地 E2E` 后做 readiness 检查。`-EndpointPreflight` 只验证 endpoint 形状和可达性；它仍然不能替代真实 E2E。不得添加 `-AllowProduction`。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-e2e-readiness.ps1 -Root $Root -EnvFile '<local-redacted-env.ps1>' -EndpointPreflight
```

输出处理：

- readiness 通过/失败摘要写入 `$E2E\00-environment.md`。
- 失败项写入 `$E2E\11-defects.md`。
- 不复制 env 文件内容，不打印 token/key 值。

### 2. 本地 admin compliance accept

只有在用户已输入 `授权本地测试合规确认` 后才运行。该脚本自身会拒绝非本地 admin URL，并且 token 值不会打印。该短语不授权任何 Desktop Manager/Codex、本地 E2E、browser handoff、gateway/admin audit 或 compatibility snapshot 操作。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\accept-07-local-admin-compliance.ps1 -Root $Root -EnvFile '<local-redacted-env.ps1>' -EvidenceDir $E2E -AllowLocalComplianceAccept
```

证据写入：

- `$E2E\03-admin-setup.md`：helper 使用 `-EvidenceDir $E2E` 自动写入脱敏证据，记录 `Result: pass` 或 `Result: fail`、本地 admin base URL、执行时间、`Acknowledged: true/false`。
- `$E2E\11-defects.md`：记录任何拒绝、非本地 URL、缺 token、API error。
- 不记录 `ADMIN_TOKEN`、Authorization header 或 ack body 中的敏感上下文。

### 3. 本地 E2E 子集

只有在用户已输入 `授权在隔离 profile 下执行 Desktop Manager/Codex 本地 E2E` 后才运行本节命令。先运行 client API/bootstrap 子集，再逐步打开 browser handoff、gateway policy、admin audit。任何 browser/gateway/admin `-Allow*` 都只在这个授权范围内有效，并且只能使用测试账号、低成本模型和非 production upstream；不得添加 `-AllowProduction`。

Client API 子集：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\sub2api-main\tools\e2e\codexplus\run-local-e2e.ps1 -Root $Root -OutputRoot $TestRuns -Timestamp $RunStamp -EnvFile '<local-redacted-env.ps1>' -ProbeHttp -EndpointPreflight
```

Browser handoff 完整子集，仅在真实 browser-authenticated 测试 token 可用后加 `-AllowBrowserComplete`：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\sub2api-main\tools\e2e\codexplus\run-local-e2e.ps1 -Root $Root -OutputRoot $TestRuns -Timestamp $RunStamp -EnvFile '<local-redacted-env.ps1>' -ProbeHttp -EndpointPreflight -RunBrowserHandoff -AllowSessionStart -AllowBrowserComplete
```

Gateway policy 和 admin audit 子集：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\sub2api-main\tools\e2e\codexplus\run-local-e2e.ps1 -Root $Root -OutputRoot $TestRuns -Timestamp $RunStamp -EnvFile '<local-redacted-env.ps1>' -ProbeHttp -EndpointPreflight -RunGatewayPolicy -AllowGatewayRequests -RunAdminAudit -AllowAdminAuditReads
```

证据写入：

- `$E2E\02-contract-checks.md`：contract/endpoint/preflight 结果。
- `$E2E\04-client-api-e2e.md`：login/bootstrap/device/client API 子集。
- `$E2E\05-gateway-policy-e2e.md`：active success、not purchased、expired、low balance、device revoked、model denied 等 policy 结果。
- `$E2E\09-usage-events-audit.md`：usage/audit/event 行，request id 必须能对上 gateway 结果。
- `$E2E\11-defects.md`：所有失败、跳过原因和 blocker。

### 4. 隔离 profile Desktop Manager/Codex E2E

生成隔离 harness 是安全准备动作。该命令只写入 `$DesktopHarnessRoot` 下的隔离 `USERPROFILE`、`HOME`、`APPDATA`、`LOCALAPPDATA`、`CODEX_HOME` 和假 manual provider seed；不得替换为真实上游 key。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\new-07-desktop-compatibility-harness.ps1 -Root $Root -OutputRoot $DesktopHarnessRoot -Timestamp $RunStamp -ManagerBuildPath '<manager-build-exe-path>'
```

收到 `授权在隔离 profile 下执行 Desktop Manager/Codex 本地 E2E` 后，先捕获 pre-upgrade snapshot：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "$DesktopHarnessRoot\$RunStamp-desktop-harness\capture-snapshot.ps1" -Stage pre-upgrade
```

同一授权范围内启动隔离 Manager。只有这一步会启动 Desktop Manager；不得改用真实用户 profile，不得添加 production target 或 `-AllowProduction`：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "$DesktopHarnessRoot\$RunStamp-desktop-harness\launch-manager-isolated.ps1"
```

人工 UI 顺序：

1. 用测试账号完成 Manager browser handoff login。
2. 确认 `Codex++ Cloud` provider 被写入或修复，且 `manual-e2e` 仍存在。
3. 运行 post-upgrade snapshot。
4. 从 Manager 启动 Codex。
5. 通过 gateway 发送一次低成本模型请求。
6. 在 Manager logout cloud。
7. 运行 logout snapshot。
8. 切回 manual provider，确认手动 provider 仍可选，provider sync 日志已脱敏。
9. 执行 rollback rehearsal 或恢复旧 snapshot。
10. 运行 rollback snapshot。

Snapshot 命令：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "$DesktopHarnessRoot\$RunStamp-desktop-harness\capture-snapshot.ps1" -Stage post-upgrade
powershell -NoProfile -ExecutionPolicy Bypass -File "$DesktopHarnessRoot\$RunStamp-desktop-harness\capture-snapshot.ps1" -Stage logout
powershell -NoProfile -ExecutionPolicy Bypass -File "$DesktopHarnessRoot\$RunStamp-desktop-harness\capture-snapshot.ps1" -Stage rollback
```

证据写入：

- `$E2E\06-desktop-manager-e2e.md`：Manager login、browser handoff、provider write、Codex launch、low-cost gateway request、logout/rollback 的人工结果。
- `$Compatibility\01-pre-upgrade-snapshot.md` 到 `$Compatibility\06-rollback-rehearsal.md`：对应 snapshot 和人工 runtime 结果。
- snapshot JSON 留在 `$DesktopHarnessRoot\$RunStamp-desktop-harness\snapshots`；不要把 raw provider config 粘贴进 Markdown。

### 5. Provider snapshot compatibility inspection

收到 `授权在隔离 profile 下执行 Desktop Manager/Codex 本地 E2E` 且四个 snapshot 都存在后，运行 inspector。它只做 hash/fingerprint 级别的 provider 兼容检查；snapshot-only pass 不能替代 UI/runtime evidence。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\inspect-07-compatibility-snapshots.ps1 -Root $Root -EvidenceDir $Compatibility -PreUpgradeSnapshot "$DesktopHarnessRoot\$RunStamp-desktop-harness\snapshots\pre-upgrade.json" -PostUpgradeSnapshot "$DesktopHarnessRoot\$RunStamp-desktop-harness\snapshots\post-upgrade.json" -LogoutSnapshot "$DesktopHarnessRoot\$RunStamp-desktop-harness\snapshots\logout.json" -RollbackSnapshot "$DesktopHarnessRoot\$RunStamp-desktop-harness\snapshots\rollback.json"
```

证据写入：

- `$Compatibility\00-test-context.md`：isolated harness 路径、Manager build、测试范围。
- `$Compatibility\01-pre-upgrade-snapshot.md`：legacy manual provider hash-only context。
- `$Compatibility\02-post-upgrade-cloud.md`：manual provider preserved、`Codex++ Cloud` present。
- `$Compatibility\03-cloud-logout-boundary.md`：logout 只清 cloud session，不删 manual provider。
- `$Compatibility\04-manual-provider-switch.md`：人工 provider switch/runtime request 结果。
- `$Compatibility\05-provider-sync.md`：sync 行为和日志脱敏检查。
- `$Compatibility\06-rollback-rehearsal.md`：rollback preserved/recovered 结果。
- `$Compatibility\07-compatibility-gate-report.md`：最终 `Result: pass` 或 `Result: fail`。

## 聚合验证和 release handoff

当 E2E、package、compatibility、docs、business 五条 evidence lane 都填完并脱敏后，按顺序运行。`-WindowsOnlyMvp` 只在当前 MVP 明确限定 Windows package evidence 时使用。

单 lane verifier：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-evidence.ps1 -Root $Root -EvidenceDir $E2E
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-package-evidence.ps1 -Root $Root -EvidenceDir $Package -WindowsOnlyMvp
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-compatibility-evidence.ps1 -Root $Root -EvidenceDir $Compatibility
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-docs-product-copy-evidence.ps1 -Root $Root -EvidenceDir $Docs
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-business-readiness.ps1 -Root $Root -EvidenceDir $Business
```

Aggregate evidence verifier：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-release-evidence.ps1 -Root $Root -E2EEvidenceDir $E2E -PackageEvidenceDir $Package -CompatibilityEvidenceDir $Compatibility -DocsEvidenceDir $Docs -WindowsOnlyMvp -LogRoot (Join-Path $Release 'aggregate-verifier-logs')
```

Coverage summary：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\summarize-07-release-coverage.ps1 -Root $Root -E2EEvidenceDir $E2E -PackageEvidenceDir $Package -CompatibilityEvidenceDir $Compatibility -DocsEvidenceDir $Docs -OutputFile (Join-Path $Release 'release-coverage-summary.md') -FailOnIncomplete
```

Readiness summary。只有真实 production-equivalent evidence 全部通过且 owner 明确允许 go candidate 时，才可加 `-AllowGoCandidate`；否则保持 no-go。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\summarize-07-release-readiness.ps1 -Root $Root -E2EEvidenceDir $E2E -PackageEvidenceDir $Package -CompatibilityEvidenceDir $Compatibility -DocsEvidenceDir $Docs -BusinessEvidenceDir $Business -CoverageSummaryFile (Join-Path $Release 'release-coverage-summary.md') -OutputFile (Join-Path $Release 'release-readiness-summary.md')
```

Module J final report verification：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-module-j-report.ps1 -Root $Root -ReportFile (Join-Path $Release 'module-j-final-report.md') -CoverageSummaryFile (Join-Path $Release 'release-coverage-summary.md') -ReadinessSummaryFile (Join-Path $Release 'release-readiness-summary.md')
```

Final release handoff verification：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\codex-plus-dev-plan\tools\verify-07-release-handoff.ps1 -Root $Root -ReleaseDir $Release
```

聚合输出规则：

- `$Release\release-coverage-summary.md`：coverage matrix。
- `$Release\release-readiness-summary.md`：Module J readiness posture。
- `$Release\module-j-final-report.md`：最终 go/no-go 报告。
- `$Release\00-release-evidence-index.md`：必须改为 `Status: final`，并填入 aggregate/docs/business/coverage/readiness/Module J/handoff 结果。
- `codex-plus-dev-plan\07-integration-release\reports\module-j-final-report.md`：只在 `$Release` 里的 final report 通过 verifier 后再复制。

## 最终通过条件

最终 MVP 本地收口只能在以下条件同时成立时记录为 go 或 go-with-accepted-risks：

- `$E2E` 13 个 required evidence files 全部存在，无 `pending`、`TODO`、placeholder、未脱敏 secret，且 `verify-07-evidence.ps1` 通过。
- `$Package`、`$Compatibility`、`$Docs`、`$Business` 各自 verifier 通过。
- `verify-07-release-evidence.ps1` 通过，且四个技术 evidence 目录互不相同。
- coverage summary complete，无 missing requirements 或 nonrelease markers。
- readiness summary 与 evidence/coverage/business 输入一致；若是 go candidate，必须显式使用 owner-approved `-AllowGoCandidate` 生成。
- Module J final report 和 release handoff verifier 通过。
- 所有人工风险、跳过项、不可用项都有 disposition；任何未完成真实 E2E、Desktop runtime、provider compatibility、package install 或 business approval 都必须保持 no-go。
