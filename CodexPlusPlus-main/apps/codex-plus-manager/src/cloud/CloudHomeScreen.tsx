import { Cloud, ExternalLink, Info, LogOut, Play, RefreshCw, SlidersHorizontal, UserRound } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { CloudDiagnosticsPanel } from "./CloudDiagnosticsPanel";
import { CloudInstallAssistant } from "./CloudInstallAssistant";
import { CloudStatusPanel } from "./CloudStatusPanel";
import { CloudTutorialPanel } from "./CloudTutorialPanel";
import { CloudUsagePanel } from "./CloudUsagePanel";
import { useCloudRuntime } from "./useCloudRuntime";
import type { CloudInstallState } from "./types";
import "./cloud.css";

type StatusLikePath = {
  status: string;
  path: string | null;
};

type OverviewLike = {
  codex_app?: StatusLikePath;
  silent_shortcut?: StatusLikePath;
  management_shortcut?: StatusLikePath;
} | null;

type SettingsLike = {
  settings?: {
    codexAppPath?: string;
  };
} | null;

type WatcherLike = {
  enabled?: boolean;
  disabled_flag?: string;
} | null;

export type CloudHomeActions = {
  launch: () => Promise<void>;
  checkHealth: () => Promise<void>;
  repairBackend: () => Promise<void>;
  repairShortcuts: () => Promise<void>;
  chooseCodexAppPath: (mode: "folder" | "file") => Promise<void>;
  openExternalUrl: (url: string) => Promise<void>;
  showMessage: (title: string, message: string, status?: string) => Promise<void>;
};

type Props = {
  overview: OverviewLike;
  settings: SettingsLike;
  watcher: WatcherLike;
  actions: CloudHomeActions;
  onOpenAdvancedProviders: () => void;
  onOpenMaintenance: () => void;
  refreshSignal?: number;
};

function mergedInstallState(overview: OverviewLike, settings: SettingsLike, watcher: WatcherLike): CloudInstallState {
  return {
    codexApp: {
      status: overview?.codex_app?.status ?? "not_checked",
      path: overview?.codex_app?.path ?? null,
    },
    savedAppPath: settings?.settings?.codexAppPath || null,
    silentShortcut: {
      status: overview?.silent_shortcut?.status ?? "not_checked",
      path: overview?.silent_shortcut?.path ?? null,
    },
    managementShortcut: {
      status: overview?.management_shortcut?.status ?? "not_checked",
      path: overview?.management_shortcut?.path ?? null,
    },
    watcherEnabled: typeof watcher?.enabled === "boolean" ? watcher.enabled : null,
    watcherDetail: watcher?.disabled_flag ?? null,
  };
}

function cloudSummary(stateStatus: string | undefined) {
  if (stateStatus === "available") return "已就绪，可以启动 Codex。";
  if (stateStatus === "not_authenticated") return "登录后即可查看权益和余额。";
  if (stateStatus === "not_purchased") return "账号有效，但服务尚未开通。";
  if (stateStatus === "expired") return "服务期已结束，请续费后继续使用。";
  if (stateStatus === "disabled") return "服务当前不可用，请联系管理员。";
  if (stateStatus === "low_balance") return "余额偏低，仍可继续使用。";
  if (stateStatus === "device_revoked") return "本机设备已被停用，请联系管理员。";
  if (stateStatus === "model_unavailable") return "默认模型暂不可用，请刷新或按提示处理。";
  if (stateStatus === "rate_limited") return "当前使用人数较多，请稍后重试。";
  if (stateStatus === "gateway_unhealthy") return "服务暂时不可用，请稍后重试。";
  if (stateStatus === "local_codex_missing") return "尚未找到本机 Codex，请先完成安装配置。";
  if (stateStatus === "local_config_failed") return "Codex 准备失败，请刷新或联系管理员。";
  if (stateStatus === "stale_snapshot") return "状态可能不是最新，请刷新。";
  return "正在读取账户状态。";
}

