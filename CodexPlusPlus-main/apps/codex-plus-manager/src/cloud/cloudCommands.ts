import { invoke } from "@tauri-apps/api/core";
import {
  mockApplyManagedProvider,
  mockCancelBrowserHandoff,
  mockConfigureEndpoint,
  mockLogin,
  mockLogin2FA,
  mockLogout,
  mockPollBrowserHandoff,
  mockReadDiagnostics,
  mockRedeem,
  mockRefreshBootstrap,
  mockRegisterDevice,
  mockRepairManagedProvider,
  mockRuntimeState,
  mockStartBrowserHandoff,
  mockUsage,
  nextFixtureState,
} from "./fixtures";
import type {
  CloudAnnouncement,
  CloudBrowserHandoffRequest,
  CloudDiagnostics,
  CloudEndpointRequest,
  CloudFeatureFlags,
  CloudLoginRequest,
  CloudModelSummary,
  CloudRedeemRequest,
  CloudRuntimeState,
  CloudServiceStatus,
  CloudTwoFactorRequest,
  CloudVersionPolicy,
} from "./types";

const commandNames = {
  loadState: "codexplus_cloud_load_state",
  configureEndpoint: "codexplus_cloud_configure_endpoint",
  login: "codexplus_cloud_login",
  login2fa: "codexplus_cloud_login_2fa",
  startBrowserHandoff: "codexplus_cloud_start_browser_handoff",
  pollBrowserHandoff: "codexplus_cloud_poll_browser_handoff",
  cancelBrowserHandoff: "codexplus_cloud_cancel_browser_handoff",
  logout: "codexplus_cloud_logout",
  refreshBootstrap: "codexplus_cloud_refresh_bootstrap",
  registerDevice: "codexplus_cloud_register_device",
  applyManagedProvider: "codexplus_cloud_apply_managed_provider",
  repairManagedProvider: "codexplus_cloud_repair_managed_provider",
  redeem: "codexplus_cloud_redeem",
  loadUsage: "codexplus_cloud_load_usage",
  readDiagnostics: "codexplus_cloud_read_redacted_diagnostics",
} as const;

type TauriCommandResult<T> = {
  status: string;
  message: string;
} & T;

type CoreRuntimeState = {
  connection?: {
    baseUrl?: string;
    authenticated?: boolean;
    userLabel?: string | null;
    tokenExpiresAt?: string | null;
    pendingTwoFactor?: boolean;
    pendingTwoFactorUserLabel?: string | null;
    pendingBrowserHandoff?: boolean;
    browserHandoffAuthorizeUrl?: string | null;
    browserHandoffVerificationCode?: string | null;
    browserHandoffExpiresAt?: string | null;
    browserHandoffPollIntervalSeconds?: number | null;
  };
  device?: {
    deviceId?: string | null;
    deviceName?: string | null;
    status?: string;
  };
  entitlement?: {
    status?: string;
    serviceStatus?: string;
    message?: string;
    messageKey?: string | null;
    actionHint?: string;
    actionType?: string | null;
    actionLabel?: string | null;
    actionCopyKey?: string | null;
    actionUrl?: string | null;
    retryable?: boolean;
    planName?: string | null;
    expiresAt?: string | null;
    renewalHint?: string | null;
  };
  usage?: {
    balanceDisplay?: string | null;
    periodUsageDisplay?: string | null;
    rateLimitState?: string | null;
    actionType?: string | null;
    actionLabel?: string | null;
    actionCopyKey?: string | null;
    actionUrl?: string | null;
  };
  provider?: {
    managedProviderId?: string;
    displayName?: string;
    configured?: boolean;
    active?: boolean;
    baseUrl?: string | null;
    defaultModel?: string | null;
    hasApiKey?: boolean;
  };
  diagnostics?: {
    lastErrorCode?: string | null;
    lastErrorMessage?: string | null;
    lastBootstrapAt?: string | null;
    configVersion?: string | null;
    snapshotVersion?: string | null;
  };
  models?: CloudModelSummary[];
  featureFlags?: CloudFeatureFlags | null;
  announcements?: CloudAnnouncement[];
  versionPolicy?: CloudVersionPolicy | null;
};

