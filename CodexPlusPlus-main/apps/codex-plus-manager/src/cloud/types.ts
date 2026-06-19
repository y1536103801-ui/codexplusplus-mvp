export type CloudCommandStatus = "success" | "error" | "ok" | "failed" | "not_implemented" | "not_checked" | string;

export type CloudServiceStatus =
  | "available"
  | "not_authenticated"
  | "not_purchased"
  | "expired"
  | "low_balance"
  | "disabled"
  | "device_revoked"
  | "model_unavailable"
  | "rate_limited"
  | "gateway_unhealthy"
  | "local_codex_missing"
  | "local_config_failed"
  | "stale_snapshot"
  | string;

export type CloudApiResult<T> = {
  status: CloudCommandStatus;
  message: string;
  error_code: string | null;
  data: T | null;
};

export type CloudService = {
  status: CloudServiceStatus;
  message: string;
  message_key?: string | null;
  action_hint: string;
  retryable: boolean;
  support_url: string | null;
  error_code: string | null;
};

export type CloudProvider = {
  provider_id: string;
  display_name: string;
  gateway_base_url: string;
  auth_mode: string;
  api_key: string | null;
  key_summary: {
    key_id: string;
    masked_key: string;
    created_at: string | null;
    last_used_at: string | null;
  };
  default_model: string;
};

export type CloudPlan = {
  plan_id: string;
  name: string;
  status: string;
  expires_at: string | null;
  renew_url: string | null;
  commerce_action?: CloudClientAction | null;
};

export type CloudModelSummary = {
  model_id: string;
  route_model: string;
  label: string;
  is_default: boolean;
  is_available: boolean;
  disabled_reason: string | null;
};

export type CloudClientAction = {
  action_type?: string;
  message_key?: string | null;
  action_copy_key?: string | null;
  label: string;
  url: string | null;
};

export type CloudUsage = {
  balance_display: string;
  low_balance: boolean;
  period_usage_display: string;
  rate_limit_state: string;
  renew_action: CloudClientAction | null;
};

export type CloudFeatureFlags = {
  advanced_provider_config: boolean;
  install_assistant: boolean;
  new_user_tutorial: boolean;
  model_selector: boolean;
  diagnostic_export: boolean;
  announcements?: boolean;
  force_update_prompt?: boolean;
  strict_device_enforcement?: boolean;
};

export type CloudAnnouncement = {
  id: string;
  severity: "info" | "warning" | "error" | string;
  message: string;
  url: string | null;
};

export type CloudVersionPolicy = {
  config_version: string;
  snapshot_version: string;
  refresh_ttl_seconds: number;
  force_refresh: boolean;
  minimum_client_version: string;
};

export type CloudDevice = {
  device_id: string;
  status: string;
  message: string;
  snapshot_version: string;
};

export type CloudBootstrapData = {
  service: CloudService;
  provider: CloudProvider;
  plan: CloudPlan;
  models: CloudModelSummary[];
  usage: CloudUsage;
  feature_flags: CloudFeatureFlags;
  announcements: CloudAnnouncement[];
  version_policy: CloudVersionPolicy;
  device: CloudDevice;
};

export type CloudBootstrapResult = CloudApiResult<CloudBootstrapData>;

export type CloudInstallItem = {
  status: string;
  path: string | null;
};

export type CloudInstallState = {
  codexApp: CloudInstallItem;
  savedAppPath: string | null;
  silentShortcut: CloudInstallItem;
  managementShortcut: CloudInstallItem;
  watcherEnabled: boolean | null;
  watcherDetail: string | null;
};

export type CloudManagedProviderState = {
  configured: boolean;
  active: boolean;
  hasApiKey: boolean;
  displayName: string;
  maskedKey: string;
  defaultModel: string;
  lastAppliedAt: string | null;
  errorMessage: string | null;
};

export type CloudDiagnostics = {
  status: CloudCommandStatus;
  message: string;
  report: string;
  updatedAt: string;
};

export type CloudRuntimeState = {
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
  bootstrap: CloudBootstrapResult;
  install: CloudInstallState;
  managedProvider: CloudManagedProviderState;
  diagnostics: CloudDiagnostics;
  cachedAt: string | null;
  source: "tauri" | "fixture";
};

export type CloudEndpointRequest = {
  endpoint: string;
};

export type CloudLoginRequest = {
  endpoint: string;
  account: string;
  password: string;
};

export type CloudBrowserHandoffRequest = {
  endpoint: string;
};

export type CloudTwoFactorRequest = {
  totpCode: string;
};

export type CloudRedeemRequest = {
  code: string;
};
