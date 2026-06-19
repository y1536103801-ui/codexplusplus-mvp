import { Cloud, ExternalLink, Play, RefreshCw, RotateCw, Wrench } from "lucide-react";
import { useCallback, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { CloudDiagnosticsPanel } from "./CloudDiagnosticsPanel";
import { CloudInstallAssistant } from "./CloudInstallAssistant";
import { CloudLoginPanel } from "./CloudLoginPanel";
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
  if (stateStatus === "not_authenticated") return "登录后自动配置 Codex++ Cloud。";
  if (stateStatus === "not_purchased") return "账号有效，但服务尚未开通。";
  if (stateStatus === "expired") return "服务期已结束，请使用服务端提供的操作入口。";
  if (stateStatus === "disabled") return "云服务当前不可用，请查看服务端提示。";
  if (stateStatus === "low_balance") return "用量状态需要关注，仍可按服务端策略继续。";
  if (stateStatus === "device_revoked") return "本机设备已被停用，请联系管理员。";
  if (stateStatus === "model_unavailable") return "默认模型暂不可用，请刷新或按提示处理。";
  if (stateStatus === "rate_limited") return "当前请求受到限流，请按服务端提示稍后重试。";
  if (stateStatus === "gateway_unhealthy") return "云服务可达，但模型网关暂不可用。";
  if (stateStatus === "local_codex_missing") return "尚未找到本机 Codex，请先完成安装配置。";
  if (stateStatus === "local_config_failed") return "云账号正常，本地 Codex 配置写入失败。";
  if (stateStatus === "stale_snapshot") return "本地配置快照已过期，请刷新云状态。";
  return "正在读取云服务状态。";
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
  const defaultModel =
    data?.models.find((model) => model.is_default)?.label ||
    data?.provider.default_model ||
    cloud.state?.managedProvider.defaultModel ||
    "";
  const planName = data?.plan.name || data?.plan.status || "";
  const connectionLabel = cloud.state?.connection?.authenticated
    ? safeText(cloud.state.connection.userLabel, "已登录")
    : "未登录";
  const action = data?.usage.renew_action || data?.plan.commerce_action || null;
  const actionUrl = action?.url || data?.service.support_url || data?.plan.renew_url || "";
  const actionLabel = action?.label || "打开操作页面";
  const canLaunch = Boolean(cloud.state?.managedProvider.active && (status === "available" || status === "low_balance"));
  const announcements = showAnnouncements ? data?.announcements ?? [] : [];

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

  return (
    <div className="cloud-home">
      <Card className="cloud-hero">
        <CardContent>
          <div className="cloud-hero-layout">
            <div className="cloud-hero-mark">
              <Cloud className="h-6 w-6" />
            </div>
            <div>
              <span className="cloud-kicker">Codex++ Cloud</span>
              <h2>{cloudSummary(status)}</h2>
              <p>{data?.service.message || cloud.state?.bootstrap.message || "打开 Manager 后会先检查登录、权益、安装和托管供应商状态。"}</p>
            </div>
            <div className="cloud-hero-actions">
              <Button disabled={!canLaunch} onClick={() => void actions.launch()}>
                <Play className="h-4 w-4" />
                启动 Codex
              </Button>
              <Button disabled={cloud.pending === "refresh"} onClick={() => void cloud.refresh()} variant="secondary">
                <RefreshCw className="h-4 w-4" />
                刷新云状态
              </Button>
              <Button onClick={() => void actions.checkHealth()} variant="outline">
                <RotateCw className="h-4 w-4" />
                检查本机
              </Button>
              <Button onClick={onOpenMaintenance} variant="outline">
                <Wrench className="h-4 w-4" />
                诊断/修复
              </Button>
              {actionUrl ? (
                <Button onClick={() => void actions.openExternalUrl(actionUrl)} variant="outline">
                  <ExternalLink className="h-4 w-4" />
                  {actionLabel}
                </Button>
              ) : null}
              {cloud.state?.source === "fixture" ? (
                <Button disabled={cloud.pending === "fixture"} onClick={() => void cloud.showNextFixtureState()} variant="ghost">
                  切换 Mock 状态
                </Button>
              ) : null}
            </div>
          </div>
          <div className="cloud-status-grid">
            <HomeSummaryCell title="登录状态" value={connectionLabel} />
            <HomeSummaryCell title="套餐" value={safeText(planName)} />
            <HomeSummaryCell title="到期时间" value={formatDate(data?.plan.expires_at)} />
            <HomeSummaryCell title="余额" value={safeText(data?.usage.balance_display)} />
            <HomeSummaryCell title="今日/本期用量" value={safeText(data?.usage.period_usage_display)} />
            <HomeSummaryCell title="默认模型" value={safeText(defaultModel)} mono />
          </div>
          {cloud.lastError ? <div className="cloud-error-line">{cloud.lastError}</div> : null}
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
        </CardContent>
      </Card>
      <div className="cloud-grid primary">
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
        <CloudLoginPanel
          state={cloud.state}
          pending={cloud.pending}
          onConfigureEndpoint={(endpoint) => cloud.configureEndpoint({ endpoint })}
          onStartBrowserHandoff={cloud.startBrowserHandoff}
          onPollBrowserHandoff={cloud.pollBrowserHandoff}
          onCancelBrowserHandoff={cloud.cancelBrowserHandoff}
          onOpenExternalUrl={actions.openExternalUrl}
          onLogin={cloud.login}
          onLogin2FA={cloud.login2FA}
          onRedeem={(code) => cloud.redeem({ code })}
          onLogout={cloud.logout}
        />
      </div>
      <div className="cloud-grid">
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
      </div>
      <div className="cloud-grid">
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
    </div>
  );
}

function HomeSummaryCell({ title, value, mono = false }: { title: string; value: string; mono?: boolean }) {
  return (
    <div className="cloud-status-cell">
      <span>{title}</span>
      <strong className={mono ? "cloud-mono" : undefined}>{value}</strong>
    </div>
  );
}