type CloudStateCommandPayload = {
  state: CoreRuntimeState;
};

type CloudDiagnosticsCommandPayload = {
  path: string;
  text: string;
  lines: number;
};

function shouldUseFixture(error: unknown): boolean {
  const text = error instanceof Error ? error.message : String(error);
  return /not implemented|not_implemented|unknown command|command not found|not found|ipc|__TAURI__|reading 'invoke'/i.test(text);
}

function hasTauriRuntime(): boolean {
  if (typeof window === "undefined") return false;
  const runtime = window as Window & {
    __TAURI__?: { core?: { invoke?: unknown }; invoke?: unknown };
    __TAURI_INTERNALS__?: { invoke?: unknown };
  };
  return Boolean(
    typeof runtime.__TAURI_INTERNALS__?.invoke === "function" ||
      typeof runtime.__TAURI__?.core?.invoke === "function" ||
      typeof runtime.__TAURI__?.invoke === "function",
  );
}

async function invokeOrFixture<TRaw, TOut>(
  command: string,
  args: Record<string, unknown> | undefined,
  runtime: (result: TauriCommandResult<TRaw>) => TOut,
  fixture: () => TOut,
): Promise<TOut> {
  if (!hasTauriRuntime()) return fixture();
  try {
    const result = await invoke<TauriCommandResult<TRaw>>(command, args);
    return runtime(result);
  } catch (error) {
    if (shouldUseFixture(error)) return fixture();
    throw error;
  }
}

function normalizeServiceStatus(value: string | undefined): CloudServiceStatus {
  if (value === "needs_login") return "not_authenticated";
  return (value || "unknown") as CloudServiceStatus;
}

function commandBootstrapStatus(commandStatus: string, serviceStatus: string, authenticated: boolean): "success" | "error" {
  if (commandStatus === "failed" || !authenticated || serviceStatus === "not_authenticated") return "error";
  return "success";
}

function versionPolicyFromCore(core: CoreRuntimeState): CloudVersionPolicy {
  return core.versionPolicy ?? {
    config_version: core.diagnostics?.configVersion || "unknown",
    snapshot_version: core.diagnostics?.snapshotVersion || "local",
    refresh_ttl_seconds: 300,
    force_refresh: false,
    minimum_client_version: "unknown",
  };
}

function featureFlagsFromCore(core: CoreRuntimeState): CloudFeatureFlags {
  return core.featureFlags ?? {
    advanced_provider_config: false,
    install_assistant: false,
    new_user_tutorial: false,
    model_selector: false,
    diagnostic_export: false,
    announcements: false,
    force_update_prompt: false,
    strict_device_enforcement: false,
  };
}

function modelsFromCore(core: CoreRuntimeState): CloudModelSummary[] {
  if (Array.isArray(core.models) && core.models.length > 0) return core.models;
  const defaultModel = core.provider?.defaultModel?.trim();
  if (!defaultModel) return [];
  return [
    {
      model_id: defaultModel,
      route_model: defaultModel,
      label: defaultModel,
      is_default: true,
      is_available: true,
      disabled_reason: null,
    },
  ];
}

function diagnosticsFromCore(core: CoreRuntimeState, status = "ok", message = "Diagnostics loaded."): CloudDiagnostics {
  const report = [
    `service_status=${normalizeServiceStatus(core.entitlement?.serviceStatus || core.entitlement?.status)}`,
    `error_code=${core.diagnostics?.lastErrorCode || "none"}`,
    `config_version=${core.diagnostics?.configVersion || "unknown"}`,
    `snapshot_version=${core.diagnostics?.snapshotVersion || "unknown"}`,
    "redaction_applied=true",
  ].join("\n");
  return {
    status,
    message,
    report,
    updatedAt: core.diagnostics?.lastBootstrapAt || new Date().toISOString(),
  };
}

