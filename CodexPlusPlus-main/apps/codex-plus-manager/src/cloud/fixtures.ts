import availableFixture from "../../../../../codex-plus-contracts/test-fixtures/client/bootstrap.available.json";
import deviceRevokedFixture from "../../../../../codex-plus-contracts/test-fixtures/client/bootstrap.device_revoked.json";
import expiredFixture from "../../../../../codex-plus-contracts/test-fixtures/client/bootstrap.expired.json";
import gatewayUnhealthyFixture from "../../../../../codex-plus-contracts/test-fixtures/client/bootstrap.gateway_unhealthy.json";
import lowBalanceFixture from "../../../../../codex-plus-contracts/test-fixtures/client/bootstrap.low_balance.json";
import modelUnavailableFixture from "../../../../../codex-plus-contracts/test-fixtures/client/bootstrap.model_unavailable.json";
import notAuthenticatedFixture from "../../../../../codex-plus-contracts/test-fixtures/client/bootstrap.not_authenticated.json";
import notPurchasedFixture from "../../../../../codex-plus-contracts/test-fixtures/client/bootstrap.not_purchased.json";
import type {
  CloudBootstrapResult,
  CloudDiagnostics,
  CloudInstallState,
  CloudManagedProviderState,
  CloudRuntimeState,
} from "./types";

type FixtureKey =
  | "available"
  | "not_authenticated"
  | "not_purchased"
  | "expired"
  | "low_balance"
  | "device_revoked"
  | "gateway_unhealthy"
  | "model_unavailable";

const bootstrapFixtures: Record<FixtureKey, CloudBootstrapResult> = {
  available: availableFixture as CloudBootstrapResult,
  not_authenticated: notAuthenticatedFixture as CloudBootstrapResult,
  not_purchased: notPurchasedFixture as CloudBootstrapResult,
  expired: expiredFixture as CloudBootstrapResult,
  low_balance: lowBalanceFixture as CloudBootstrapResult,
  device_revoked: deviceRevokedFixture as CloudBootstrapResult,
  gateway_unhealthy: gatewayUnhealthyFixture as CloudBootstrapResult,
  model_unavailable: modelUnavailableFixture as CloudBootstrapResult,
};

const fixtureOrder: FixtureKey[] = [
  "available",
  "not_authenticated",
  "not_purchased",
  "expired",
  "low_balance",
  "device_revoked",
  "gateway_unhealthy",
  "model_unavailable",
];

let activeFixture: FixtureKey = "not_authenticated";
let providerApplied = false;
let localConfigFailed = false;
let browserHandoffPending = false;

const mockInstall: CloudInstallState = {
  codexApp: { status: "not_checked", path: null },
  savedAppPath: null,
  silentShortcut: { status: "not_checked", path: null },
  managementShortcut: { status: "not_checked", path: null },
  watcherEnabled: null,
  watcherDetail: null,
};

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function activeBootstrap(): CloudBootstrapResult {
  const result = clone(bootstrapFixtures[activeFixture]);
  if (localConfigFailed && result.data) {
    result.message = "本机 Codex 配置写入失败。";
    result.data.service = {
      status: "local_config_failed",
      message: "账号已就绪，但本机 Codex 配置未能更新。",
      action_hint: "repair_local_config",
      retryable: true,
      support_url: null,
      error_code: "CLIENT_LOCAL_CONFIG_WRITE_FAILED",
    };
  }
  return result;
}

function managedProviderFromBootstrap(bootstrap: CloudBootstrapResult): CloudManagedProviderState {
  const provider = bootstrap.data?.provider;
  const serviceStatus = bootstrap.data?.service.status;
  const hasApiKey = Boolean(provider?.api_key || provider?.key_summary.masked_key);
  const readyStatus = serviceStatus === "available" || serviceStatus === "low_balance" || serviceStatus === "model_unavailable";
  return {
    configured: providerApplied && readyStatus,
    active: providerApplied && readyStatus,
    hasApiKey,
    displayName: provider?.display_name || "Codex++ 服务",
    maskedKey: provider?.key_summary.masked_key || "",
    defaultModel: provider?.default_model || "",
    lastAppliedAt: providerApplied ? new Date().toISOString() : null,
    errorMessage: localConfigFailed ? "本地写入失败，可尝试修复配置。" : null,
  };
}