function serviceStatusLabel(stateStatus: string | undefined) {
  if (stateStatus === "available") return "已就绪";
  if (stateStatus === "low_balance") return "余额提醒";
  if (stateStatus === "not_authenticated") return "未登录";
  if (stateStatus === "not_purchased") return "未开通";
  if (stateStatus === "expired") return "已过期";
  if (stateStatus === "disabled") return "不可用";
  if (stateStatus === "device_revoked") return "设备停用";
  if (stateStatus === "model_unavailable") return "模型不可用";
  if (stateStatus === "rate_limited") return "繁忙";
  if (stateStatus === "gateway_unhealthy") return "暂不可用";
  if (stateStatus === "local_codex_missing") return "未检测到 Codex";
  if (stateStatus === "local_config_failed") return "准备失败";
  if (stateStatus === "stale_snapshot") return "需刷新";
  return "读取中";
}

function safeText(value: string | null | undefined, fallback = "未返回") {
  return value && value.trim() ? value : fallback;
}

function formatDate(value: string | null | undefined) {
  if (!value) return "未返回";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

export function CloudHomeScreen({ overview, settings, watcher, actions, onOpenAdvancedProviders, onOpenMaintenance, refreshSignal }: Props) {
  const handleNotice = useCallback(
    (title: string, message: string, status?: string) => {
      void actions.showMessage(title, message, status);
    },
    [actions],
  );
  const cloud = useCloudRuntime({
    onNotice: handleNotice,
  });
  const data = cloud.state?.bootstrap.data;
  const status = data?.service.status ?? (cloud.state?.bootstrap.status === "error" ? "not_authenticated" : "unknown");
  const install = mergedInstallState(overview, settings, watcher);
  const featureFlags = data?.feature_flags;
  const showInstallAssistant = featureFlags?.install_assistant !== false;
  const showTutorial = featureFlags?.new_user_tutorial !== false;
  const showDiagnostics = featureFlags?.diagnostic_export !== false;
  const showAnnouncements = featureFlags?.announcements !== false;
  const planName = data?.plan.name || "";
  const connectionLabel = cloud.state?.connection?.authenticated
    ? safeText(cloud.state.connection.userLabel, "已登录")
    : "未登录";
  const action = data?.usage.renew_action || data?.plan.commerce_action || null;
  const actionUrl = action?.url || data?.service.support_url || data?.plan.renew_url || "";
  const actionLabel = action?.label || "打开操作页面";
  const canLaunch = Boolean(cloud.state?.managedProvider.active && (status === "available" || status === "low_balance"));
  const announcements = showAnnouncements ? data?.announcements ?? [] : [];
  const authenticated = Boolean(cloud.state?.connection?.authenticated);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [launchDetailOpen, setLaunchDetailOpen] = useState(false);
  const savedEndpoint = cloud.state?.connection?.baseUrl?.trim() || "";
  const realEndpointReady = Boolean(savedEndpoint && !savedEndpoint.startsWith("mock://") && cloud.state?.source !== "fixture");
  const pendingBrowserHandoff = !authenticated && Boolean(cloud.state?.connection?.pendingBrowserHandoff);
  const loginDisabled = !pendingBrowserHandoff && (cloud.pending === "startBrowserHandoff" || !realEndpointReady);
  const launchTone = canLaunch || status === "available" || status === "low_balance" ? "ready" : "warn";
  const launchTitle = canLaunch
    ? "启动 Codex"
    : status === "available"
      ? "准备 Codex"
      : cloudSummary(status);
  const launchDetail = data?.service.message || cloud.state?.bootstrap.message || "刷新后会读取权益、余额和本机状态。";
  const launchActionLabel = canLaunch ? "启动 Codex" : status === "available" ? "准备 Codex" : "刷新状态";
  const launchAction = canLaunch ? actions.launch : status === "available" ? cloud.applyProvider : cloud.refresh;
  const serviceLabel = serviceStatusLabel(status);

  useEffect(() => {
    if (refreshSignal === undefined) return;
    void cloud.refresh(true);
  }, [cloud.refresh, refreshSignal]);

  const copyText = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      await actions.showMessage("已复制", "诊断内容已复制。", "ok");
    } catch (error) {
      await actions.showMessage("复制失败", error instanceof Error ? error.message : String(error), "failed");
    }
  };

  const handleSimpleLogin = async () => {
    if (pendingBrowserHandoff) {
      await cloud.pollBrowserHandoff();
      return;
    }
    if (!realEndpointReady) {
      await actions.showMessage("无法登录", "客户端尚未连接，请联系管理员处理。", "failed");
      return;
    }
    await cloud.startBrowserHandoff({ endpoint: savedEndpoint });
  };

  if (!authenticated) {
    return (
      <div className="cloud-home cloud-user-home">
        <section className="cloud-auth-stage">
          <div className="cloud-simple-login-card">
            <span className="cloud-auth-mark">
              <UserRound className="h-7 w-7" />
            </span>
            <div className="cloud-auth-copy">
              <span>Codex++</span>
              <h1>登录账户</h1>
              <p>
                {pendingBrowserHandoff
                  ? "请在浏览器完成登录，完成后回到这里。"
                  : realEndpointReady
                    ? "登录后查看权益、余额并启动 Codex。"
                    : "客户端尚未连接，请联系管理员。"}
              </p>
            </div>
            <div className="cloud-simple-login-actions">
              <Button
                disabled={loginDisabled || cloud.pending === "pollBrowserHandoff"}
                onClick={() => void handleSimpleLogin()}
              >
                {pendingBrowserHandoff ? <RefreshCw className="h-4 w-4" /> : <ExternalLink className="h-4 w-4" />}
                {pendingBrowserHandoff ? "我已完成" : realEndpointReady ? "登录 Codex++" : "暂不可登录"}
              </Button>
            </div>
            {!realEndpointReady && !pendingBrowserHandoff ? (
              <p className="cloud-simple-login-note">客户端未连接，请联系管理员。</p>
            ) : null}
            {cloud.lastError ? <p className="cloud-login-error">{cloud.lastError}</p> : null}
            <AdvancedModeButton onClick={onOpenAdvancedProviders} />
          </div>
        </section>
      </div>
    );
  }

  return (
    <div className="cloud-home cloud-user-home">
      <section className="cloud-simple-workbench">
        <header className="cloud-simple-titlebar">
          <div className="cloud-simple-title-main">
            <div>
              <span>Codex++</span>
              <h1>{connectionLabel}</h1>
            </div>
          </div>
          <div className="cloud-simple-title-actions">
            <AdvancedModeButton onClick={onOpenAdvancedProviders} />
            <Button onClick={() => void cloud.logout()} variant="outline">
              <LogOut className="h-4 w-4" />
              退出
            </Button>
          </div>
        </header>

        <section className={"cloud-simple-launch tone-" + launchTone}>
          <span className="cloud-simple-launch-icon">
            <Cloud className="h-6 w-6" />
          </span>
          <button
            aria-expanded={launchDetailOpen}
            className={"cloud-simple-launch-copy" + (launchDetailOpen ? " is-open" : "")}
            onClick={() => setLaunchDetailOpen((open) => !open)}
            title={launchDetail}
            type="button"
          >
            <span>快捷启动</span>
            <h2>{launchTitle}</h2>
            <small className="cloud-launch-popover">{launchDetail}</small>
          </button>
          <div className="cloud-simple-launch-actions">
            <Button disabled={cloud.pending === "applyProvider" || cloud.pending === "refresh"} onClick={() => void launchAction()}>
              {canLaunch ? <Play className="h-4 w-4" /> : <RefreshCw className="h-4 w-4" />}
              {launchActionLabel}
            </Button>
            <Button aria-expanded={detailsOpen} onClick={() => setDetailsOpen((open) => !open)} variant="outline">
              <Info className="h-4 w-4" />
              详情
            </Button>
            {actionUrl ? (
              <Button className="cloud-simple-secondary-action" onClick={() => void actions.openExternalUrl(actionUrl)} variant="outline">
                <ExternalLink className="h-4 w-4" />
                {actionLabel}
              </Button>
            ) : null}
          </div>
        </section>

        <div className="cloud-simple-status-strip">
          <CompactStatusItem detail="当前登录账号" label="账户" value={connectionLabel} />
          <CompactStatusItem detail={`到期：${formatDate(data?.plan.expires_at)}`} value={safeText(planName)} />
          <CompactStatusItem detail={`本期用量：${safeText(data?.usage.period_usage_display)}`} label="余额" tone="good" value={safeText(data?.usage.balance_display)} />
          <CompactStatusItem detail={cloudSummary(status)} label="状态" tone={launchTone === "ready" ? "good" : "warn"} value={serviceLabel} />
        </div>

        {cloud.lastError ? <div className="cloud-error-line">{cloud.lastError}</div> : null}

        {detailsOpen ? (
          <div className="cloud-details-stack">
            {announcements.length ? (
              <div className="cloud-model-list">
                {announcements.map((announcement) => (
                  <div className="cloud-model-row" key={announcement.id}>
                    <div>
                      <strong>{safeText(announcement.severity, "公告")}</strong>
                      <span>{announcement.message}</span>
                    </div>
                    {announcement.url ? (
                      <Button onClick={() => void actions.openExternalUrl(announcement.url || "")} size="sm" variant="outline">
                        <ExternalLink className="h-4 w-4" />
                        查看
                      </Button>
                    ) : null}
                  </div>
                ))}
              </div>
            ) : null}
            <CloudStatusPanel
              state={cloud.state}
              pending={cloud.pending}
              onRefresh={() => cloud.refresh()}
              onRegisterDevice={cloud.registerDevice}
              onApplyProvider={cloud.applyProvider}
              onRepairProvider={cloud.repairProvider}
              onLaunch={actions.launch}
              onOpenActionUrl={actions.openExternalUrl}
              onOpenAdvancedProviders={onOpenAdvancedProviders}
            />
            <CloudUsagePanel
              state={cloud.state}
              pending={cloud.pending}
              onRefreshUsage={cloud.refreshUsage}
              onOpenActionUrl={actions.openExternalUrl}
            />
            {showInstallAssistant ? (
              <CloudInstallAssistant
                install={install}
                onCheck={actions.checkHealth}
                onRepairShortcuts={actions.repairShortcuts}
                onRepairBackend={actions.repairBackend}
                onChooseCodexAppPath={actions.chooseCodexAppPath}
                onOpenMaintenance={onOpenMaintenance}
              />
            ) : null}
            {showTutorial ? (
              <CloudTutorialPanel
                state={cloud.state}
                onLaunch={actions.launch}
                onRefreshUsage={cloud.refreshUsage}
                onRepairProvider={cloud.repairProvider}
              />
            ) : null}
            {showDiagnostics ? (
              <CloudDiagnosticsPanel
                state={cloud.state}
                pending={cloud.pending}
                lastError={cloud.lastError}
                onReadDiagnostics={cloud.readDiagnostics}
                onCopy={copyText}
              />
            ) : null}
          </div>
        ) : null}
      </section>
    </div>
  );
}

function AdvancedModeButton({ onClick }: { onClick: () => void }) {
  return (
    <Button className="cloud-advanced-entry" onClick={onClick} title="进入高级模式工作台" variant="outline">
      <SlidersHorizontal className="h-4 w-4" />
      高级模式
    </Button>
  );
}

function CompactStatusItem({
  detail,
  label,
  tone,
  value,
}: {
  detail: string;
  label?: string;
  tone?: "good" | "warn";
  value: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <button
      aria-expanded={open}
      aria-label={label ? `${label}：${value}` : value}
      className={"cloud-simple-status-item" + (tone ? " tone-" + tone : "") + (open ? " is-open" : "")}
      onClick={() => setOpen((current) => !current)}
      title={detail}
      type="button"
    >
      {label ? <span>{label}</span> : null}
      <strong>{value}</strong>
      <small className="cloud-launch-popover">{detail}</small>
    </button>
  );
}
