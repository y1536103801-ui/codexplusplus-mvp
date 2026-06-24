import { CheckCircle2, FolderOpen, RefreshCw, Wrench, XCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import type { CloudInstallState } from "./types";

type Props = {
  install: CloudInstallState;
  onCheck: () => Promise<unknown>;
  onRepairShortcuts: () => Promise<unknown>;
  onRepairBackend: () => Promise<unknown>;
  onChooseCodexAppPath: (mode: "folder" | "file") => Promise<unknown>;
  onOpenMaintenance: () => void;
};

function isOk(status: string | null | undefined) {
  const normalized = normalizeStatus(status);
  return ["ok", "success", "installed", "running", "found", "enabled", "saved", "auto", "info"].includes(normalized);
}

function normalizeStatus(status: string | null | undefined) {
  return (status || "").trim().toLowerCase();
}

function isNotChecked(status: string | null | undefined) {
  const normalized = normalizeStatus(status);
  return !normalized || normalized === "not_checked";
}

function isMissing(status: string | null | undefined) {
  return ["missing", "not_found", "not_installed", "absent"].includes(normalizeStatus(status));
}

function isPermissionIssue(status: string | null | undefined) {
  const normalized = normalizeStatus(status);
  return normalized.includes("permission") || normalized.includes("denied") || normalized.includes("gatekeeper");
}

function localStatusLabel(status: string | null | undefined) {
  const normalized = normalizeStatus(status);
  if (!normalized || normalized === "not_checked") return "未检查";
  if (normalized === "info") return "提示";
  if (normalized === "saved") return "已保存";
  if (normalized === "auto") return "自动探测";
  if (isOk(status)) return "正常";
  if (normalized === "disabled") return "未启用";
  if (normalized === "invalid_path") return "路径异常";
  if (isMissing(status)) return "未安装";
  if (isPermissionIssue(status)) return "权限受限";
  if (normalized === "failed" || normalized === "error") return "检查失败";
  return "需要处理";
}

export function CloudInstallAssistant({
  install,
  onCheck,
  onRepairShortcuts,
  onRepairBackend,
  onChooseCodexAppPath,
  onOpenMaintenance,
}: Props) {
  const summary = buildInstallSummary(install);
  const savedPathStatus = savedAppPathStatus(install);

  return (
    <Card className="cloud-panel">
      <CardHeader className="cloud-panel-head">
        <div>
          <CardTitle>安装辅助</CardTitle>
          <CardDescription>只检查本机 Codex 可用性、保存路径、桌面入口和 Watcher 状态。</CardDescription>
        </div>
      </CardHeader>
      <CardContent>
        <div className="cloud-check-list">
          <InstallRow title="Codex 是否已安装" status={install.codexApp.status} detail={codexAppDetail(install)} />
          <InstallRow title="保存路径" status={savedPathStatus} detail={install.savedAppPath || "未保存；会使用本机自动探测结果。"} />
          <InstallRow title="桌面入口" status={install.managementShortcut.status} detail={install.managementShortcut.path || "未找到入口时可使用“修复入口”重新创建。"} />
          <InstallRow title="Watcher 状态" status={watcherStatus(install)} detail={watcherDetail(install)} />
          <InstallRow title={summary.title} status={summary.status} detail={summary.detail} />
          <InstallRow title="权限提示" status="info" detail={platformHelpText()} />
        </div>
        <div className="cloud-secondary-actions">
          <Button onClick={() => void onCheck()} variant="secondary">
            <RefreshCw className="h-4 w-4" />
            检查本机
          </Button>
          <Button onClick={() => void onChooseCodexAppPath("folder")} variant="outline">
            <FolderOpen className="h-4 w-4" />
            选择应用目录
          </Button>
          <Button onClick={() => void onChooseCodexAppPath("file")} variant="outline">
            选择 Codex.exe / .app
          </Button>
          <Button onClick={() => void onRepairShortcuts()} variant="secondary">
            <Wrench className="h-4 w-4" />
            修复入口
          </Button>
          <Button onClick={() => void onRepairBackend()} variant="secondary">
            <Wrench className="h-4 w-4" />
            修复后端
          </Button>
          <Button onClick={onOpenMaintenance} variant="ghost">
            更多维护
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function savedAppPathStatus(install: CloudInstallState) {
  if (!install.savedAppPath) return isOk(install.codexApp.status) ? "auto" : "not_checked";
  if (isNotChecked(install.codexApp.status) || isOk(install.codexApp.status)) return "saved";
  return "invalid_path";
}

function codexAppDetail(install: CloudInstallState) {
  if (install.codexApp.path) return install.codexApp.path;
  if (install.savedAppPath) return `未从保存路径识别到 Codex：${install.savedAppPath}`;
  if (isNotChecked(install.codexApp.status)) return "尚未检查 Codex 应用路径。";
  if (isMissing(install.codexApp.status)) return "未识别到本机 Codex；请先完成官方安装，或选择免安装/解包版目录。";
  return "当前路径不可用；请选择应用目录或 Codex.exe / .app 后重新检查。";
}

function watcherStatus(install: CloudInstallState) {
  if (install.watcherEnabled === null) return "not_checked";
  return install.watcherEnabled ? "ok" : "disabled";
}

function watcherDetail(install: CloudInstallState) {
  if (install.watcherDetail) return install.watcherDetail;
  if (install.watcherEnabled === true) return "已启用，会保持本地接管状态。";
  if (install.watcherEnabled === false) return "当前未启用；可进入更多维护处理 Watcher。";
  return "尚未检查 Watcher 状态。";
}

function platformHelpText() {
  const platform = typeof navigator === "undefined" ? "" : navigator.platform.toLowerCase();
  if (platform.includes("mac")) {
    return "macOS 如遇 Gatekeeper 或权限拦截，请在系统设置中允许 Codex / Codex++，再回到这里检查或修复入口。";
  }
  if (platform.includes("win")) {
    return "Windows 如遇入口修复失败，请确认桌面写入权限；防火墙提示只影响本地调试端口，允许后再检查。";
  }
  return "如遇权限、防火墙或系统拦截，请先在系统设置中允许 Codex / Codex++，再重新检查。";
}

function buildInstallSummary(install: CloudInstallState) {
  if (isNotChecked(install.codexApp.status)) {
    return {
      status: "not_checked",
      title: "下一步",
      detail: "先点击“检查本机”读取 Codex 应用、桌面入口和 Watcher 状态。",
    };
  }
  if (!isOk(install.codexApp.status)) {
    if (install.savedAppPath) {
      return {
        status: "invalid_path",
        title: "路径需要修复",
        detail: "保存路径当前不可用。请选择应用目录或 Codex.exe / .app，然后点击“检查本机”。",
      };
    }
    return {
      status: "missing",
      title: "未识别到 Codex",
      detail: "请先通过官方渠道安装 Codex；免安装或解包版可直接选择应用目录。本页不会自动下载或静默安装第三方软件。",
    };
  }
  if (isNotChecked(install.managementShortcut.status)) {
    return {
      status: "not_checked",
      title: "入口尚未检查",
      detail: "Codex 已识别；继续点击“检查本机”确认桌面入口是否存在。",
    };
  }
  if (!isOk(install.managementShortcut.status)) {
    return {
      status: "missing",
      title: "入口需要修复",
      detail: "Codex 已识别，但桌面入口缺失。点击“修复入口”后再检查本机。",
    };
  }
  if (install.watcherEnabled === null) {
    return {
      status: "not_checked",
      title: "Watcher 尚未检查",
      detail: "入口已就绪；点击“检查本机”同步 Watcher 状态。",
    };
  }
  if (install.watcherEnabled === false) {
    return {
      status: "disabled",
      title: "Watcher 未启用",
      detail: "Codex 和入口已就绪；如需要保持静默接管，请进入更多维护启用 Watcher。",
    };
  }
  return {
    status: "ok",
    title: "本地 Codex 已就绪",
    detail: "Codex 应用、保存路径、桌面入口和 Watcher 状态已完成检查。",
  };
}

function InstallRow({ title, status, detail }: { title: string; status: string | null | undefined; detail: string | null | undefined }) {
  const ok = isOk(status);
  return (
    <div className={`cloud-install-row ${ok ? "ok" : ""}`}>
      <span className="cloud-install-icon">{ok ? <CheckCircle2 className="h-4 w-4" /> : <XCircle className="h-4 w-4" />}</span>
      <div>
        <strong>{title}</strong>
        <span>{detail || "暂无路径"}</span>
      </div>
      <em>{localStatusLabel(status)}</em>
    </div>
  );
}