function mockDiagnostics(): CloudDiagnostics {
  const bootstrap = activeBootstrap();
  const version = bootstrap.data?.version_policy;
  const service = bootstrap.data?.service;
  return {
    status: "ok",
    message: "Redacted diagnostics are ready.",
    updatedAt: new Date().toISOString(),
    report: [
      `service_status=${service?.status ?? "not_authenticated"}`,
      `error_code=${service?.error_code ?? bootstrap.error_code ?? "none"}`,
      `config_version=${version?.config_version ?? "unknown"}`,
      `snapshot_version=${version?.snapshot_version ?? "unknown"}`,
      "redaction_applied=true",
      "api_key=<redacted>",
      "authorization=<redacted>",
    ].join("\n"),
  };
}

export function nextFixtureState(): CloudRuntimeState {
  const currentIndex = fixtureOrder.indexOf(activeFixture);
  activeFixture = fixtureOrder[(currentIndex + 1) % fixtureOrder.length];
  browserHandoffPending = false;
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockRuntimeState(): CloudRuntimeState {
  const bootstrap = activeBootstrap();
  return {
    connection: {
      baseUrl: "mock://codex-plus-cloud",
      authenticated: activeFixture !== "not_authenticated",
      userLabel: activeFixture === "not_authenticated" ? null : "mock-user@example.com",
      tokenExpiresAt: null,
      pendingTwoFactor: false,
      pendingTwoFactorUserLabel: null,
      pendingBrowserHandoff: browserHandoffPending,
      browserHandoffAuthorizeUrl: browserHandoffPending
        ? "https://codex-plus.example/auth/desktop/authorize?session_token=mock-session-token&verification_code=123456"
        : null,
      browserHandoffVerificationCode: browserHandoffPending ? "123456" : null,
      browserHandoffExpiresAt: browserHandoffPending ? new Date(Date.now() + 15 * 60 * 1000).toISOString() : null,
      browserHandoffPollIntervalSeconds: browserHandoffPending ? 2 : null,
    },
    bootstrap,
    install: clone(mockInstall),
    managedProvider: managedProviderFromBootstrap(bootstrap),
    diagnostics: mockDiagnostics(),
    cachedAt: new Date().toISOString(),
    source: "fixture",
  };
}

export function mockConfigureEndpoint(): CloudRuntimeState {
  activeFixture = activeFixture === "not_authenticated" ? "not_authenticated" : activeFixture;
  return mockRuntimeState();
}

export function mockLogin(): CloudRuntimeState {
  activeFixture = "available";
  browserHandoffPending = false;
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockLogin2FA(): CloudRuntimeState {
  activeFixture = "available";
  browserHandoffPending = false;
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockStartBrowserHandoff(): CloudRuntimeState {
  activeFixture = "not_authenticated";
  browserHandoffPending = true;
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockPollBrowserHandoff(): CloudRuntimeState {
  activeFixture = "available";
  browserHandoffPending = false;
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockCancelBrowserHandoff(): CloudRuntimeState {
  browserHandoffPending = false;
  return mockRuntimeState();
}

export function mockLogout(): CloudRuntimeState {
  activeFixture = "not_authenticated";
  browserHandoffPending = false;
  providerApplied = false;
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockRefreshBootstrap(): CloudRuntimeState {
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockRegisterDevice(): CloudRuntimeState {
  if (activeFixture === "device_revoked") return mockRuntimeState();
  return mockRuntimeState();
}

export function mockApplyManagedProvider(): CloudRuntimeState {
  const status = activeBootstrap().data?.service.status;
  if (status === "available" || status === "low_balance" || status === "model_unavailable") {
    providerApplied = true;
    localConfigFailed = false;
  }
  return mockRuntimeState();
}

export function mockRepairManagedProvider(): CloudRuntimeState {
  providerApplied = true;
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockRedeem(): CloudRuntimeState {
  activeFixture = "available";
  localConfigFailed = false;
  return mockRuntimeState();
}

export function mockUsage(): CloudRuntimeState {
  return mockRuntimeState();
}

export function mockReadDiagnostics(): CloudDiagnostics {
  return mockDiagnostics();
}