function toCloudRuntimeState(result: TauriCommandResult<CloudStateCommandPayload>): CloudRuntimeState {
  const core = result.state ?? {};
  const serviceStatus = normalizeServiceStatus(core.entitlement?.serviceStatus || core.entitlement?.status);
  const authenticated = Boolean(core.connection?.authenticated);
  const versionPolicy = versionPolicyFromCore(core);
  const featureFlags = featureFlagsFromCore(core);
  const actionUrl = core.entitlement?.actionUrl || core.usage?.actionUrl || core.entitlement?.renewalHint || null;
  const actionLabel = core.entitlement?.actionLabel || core.usage?.actionLabel || "打开操作页面";
  const actionType = core.entitlement?.actionType || core.usage?.actionType || "open_url";
  const actionCopyKey = core.entitlement?.actionCopyKey || core.usage?.actionCopyKey || null;
  const providerId = core.provider?.managedProviderId || "codex-plus-cloud";
  const providerName = core.provider?.displayName || "Codex++ Cloud";
  const defaultModel = core.provider?.defaultModel || "";
  const hasApiKey = Boolean(core.provider?.hasApiKey);

  return {
    connection: {
      baseUrl: core.connection?.baseUrl || "",
      authenticated,
      userLabel: core.connection?.userLabel ?? null,
      tokenExpiresAt: core.connection?.tokenExpiresAt ?? null,
      pendingTwoFactor: Boolean(core.connection?.pendingTwoFactor),
      pendingTwoFactorUserLabel: core.connection?.pendingTwoFactorUserLabel ?? null,
      pendingBrowserHandoff: Boolean(core.connection?.pendingBrowserHandoff),
      browserHandoffAuthorizeUrl: core.connection?.browserHandoffAuthorizeUrl ?? null,
      browserHandoffVerificationCode: core.connection?.browserHandoffVerificationCode ?? null,
      browserHandoffExpiresAt: core.connection?.browserHandoffExpiresAt ?? null,
      browserHandoffPollIntervalSeconds: core.connection?.browserHandoffPollIntervalSeconds ?? null,
    },
    bootstrap: {
      status: commandBootstrapStatus(result.status, serviceStatus, authenticated),
      message: result.message || core.entitlement?.message || "Codex++ Cloud state loaded.",
      error_code: core.diagnostics?.lastErrorCode || null,
      data: {
        service: {
          status: serviceStatus,
          message: core.entitlement?.message || result.message || "Codex++ Cloud state loaded.",
          message_key: core.entitlement?.messageKey || null,
          action_hint: core.entitlement?.actionHint || "none",
          retryable: core.entitlement?.retryable !== false,
          support_url: actionUrl,
          error_code: core.diagnostics?.lastErrorCode || null,
        },
        provider: {
          provider_id: providerId,
          display_name: providerName,
          gateway_base_url: core.provider?.baseUrl || "",
          auth_mode: "user_side_api_key",
          api_key: null,
          key_summary: {
            key_id: hasApiKey ? providerId : "",
            masked_key: hasApiKey ? "configured" : "",
            created_at: null,
            last_used_at: null,
          },
          default_model: defaultModel,
        },
        plan: {
          plan_id: "",
          name: core.entitlement?.planName || "",
          status: serviceStatus,
          expires_at: core.entitlement?.expiresAt || null,
          renew_url: actionUrl,
          commerce_action: actionUrl
            ? {
                action_type: actionType,
                message_key: core.entitlement?.messageKey || null,
                action_copy_key: actionCopyKey,
                label: actionLabel,
                url: actionUrl,
              }
            : null,
        },
        models: modelsFromCore(core),
        usage: {
          balance_display: core.usage?.balanceDisplay || "",
          low_balance: serviceStatus === "low_balance",
          period_usage_display: core.usage?.periodUsageDisplay || "",
          rate_limit_state: core.usage?.rateLimitState || "",
          renew_action: actionUrl
            ? {
                action_type: core.usage?.actionType || actionType,
                message_key: core.entitlement?.messageKey || null,
                action_copy_key: core.usage?.actionCopyKey || actionCopyKey,
                label: core.usage?.actionLabel || actionLabel,
                url: actionUrl,
              }
            : null,
        },
        feature_flags: featureFlags,
        announcements: core.announcements ?? [],
        version_policy: versionPolicy,
        device: {
          device_id: core.device?.deviceId || "",
          status: core.device?.status || "unknown",
          message: core.device?.status || "unknown",
          snapshot_version: versionPolicy.snapshot_version,
        },
      },
    },
    install: {
      codexApp: { status: "not_checked", path: null },
      savedAppPath: null,
      silentShortcut: { status: "not_checked", path: null },
      managementShortcut: { status: "not_checked", path: null },
      watcherEnabled: null,
      watcherDetail: null,
    },
    managedProvider: {
      configured: Boolean(core.provider?.configured),
      active: Boolean(core.provider?.active),
      hasApiKey,
      displayName: providerName,
      maskedKey: hasApiKey ? "configured" : "",
      defaultModel,
      lastAppliedAt: core.diagnostics?.lastBootstrapAt || null,
      errorMessage: core.diagnostics?.lastErrorMessage || null,
    },
    diagnostics: diagnosticsFromCore(core, result.status, result.message),
    cachedAt: core.diagnostics?.lastBootstrapAt || new Date().toISOString(),
    source: "tauri",
  };
}

