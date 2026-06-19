import { Copy, ExternalLink, LogIn, LogOut, RefreshCw, Send, ShieldCheck, Ticket, X } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { CloudActionName } from "./useCloudRuntime";
import type { CloudRuntimeState } from "./types";

type Props = {
  state: CloudRuntimeState | null;
  pending: CloudActionName | null;
  onConfigureEndpoint: (endpoint: string) => Promise<unknown>;
  onStartBrowserHandoff: (request: { endpoint: string }) => Promise<CloudRuntimeState | null>;
  onPollBrowserHandoff: () => Promise<CloudRuntimeState | null>;
  onCancelBrowserHandoff: () => Promise<CloudRuntimeState | null>;
  onOpenExternalUrl: (url: string) => Promise<void>;
  onLogin: (request: { endpoint: string; account: string; password: string }) => Promise<unknown>;
  onLogin2FA: (request: { totpCode: string }) => Promise<unknown>;
  onRedeem: (code: string) => Promise<unknown>;
  onLogout: () => Promise<unknown>;
};

function formatExpiration(value: string | null | undefined): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString("zh-CN", { hour12: false });
}

export function CloudLoginPanel({
  state,
  pending,
  onConfigureEndpoint,
  onStartBrowserHandoff,
  onPollBrowserHandoff,
  onCancelBrowserHandoff,
  onOpenExternalUrl,
  onLogin,
  onLogin2FA,
  onRedeem,
  onLogout,
}: Props) {
  const [endpoint, setEndpoint] = useState("");
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [twoFactorCode, setTwoFactorCode] = useState("");
  const [redeemCode, setRedeemCode] = useState("");
  const service = state?.bootstrap.data?.service;
  const device = state?.bootstrap.data?.device;
  const serviceStatus = service?.status ?? "not_authenticated";
  const pendingTwoFactor = Boolean(state?.connection?.pendingTwoFactor);
  const isAuthenticated = Boolean(state?.connection?.authenticated) && !pendingTwoFactor;
  const canLogout = isAuthenticated || pendingTwoFactor;
  const userLabel = state?.connection?.userLabel?.trim();
  const pendingTwoFactorLabel = state?.connection?.pendingTwoFactorUserLabel?.trim();
  const pendingBrowserHandoff = !isAuthenticated && Boolean(state?.connection?.pendingBrowserHandoff);
  const browserHandoffUrl = state?.connection?.browserHandoffAuthorizeUrl?.trim() || "";
  const rawBrowserHandoffVerificationCode = state?.connection?.browserHandoffVerificationCode?.trim() || "";
  const browserHandoffVerificationCode = /^\d{6}$/.test(rawBrowserHandoffVerificationCode)
    ? rawBrowserHandoffVerificationCode
    : "";
  const browserHandoffExpiresAt = formatExpiration(state?.connection?.browserHandoffExpiresAt);
  const browserHandoffPollIntervalSeconds = state?.connection?.browserHandoffPollIntervalSeconds;
  const savedEndpoint = state?.connection?.baseUrl?.trim() || "";
  const activeEndpoint = endpoint.trim() || savedEndpoint;
  const normalizedTwoFactorCode = twoFactorCode.trim();
  const canSubmitTwoFactorCode = /^\d{6}$/.test(normalizedTwoFactorCode);
  const deviceRevoked = serviceStatus === "device_revoked" || device?.status === "revoked";
  const deviceRevokedMessage = service?.message || device?.message || "当前设备需要重新绑定或联系支持。";

  const startBrowserLogin = async () => {
    if (!activeEndpoint) return;
    const nextState = await onStartBrowserHandoff({ endpoint: activeEndpoint });
    const url = nextState?.connection?.browserHandoffAuthorizeUrl?.trim();
    if (url) {
      await onOpenExternalUrl(url);
    }
  };

  const handleBrowserLogin = async () => {
    if (pendingBrowserHandoff && browserHandoffUrl) {
      await onOpenExternalUrl(browserHandoffUrl);
      return;
    }
    await startBrowserLogin();
  };

  const handleCopyBrowserHandoffUrl = async () => {
    if (!browserHandoffUrl || typeof navigator === "undefined") return;
    await navigator.clipboard?.writeText(browserHandoffUrl);
  };

  return (
    <Card className="cloud-panel">
      <CardHeader className="cloud-panel-head">
        <div>
          <CardTitle>登录绑定</CardTitle>
          <CardDescription>默认使用浏览器完成登录、安全校验和本机授权。</CardDescription>
        </div>
        <span className="cloud-source">{state?.source === "fixture" ? "Mock" : "Runtime"}</span>
      </CardHeader>
      <CardContent className="cloud-login-grid">
        <div className="cloud-form-block cloud-browser-login">
          <Label>浏览器登录</Label>
          {savedEndpoint ? <p className="cloud-muted">服务地址：{savedEndpoint}</p> : null}
          {!savedEndpoint ? (
            <div className="cloud-form-block">
              <Label htmlFor="cloud-endpoint-primary">服务地址</Label>
              <div className="cloud-input-action">
                <Input
                  id="cloud-endpoint-primary"
                  value={endpoint}
                  onChange={(event) => setEndpoint(event.currentTarget.value)}
                  placeholder="输入 Codex++ Cloud 服务地址"
                />
                <Button
                  disabled={!endpoint.trim() || pending === "configureEndpoint"}
                  onClick={() => void onConfigureEndpoint(endpoint.trim())}
                  variant="secondary"
                >
                  <Send className="h-4 w-4" />
                  检查
                </Button>
              </div>
            </div>
          ) : null}
          {isAuthenticated ? (
            <div className="cloud-form-block">
              <Label>当前账号</Label>
              <p className="cloud-muted">{userLabel || "已登录账号"}</p>
            </div>
          ) : null}
          {deviceRevoked ? <p className="cloud-muted">{deviceRevokedMessage}</p> : null}
          <div className="cloud-login-actions">
            <Button
              disabled={
                (!activeEndpoint && !browserHandoffUrl) ||
                pending === "startBrowserHandoff" ||
                isAuthenticated ||
                pendingTwoFactor
              }
              onClick={() => void handleBrowserLogin()}
            >
              <ExternalLink className="h-4 w-4" />
              {pendingBrowserHandoff ? "重新打开浏览器" : "用浏览器登录"}
            </Button>
            <Button disabled={!canLogout || pending === "logout"} onClick={() => void onLogout()} variant="outline">
              <LogOut className="h-4 w-4" />
              退出
            </Button>
            {deviceRevoked && service?.support_url ? (
              <Button onClick={() => void onOpenExternalUrl(service.support_url || "")} variant="ghost">
                <ExternalLink className="h-4 w-4" />
                支持链接
              </Button>
            ) : null}
          </div>
          <p className="cloud-muted">
            {pendingBrowserHandoff
              ? "请在浏览器授权页核对确认码，完成后回到这里检查状态。"
              : "浏览器登录是生产默认路径，可完成网页安全校验后绑定当前桌面端。"}
          </p>
        </div>
        {pendingBrowserHandoff ? (
          <div className="cloud-form-block cloud-handoff-status">
            <Label>等待浏览器授权</Label>
            {browserHandoffVerificationCode ? (
              <div className="cloud-handoff-code">
                <span>确认码</span>
                <strong>{browserHandoffVerificationCode}</strong>
              </div>
            ) : null}
            {browserHandoffExpiresAt ? <p className="cloud-muted">授权有效期至：{browserHandoffExpiresAt}</p> : null}
            {browserHandoffPollIntervalSeconds ? (
              <p className="cloud-muted">建议间隔 {browserHandoffPollIntervalSeconds} 秒后检查授权状态。</p>
            ) : null}
            <div className="cloud-form-block">
              <Label htmlFor="cloud-browser-handoff-url">授权链接</Label>
              <div className="cloud-input-action">
                <Input
                  id="cloud-browser-handoff-url"
                  readOnly
                  value={browserHandoffUrl}
                  onFocus={(event) => event.currentTarget.select()}
                  placeholder="授权链接尚未返回"
                />
                <Button
                  disabled={!browserHandoffUrl}
                  onClick={() => void onOpenExternalUrl(browserHandoffUrl)}
                  variant="secondary"
                >
                  <ExternalLink className="h-4 w-4" />
                  打开
                </Button>
              </div>
            </div>
            <div className="cloud-login-actions">
              <Button
                disabled={!browserHandoffUrl}
                onClick={() => void handleCopyBrowserHandoffUrl()}
                variant="outline"
              >
                <Copy className="h-4 w-4" />
                复制链接
              </Button>
              <Button
                disabled={pending === "pollBrowserHandoff"}
                onClick={() => void onPollBrowserHandoff()}
                variant="secondary"
              >
                <RefreshCw className="h-4 w-4" />
                检查状态
              </Button>
              <Button
                disabled={pending === "cancelBrowserHandoff"}
                onClick={() => void onCancelBrowserHandoff()}
                variant="ghost"
              >
                <X className="h-4 w-4" />
                取消
              </Button>
            </div>
          </div>
        ) : null}
        {pendingTwoFactor ? (
          <div className="cloud-form-block">
            <Label htmlFor="cloud-two-factor">双重验证码</Label>
            <p className="cloud-muted">
              {pendingTwoFactorLabel ? `${pendingTwoFactorLabel} 正在等待双重验证。` : "账号正在等待双重验证。"}
              验证完成前不会视为已登录。
            </p>
            <div className="cloud-input-action">
              <Input
                id="cloud-two-factor"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={6}
                value={twoFactorCode}
                onChange={(event) => setTwoFactorCode(event.currentTarget.value.replace(/\D/g, "").slice(0, 6))}
                placeholder={pendingTwoFactorLabel ? `${pendingTwoFactorLabel} 的验证码` : "6 位动态验证码"}
              />
              <Button
                disabled={!canSubmitTwoFactorCode || pending === "login2fa"}
                onClick={() => void onLogin2FA({ totpCode: normalizedTwoFactorCode })}
                variant="secondary"
              >
                <ShieldCheck className="h-4 w-4" />
                验证
              </Button>
            </div>
          </div>
        ) : null}
        <details className="cloud-form-block cloud-advanced-login">
          <summary>兼容账号密码登录</summary>
          <div className="cloud-advanced-login-body">
            <div className="cloud-form-block">
              <Label htmlFor="cloud-endpoint">服务地址</Label>
              <div className="cloud-input-action">
                <Input
                  id="cloud-endpoint"
                value={endpoint}
                onChange={(event) => setEndpoint(event.currentTarget.value)}
                  placeholder={savedEndpoint || "由服务端或管理员提供"}
                />
                <Button
                  disabled={!endpoint.trim() || pending === "configureEndpoint"}
                  onClick={() => void onConfigureEndpoint(endpoint.trim())}
                  variant="secondary"
                >
                  <Send className="h-4 w-4" />
                  检查
                </Button>
              </div>
            </div>
            <div className="cloud-form-block">
              <Label htmlFor="cloud-account">邮箱</Label>
              <Input
                id="cloud-account"
                value={account}
                onChange={(event) => setAccount(event.currentTarget.value)}
                placeholder="账号邮箱"
              />
            </div>
            <div className="cloud-form-block">
              <Label htmlFor="cloud-password">密码</Label>
              <Input
                id="cloud-password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.currentTarget.value)}
                placeholder="仅用于兼容模式"
              />
            </div>
            <div className="cloud-login-actions">
              <Button
                disabled={
                  !activeEndpoint ||
                  !account.trim() ||
                  !password.trim() ||
                  pendingTwoFactor ||
                  pending === "login" ||
                  pending === "login2fa"
                }
                onClick={() => {
                  setTwoFactorCode("");
                  void onLogin({ endpoint: activeEndpoint, account: account.trim(), password });
                }}
                variant="secondary"
              >
                <LogIn className="h-4 w-4" />
                兼容登录
              </Button>
            </div>
          </div>
        </details>
        <div className="cloud-form-block cloud-redeem">
          <Label htmlFor="cloud-redeem">激活码</Label>
          <div className="cloud-input-action">
            <Input
              id="cloud-redeem"
              value={redeemCode}
              onChange={(event) => setRedeemCode(event.currentTarget.value)}
              placeholder="输入后绑定到当前账号"
            />
            <Button
              disabled={!redeemCode.trim() || pending === "redeem"}
              onClick={() => void onRedeem(redeemCode.trim())}
              variant="secondary"
            >
              <Ticket className="h-4 w-4" />
              兑换
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
