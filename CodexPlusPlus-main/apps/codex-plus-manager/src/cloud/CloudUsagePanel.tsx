import { ExternalLink, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { CloudActionName } from "./useCloudRuntime";
import type { CloudModelSummary, CloudRuntimeState } from "./types";

type Props = {
  state: CloudRuntimeState | null;
  pending: CloudActionName | null;
  onRefreshUsage: () => Promise<unknown>;
  onOpenActionUrl: (url: string) => Promise<unknown>;
};

function safe(value: string | null | undefined, fallback = "未知") {
  return value && value.trim() ? value : fallback;
}

function usageState(value: string | null | undefined) {
  const text = (value || "").trim().toLowerCase();
  if (!text || text === "ok" || text === "normal" || text === "none") return "正常";
  if (text === "limited" || text === "rate_limited") return "繁忙";
  if (text === "blocked") return "不可用";
  return value || "正常";
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
    return [{ label: safe(action?.label, "打开操作页面"), url }];
  });
}

function fallbackDefaultModel(state: CloudRuntimeState | null): CloudModelSummary | null {
  const model = state?.bootstrap.data?.provider.default_model || state?.managedProvider.defaultModel || "";
  if (!model.trim()) return null;
  return {
    model_id: model,
    route_model: model,
    label: model,
    is_default: true,
    is_available: state?.bootstrap.data?.service.status !== "model_unavailable",
    disabled_reason: state?.bootstrap.data?.service.status === "model_unavailable" ? state.bootstrap.data.service.message : null,
  };
}

export function CloudUsagePanel({ state, pending, onRefreshUsage, onOpenActionUrl }: Props) {
  const data = state?.bootstrap.data;
  const usage = data?.usage;
  const operationActions = uniqueActions([usage?.renew_action, data?.plan.commerce_action]);
  const models = data?.models?.length ? data.models : fallbackDefaultModel(state) ? [fallbackDefaultModel(state)!] : [];
  const defaultModel = models.find((model) => model.is_default) || fallbackDefaultModel(state);
  const planName = data?.plan.name || "";
  const usageNotice =
    usage?.low_balance ||
    data?.service.status === "rate_limited" ||
    data?.service.status === "model_unavailable" ||
    data?.service.status === "gateway_unhealthy"
      ? data?.service.message || state?.bootstrap.message || ""
      : "";

  return (
    <Card className="cloud-panel">
      <CardHeader className="cloud-panel-head">
        <div>
          <CardTitle>用量与模型状态</CardTitle>
          <CardDescription>{safe(data?.service.message, "这里仅显示服务端返回的摘要，不在本地计算额度。")}</CardDescription>
        </div>
        <Button disabled={pending === "usage"} onClick={() => void onRefreshUsage()} size="sm" variant="outline">
          <RefreshCw className="h-4 w-4" />
          刷新
        </Button>
      </CardHeader>
      <CardContent>
        <div className="cloud-usage-grid">
          <div className="cloud-usage-card">
            <span>套餐</span>
            <strong>{safe(planName)}</strong>
          </div>
          <div className="cloud-usage-card">
            <span>到期时间</span>
            <strong>{formatDate(data?.plan.expires_at)}</strong>
          </div>
          <div className="cloud-usage-card">
            <span>剩余</span>
            <strong>{safe(usage?.balance_display)}</strong>
          </div>
          <div className="cloud-usage-card">
            <span>今日/本期用量</span>
            <strong>{safe(usage?.period_usage_display)}</strong>
          </div>
          <div className="cloud-usage-card">
            <span>限流状态</span>
            <strong>{usageState(usage?.rate_limit_state)}</strong>
          </div>
          <div className="cloud-usage-card">
            <span>默认模型</span>
            <strong>{safe(defaultModel?.label || defaultModel?.model_id)}</strong>
          </div>
        </div>
        {usageNotice ? <div className={usage?.low_balance ? "cloud-error-line" : "cloud-empty"}>{usageNotice}</div> : null}
        {models.length ? (
          <div className="cloud-model-list">
            {models.map((model) => (
              <div className="cloud-model-row" key={`${model.model_id}:${model.route_model}`}>
                <div>
                  <strong>{model.label || model.model_id}</strong>
                  <span>
                    {model.is_default ? "默认模型" : "可用模型"}
                  </span>
                </div>
                <em className={model.is_available ? "good" : "bad"}>
                  {model.is_available ? "可用" : safe(model.disabled_reason, "不可用")}
                </em>
              </div>
            ))}
          </div>
        ) : (
          <div className="cloud-empty">当前状态没有可展示的模型摘要。</div>
        )}
        {operationActions.length ? (
          <div className="cloud-secondary-actions">
            {operationActions.map((action) => (
              <Button key={action.url} onClick={() => void onOpenActionUrl(action.url)} variant="outline">
                <ExternalLink className="h-4 w-4" />
                {action.label}
              </Button>
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
