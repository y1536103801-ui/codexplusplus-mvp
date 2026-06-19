import { useCallback, useEffect, useMemo, useState } from "react";
import { cloudCommands } from "./cloudCommands";
import type {
  CloudDiagnostics,
  CloudBrowserHandoffRequest,
  CloudEndpointRequest,
  CloudLoginRequest,
  CloudRedeemRequest,
  CloudRuntimeState,
  CloudTwoFactorRequest,
} from "./types";

export type CloudActionName =
  | "load"
  | "configureEndpoint"
  | "login"
  | "login2fa"
  | "startBrowserHandoff"
  | "pollBrowserHandoff"
  | "cancelBrowserHandoff"
  | "logout"
  | "refresh"
  | "registerDevice"
  | "applyProvider"
  | "repairProvider"
  | "redeem"
  | "usage"
  | "diagnostics"
  | "fixture";

type NoticeStatus = "ok" | "failed" | string;

type UseCloudRuntimeOptions = {
  onNotice?: (title: string, message: string, status?: NoticeStatus) => void;
};

function cloudErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function useCloudRuntime(options: UseCloudRuntimeOptions = {}) {
  const { onNotice } = options;
  const [state, setState] = useState<CloudRuntimeState | null>(null);
  const [pending, setPending] = useState<CloudActionName | null>(null);
  const [lastError, setLastError] = useState<string | null>(null);

  const runState = useCallback(
    async (action: CloudActionName, title: string, task: () => Promise<CloudRuntimeState>, silentSuccess = false) => {
      setPending(action);
      setLastError(null);
      try {
        const result = await task();
        setState(result);
        const status = result.bootstrap.status === "error" ? "failed" : "ok";
        if (!silentSuccess) {
          onNotice?.(title, result.bootstrap.message || "Codex++ Cloud 状态已更新。", status);
        }
        return result;
      } catch (error) {
        const message = cloudErrorMessage(error);
        setLastError(message);
        onNotice?.(title, message, "failed");
        return null;
      } finally {
        setPending(null);
      }
    },
    [onNotice],
  );

  const load = useCallback(
    (silent = true) => runState("load", "Codex++ Cloud", cloudCommands.loadState, silent),
    [runState],
  );

  const configureEndpoint = useCallback(
    (request: CloudEndpointRequest) =>
      runState("configureEndpoint", "服务地址", () => cloudCommands.configureEndpoint(request)),
    [runState],
  );

  const login = useCallback(
    (request: CloudLoginRequest) => runState("login", "登录 Codex++ Cloud", () => cloudCommands.login(request)),
    [runState],
  );

  const login2FA = useCallback(
    (request: CloudTwoFactorRequest) =>
      runState("login2fa", "完成双重验证", () => cloudCommands.login2FA(request)),
    [runState],
  );

  const startBrowserHandoff = useCallback(
    (request: CloudBrowserHandoffRequest) =>
      runState("startBrowserHandoff", "浏览器登录", () => cloudCommands.startBrowserHandoff(request)),
    [runState],
  );

  const pollBrowserHandoff = useCallback(
    () => runState("pollBrowserHandoff", "检查浏览器登录", cloudCommands.pollBrowserHandoff),
    [runState],
  );

  const cancelBrowserHandoff = useCallback(
    () => runState("cancelBrowserHandoff", "取消浏览器登录", cloudCommands.cancelBrowserHandoff),
    [runState],
  );

  const logout = useCallback(
    () => runState("logout", "退出登录", cloudCommands.logout),
    [runState],
  );

  const refresh = useCallback(
    (silent = false) => runState("refresh", "刷新云服务", cloudCommands.refreshBootstrap, silent),
    [runState],
  );

  const registerDevice = useCallback(
    () => runState("registerDevice", "注册设备", cloudCommands.registerDevice),
    [runState],
  );

  const applyProvider = useCallback(
    () => runState("applyProvider", "配置 Codex++ Cloud", cloudCommands.applyManagedProvider),
    [runState],
  );

  const repairProvider = useCallback(
    () => runState("repairProvider", "修复 Codex++ Cloud", cloudCommands.repairManagedProvider),
    [runState],
  );

  const redeem = useCallback(
    (request: CloudRedeemRequest) => runState("redeem", "兑换激活码", () => cloudCommands.redeem(request)),
    [runState],
  );

  const refreshUsage = useCallback(
    () => runState("usage", "刷新用量", cloudCommands.loadUsage),
    [runState],
  );

  const readDiagnostics = useCallback(async () => {
    setPending("diagnostics");
    setLastError(null);
    try {
      const diagnostics: CloudDiagnostics = await cloudCommands.readDiagnostics();
      setState((current) => (current ? { ...current, diagnostics } : current));
      onNotice?.("诊断", diagnostics.message, diagnostics.status);
      return diagnostics;
    } catch (error) {
      const message = cloudErrorMessage(error);
      setLastError(message);
      onNotice?.("诊断", message, "failed");
      return null;
    } finally {
      setPending(null);
    }
  }, [onNotice]);

  const showNextFixtureState = useCallback(
    () => runState("fixture", "Mock 状态", cloudCommands.nextFixtureState),
    [runState],
  );

  useEffect(() => {
    void load(true);
  }, [load]);

  return useMemo(
    () => ({
      state,
      pending,
      lastError,
      isBusy: pending !== null,
      load,
      configureEndpoint,
      login,
      login2FA,
      startBrowserHandoff,
      pollBrowserHandoff,
      cancelBrowserHandoff,
      logout,
      refresh,
      registerDevice,
      applyProvider,
      repairProvider,
      redeem,
      refreshUsage,
      readDiagnostics,
      showNextFixtureState,
    }),
    [
      state,
      pending,
      lastError,
      load,
      configureEndpoint,
      login,
      login2FA,
      startBrowserHandoff,
      pollBrowserHandoff,
      cancelBrowserHandoff,
      logout,
      refresh,
      registerDevice,
      applyProvider,
      repairProvider,
      redeem,
      refreshUsage,
      readDiagnostics,
      showNextFixtureState,
    ],
  );
}
