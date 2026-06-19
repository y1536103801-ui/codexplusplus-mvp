import { AlertTriangle, CheckCircle2, Cloud, ExternalLink, Play, RefreshCw, ShieldAlert, Wrench } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { CloudActionName } from "./useCloudRuntime";
import type { CloudRuntimeState } from "./types";

type Props = {
  state: CloudRuntimeState | null;
  pending: CloudActionName | null;
  onRefresh: () => Promise<unknown>;
  onRegisterDevice: () => Promise<unknown>;
  onApplyProvider: () => Promise<unknown>;
  onRepairProvider: () => Promise<unknown>;
  onLaunch: () => Promise<unknown>;
  onOpenActionUrl: (url: string) => Promise<unknown>;
  onOpenAdvancedProviders: () => void;
};

const statusLabels: Record<string, string> = {
  available: "服务可用",
  not_authenticated: "未登录",
  not_purchased: "未开通",
  expired: "已过期",
  low_balance: "余额偏低",
  disabled: "服务停用",
  device_revoked: "设备停用",
  model_unavailable: "模型不可用",
  rate_limited: "限流中",
  gateway_unhealthy: "网关异常",
  local_codex_missing: "未找到 Codex",
  local_config_failed: "本地配置失败",
  stale_snapshot: "快照过期",
  unknown: "状态未知",
};

const statusFallbacks: Record<string, string> = {
  available: "Codex++ Cloud 已可用。",
  not_authenticated: "请先登录 Codex++ Cloud。",
  not_purchased: "当前账号尚未开通服务。",
  expired: "当前套餐已过期。",
  low_balance: "服务端提示余额需要关注。",
  disabled: "服务端当前停用了云服务。",
  device_revoked: "本机设备不可继续使用云服务。",
  model_unavailable: "默认模型当前不可用。",
  rate_limited: "当前请求受到服务端限流。",
  gateway_unhealthy: "模型网关暂不可用。",
  local_codex_missing: "本机 Codex 安装状态需要处理。",
  local_config_failed: "本地托管供应商配置写入失败。",
  stale_snapshot: "本地云配置快照需要刷新。",
  unknown: "正在读取云服务状态。",
};

function statusTone(status: string | undefined) {
  if (status === "available") return "good";
  if (
    status === "not_authenticated" ||
    status === "not_purchased" ||
    status === "expired" ||
    status === "disabled" ||
    status === "device_revoked"
  ) {
    return "bad";
  }
  if (
    status === "local_config_failed" ||
    status === "gateway_unhealthy" ||
    status === "low_balance" ||
    status === "model_unavailable" ||
    status === "rate_limited" ||
    status === "local_codex_missing" ||
    status === "stale_snapshot"
  ) {
    return "warn";
  }
  return "muted";
}

function statusIcon(status: string | undefined) {
  if (status === "available") return <CheckCircle2 className="h-5 w-5" />;
  if (status === "device_revoked") return <ShieldAlert className="h-5 w-5" />;
  if (status === "local_config_failed" || status === "local_codex_missing" || status === "stale_snapshot") return <Wrench className="h-5 w-5" />;
  return <AlertTriangle className="h-5 w-5" />;
}

function safeText(value: string | null | undefined, fallback = "未知") {
  return value && value.trim() ? value : fallback;
}

function statusLabel(status: string | undefined) {
  return statusLabels[status || ""] || safeText(status, "状态未知");
}

