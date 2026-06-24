import { ExternalLink, LogOut, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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

export function CloudLoginPanel({
  state,
  pending,
  onStartBrowserHandoff,
  onPollBrowserHandoff,
  onLogout,
}: Props) {
  const isAuthenticated = Boolean(state?.connection?.authenticated);
  const pendingBrowserHandoff = !isAuthenticated && Boolean(state?.connection?.pendingBrowserHandoff);
  const savedEndpoint = state?.connection?.baseUrl?.trim() || "";
  const realEndpointReady = Boolean(savedEndpoint && !savedEndpoint.startsWith("mock://") && state?.source !== "fixture");
  const userLabel = state?.connection?.userLabel?.trim() || "已登录";
  const loginDisabled = !pendingBrowserHandoff && (pending === "startBrowserHandoff" || !realEndpointReady);

  const handleLogin = async () => {
    if (pendingBrowserHandoff) {
      await onPollBrowserHandoff();
      return;
    }
    if (!realEndpointReady) return;
    await onStartBrowserHandoff({ endpoint: savedEndpoint });
  };

  return (
    <Card className="cloud-panel cloud-login-panel-simple">
      <CardHeader className="cloud-panel-head">
        <div>
          <CardTitle>登录账户</CardTitle>
          <CardDescription>
            {isAuthenticated
              ? userLabel
              : pendingBrowserHandoff
                ? "请在浏览器完成登录，完成后回到这里。"
                : "登录后查看套餐、余额并启动 Codex。"}
          </CardDescription>
        </div>
      </CardHeader>
      <CardContent className="cloud-login-grid cloud-login-simple">
        <div className="cloud-login-actions">
          {isAuthenticated ? (
            <Button disabled={pending === "logout"} onClick={() => void onLogout()} variant="outline">
              <LogOut className="h-4 w-4" />
              退出
            </Button>
          ) : (
            <>
              <Button disabled={loginDisabled} onClick={() => void handleLogin()}>
                {pendingBrowserHandoff ? <RefreshCw className="h-4 w-4" /> : <ExternalLink className="h-4 w-4" />}
                {pendingBrowserHandoff ? "我已完成" : realEndpointReady ? "登录 Codex++" : "暂不可登录"}
              </Button>
            </>
          )}
        </div>
        {!isAuthenticated && !realEndpointReady && !pendingBrowserHandoff ? (
          <p className="cloud-simple-login-note">客户端未连接，请联系管理员。</p>
        ) : null}
      </CardContent>
    </Card>
  );
}
