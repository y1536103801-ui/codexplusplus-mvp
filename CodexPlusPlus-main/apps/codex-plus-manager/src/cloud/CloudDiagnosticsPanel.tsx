import { Clipboard, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { CloudActionName } from "./useCloudRuntime";
import type { CloudRuntimeState } from "./types";

type Props = {
  state: CloudRuntimeState | null;
  pending: CloudActionName | null;
  lastError: string | null;
  onReadDiagnostics: () => Promise<unknown>;
  onCopy: (text: string) => Promise<unknown>;
};

export function CloudDiagnosticsPanel({ state, pending, lastError, onReadDiagnostics, onCopy }: Props) {
  const diagnostics = state?.diagnostics;
  const report = diagnostics?.report || "";

  return (
    <Card className="cloud-panel">
      <CardHeader className="cloud-panel-head">
        <div>
          <CardTitle>诊断</CardTitle>
          <CardDescription>仅展示已脱敏信息，用于排查登录、设备和本地写入问题。</CardDescription>
        </div>
        <Button disabled={pending === "diagnostics"} onClick={() => void onReadDiagnostics()} size="sm" variant="outline">
          <RefreshCw className="h-4 w-4" />
          生成
        </Button>
      </CardHeader>
      <CardContent>
        {lastError ? <div className="cloud-error-line">{lastError}</div> : null}
        {report ? (
          <>
            <pre className="cloud-diagnostics">{report}</pre>
            <div className="cloud-secondary-actions">
              <Button onClick={() => void onCopy(report)} variant="secondary">
                <Clipboard className="h-4 w-4" />
                复制诊断
              </Button>
            </div>
          </>
        ) : (
          <div className="cloud-empty">点击生成后显示脱敏诊断。</div>
        )}
      </CardContent>
    </Card>
  );
}