function serviceMessage(status: string | undefined, message: string | null | undefined, bootstrapMessage: string | null | undefined) {
  return safeText(message || bootstrapMessage, statusFallbacks[status || ""] || statusFallbacks.unknown);
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

function uniqueActions(
  actions: Array<{
    label: string;
    url: string | null;
  } | null | undefined>,
) {
  const seen = new Set<string>();
  return actions.flatMap((action) => {
    const url = action?.url?.trim();
    if (!url || seen.has(url)) return [];
    seen.add(url);
    return [{ label: safeText(action?.label, "打开操作页面"), url }];
  });
}

export function CloudStatusPanel({
  state,
  pending,
  onRefresh,
  onRegisterDevice,
  onApplyProvider,
  onRepairProvider,
  onLaunch,
  onOpenActionUrl,
  onOpenAdvancedProviders,
}: Props) {
  const data = state?.bootstrap.data;
  const service = data?.service;
  const status = service?.status ?? (state?.bootstrap.status === "error" ? "not_authenticated" : "unknown");
  const operationActions = uniqueActions([
    data?.usage.renew_action,
    data?.plan.commerce_action,
    service?.support_url ? { label: "打开操作页面", url: service.support_url } : null,
    data?.plan.renew_url ? { label: "打开操作页面", url: data.plan.renew_url } : null,
  ]);
  const applyBlockedStatuses = new Set(["not_authenticated", "not_purchased", "expired", "disabled", "device_revoked", "gateway_unhealthy"]);
  const canApply = Boolean(data && !applyBlockedStatuses.has(status));
  const canLaunch = state?.managedProvider.active && (status === "available" || status === "low_balance");
  const defaultModel =
    data?.models.find((model) => model.is_default)?.label ||
    data?.provider.default_model ||
    state?.managedProvider.defaultModel ||
    "";
  const loginStatus = state?.connection?.authenticated ? safeText(state.connection.userLabel, "已登录") : "未登录";
  const planName = data?.plan.name || data?.plan.status || "";

  return (
    <Card className={`cloud-panel cloud-status-panel tone-${statusTone(status)}`}>
      <CardHeader className="cloud-panel-head">
        <div>
          <CardTitle>Codex++ Cloud</CardTitle>
          <CardDescription>当前账号、设备和托管供应商状态。</CardDescription>
        </div>
        <span className={`cloud-status-icon ${statusTone(status)}`}>{statusIcon(status)}</span>
      </CardHeader>
      <CardContent>
        <div className="cloud-status-main">
          <div>
            <span className="cloud-kicker">{statusLabel(status)}</span>
            <h2>{serviceMessage(status, service?.message, state?.bootstrap.message)}</h2>
          </div>
          <div className="cloud-action-strip">
            <Button disabled={pending === "refresh"} onClick={() => void onRefresh()} variant="secondary">
              <RefreshCw className="h-4 w-4" />
              刷新
            </Button>
            <Button disabled={pending === "applyProvider" || !canApply} onClick={() => void onApplyProvider()}>
              <Cloud className="h-4 w-4" />
              配置云服务
            </Button>
            <Button disabled={pending === "repairProvider"} onClick={() => void onRepairProvider()} variant="secondary">
              <Wrench className="h-4 w-4" />
              修复配置
            </Button>
            <Button disabled={!canLaunch} onClick={() => void onLaunch()} variant={canLaunch ? "default" : "outline"}>
              <Play className="h-4 w-4" />
              启动 Codex
            </Button>
          </div>
        </div>
        <div className="cloud-status-grid">
          <CloudStatusCell title="登录状态" value={loginStatus} />
          <CloudStatusCell title="服务状态" value={statusLabel(status)} />
          <CloudStatusCell title="套餐" value={safeText(planName)} />
          <CloudStatusCell title="到期时间" value={formatDate(data?.plan.expires_at)} />
          <CloudStatusCell title="余额" value={safeText(data?.usage.balance_display)} />
          <CloudStatusCell title="今日/本期用量" value={safeText(data?.usage.period_usage_display)} />
          <CloudStatusCell title="默认模型" value={safeText(defaultModel)} mono />
          <CloudStatusCell title="设备" value={safeText(data?.device.message || data?.device.status)} />
          <CloudStatusCell title="供应商" value={state?.managedProvider.active ? "已配置 Codex++ Cloud" : "尚未写入托管配置"} />
          <CloudStatusCell title="限流状态" value={safeText(data?.usage.rate_limit_state)} />
          <CloudStatusCell title="配置版本" value={safeText(data?.version_policy.config_version)} mono />
          <CloudStatusCell title="快照版本" value={safeText(data?.version_policy.snapshot_version)} mono />
          <CloudStatusCell title="Key 状态" value={state?.managedProvider.hasApiKey ? safeText(state.managedProvider.maskedKey) : "未下发"} mono />
        </div>
        {service?.error_code ? <div className="cloud-error-line">{service.error_code}</div> : null}
        <div className="cloud-secondary-actions">
          <Button disabled={pending === "registerDevice" || status === "device_revoked"} onClick={() => void onRegisterDevice()} variant="outline">
            注册本机
          </Button>
          {operationActions.map((action) => (
            <Button key={action.url} onClick={() => void onOpenActionUrl(action.url)} variant="outline">
              <ExternalLink className="h-4 w-4" />
              {action.label}
            </Button>
          ))}
          {data?.feature_flags.advanced_provider_config ? (
            <Button onClick={onOpenAdvancedProviders} variant="ghost">
              高级供应商配置
            </Button>
          ) : null}
        </div>
      </CardContent>
    </Card>
  );
}

function CloudStatusCell({ title, value, mono = false }: { title: string; value: string; mono?: boolean }) {
  return (
    <div className="cloud-status-cell">
      <span>{title}</span>
      <strong className={mono ? "cloud-mono" : undefined}>{value}</strong>
    </div>
  );
}