function toCloudDiagnostics(result: TauriCommandResult<CloudDiagnosticsCommandPayload>): CloudDiagnostics {
  return {
    status: result.status,
    message: result.message,
    report: result.text || "",
    updatedAt: new Date().toISOString(),
  };
}

export const cloudCommands = {
  loadState: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.loadState,
      undefined,
      toCloudRuntimeState,
      mockRuntimeState,
    ),

  configureEndpoint: (request: CloudEndpointRequest) =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.configureEndpoint,
      { baseUrl: request.endpoint },
      toCloudRuntimeState,
      mockConfigureEndpoint,
    ),

  login: (request: CloudLoginRequest) =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.login,
      { baseUrl: request.endpoint, email: request.account, password: request.password },
      toCloudRuntimeState,
      mockLogin,
    ),

  login2FA: (request: CloudTwoFactorRequest) =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.login2fa,
      { totpCode: request.totpCode },
      toCloudRuntimeState,
      mockLogin2FA,
    ),

  startBrowserHandoff: (request: CloudBrowserHandoffRequest) =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.startBrowserHandoff,
      { baseUrl: request.endpoint },
      toCloudRuntimeState,
      mockStartBrowserHandoff,
    ),

  pollBrowserHandoff: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.pollBrowserHandoff,
      undefined,
      toCloudRuntimeState,
      mockPollBrowserHandoff,
    ),

  cancelBrowserHandoff: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.cancelBrowserHandoff,
      undefined,
      toCloudRuntimeState,
      mockCancelBrowserHandoff,
    ),

  logout: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.logout,
      undefined,
      toCloudRuntimeState,
      mockLogout,
    ),

  refreshBootstrap: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.refreshBootstrap,
      undefined,
      toCloudRuntimeState,
      mockRefreshBootstrap,
    ),

  registerDevice: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.registerDevice,
      { deviceName: undefined },
      toCloudRuntimeState,
      mockRegisterDevice,
    ),

  applyManagedProvider: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.applyManagedProvider,
      undefined,
      toCloudRuntimeState,
      mockApplyManagedProvider,
    ),

  repairManagedProvider: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.repairManagedProvider,
      undefined,
      toCloudRuntimeState,
      mockRepairManagedProvider,
    ),

  redeem: (request: CloudRedeemRequest) =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.redeem,
      { code: request.code },
      toCloudRuntimeState,
      mockRedeem,
    ),

  loadUsage: () =>
    invokeOrFixture<CloudStateCommandPayload, CloudRuntimeState>(
      commandNames.loadUsage,
      undefined,
      toCloudRuntimeState,
      mockUsage,
    ),

  readDiagnostics: () =>
    invokeOrFixture<CloudDiagnosticsCommandPayload, CloudDiagnostics>(
      commandNames.readDiagnostics,
      { lines: 200 },
      toCloudDiagnostics,
      mockReadDiagnostics,
    ),

  nextFixtureState: () => Promise.resolve(nextFixtureState()),
};
