import { apiClient } from '../client'

export interface CodexPlusConfig {
  config_version: string
  draft_status?: string
  publish_scope: 'draft' | 'internal' | 'canary' | 'production'
  updated_by?: string
  updated_at?: string
  change_reason?: string
  rollback_from?: string | null
  plan_catalog: CodexPlusPlanCatalog
  model_catalog: CodexPlusModelCatalog
  usage_policy: CodexPlusUsagePolicy
  feature_flags: CodexPlusFeatureFlagsDoc
}

export interface CodexPlusPlanCatalog {
  config_version: string
  draft_status?: string
  publish_scope?: string
  rollback_from?: string | null
  updated_by?: string
  updated_at?: string
  change_reason?: string
  plans: CodexPlusPlan[]
}

export interface CodexPlusPlan {
  plan_id: string
  name: string
  description?: string
  billing_period: 'none' | 'trial' | 'monthly' | 'quarterly' | 'yearly' | 'one_time'
  currency: string
  price_amount_minor: number
  display_price: string
  entitlement_grant: {
    balance_credit: number
    duration_days: number
    daily_quota?: number | null
    period_quota?: number | null
  }
  entitlement_sources?: CodexPlusEntitlementSources
  model_groups: string[]
  usage_policy_id?: string
  purchase_url?: string | null
  renew_url: string | null
  copy_keys?: CodexPlusPlanCopyKeys
  is_listed: boolean
  status: 'active' | 'hidden' | 'disabled' | string
  sort_order?: number
  external_billing_refs?: {
    product_id?: string | null
    sku_id?: string | null
  }
}

export interface CodexPlusEntitlementSources {
  subscription_group_ids?: number[]
  api_key_group_ids?: number[]
  group_names?: string[]
}

export interface CodexPlusPlanCopyKeys {
  purchase_action?: string
  renew_action?: string
  upgrade_action?: string
  not_purchased_message?: string
  expired_message?: string
  low_balance_message?: string
}

export interface CodexPlusModelCatalog {
  config_version: string
  draft_status?: string
  publish_scope?: string
  rollback_from?: string | null
  updated_by?: string
  updated_at?: string
  change_reason?: string
  models: CodexPlusModel[]
}

export interface CodexPlusModel {
  model_id: string
  display_name: string
  route_model: string
  model_group: string
  context_window: number
  billing_multiplier: number
  is_default: boolean
  is_enabled: boolean
  is_hidden: boolean
  disabled_reason?: string | null
  rollout_channel?: string
  quality_tier?: string
  fallback_model_id?: string | null
  deprecation_at?: string | null
  disabled_replacement_model_id?: string | null
  disabled_message_key?: string | null
  sort_order?: number
  operator_tags?: string[]
}

export interface CodexPlusUsagePolicy {
  config_version: string
  draft_status?: string
  publish_scope?: string
  rollback_from?: string | null
  updated_by?: string
  updated_at?: string
  change_reason?: string
  policies: CodexPlusUsageRule[]
}

export interface CodexPlusUsageRule {
  policy_id: string
  applies_to?: {
    plan_ids?: string[]
    model_groups?: string[]
    user_segments?: string[]
  }
  low_balance_threshold: number
  daily_quota: number
  monthly_quota?: number | null
  concurrency_limit: number
  rpm_limit: number
  tpm_limit: number
  burst_limit?: number
  rate_limit_window_seconds?: number
  expired_behavior: 'block' | 'degrade' | 'allow_grace_period'
  grace_period_hours: number
  overage_behavior?: string
  copy_keys?: CodexPlusUsagePolicyCopyKeys
  device_policy?: CodexPlusUsagePolicyDevicePolicy
  insufficient_balance_message: string
  rate_limited_message?: string
}

export interface CodexPlusUsagePolicyCopyKeys {
  low_balance_message?: string
  insufficient_balance_message?: string
  rate_limited_message?: string
  expired_message?: string
  renew_action?: string
  purchase_action?: string
  device_revoked_message?: string
}

export interface CodexPlusUsagePolicyDevicePolicy {
  registration_required?: boolean
  max_devices_per_user?: number
  allow_self_service_replacement?: boolean
  replacement_cooldown_hours?: number
  strict_enforcement_default?: boolean
  revoke_reason_taxonomy?: string[]
  support_unlock_policy?: string
  revoked_behavior?: string
  message_keys?: {
    limit_reached?: string
    replacement_cooldown?: string
    revoked?: string
    support_unlock_required?: string
  }
}

export interface CodexPlusFeatureFlagsDoc {
  config_version: string
  draft_status?: string
  publish_scope?: string
  rollback_from?: string | null
  updated_by?: string
  updated_at?: string
  change_reason?: string
  flags: CodexPlusFeatureFlags & Record<string, boolean>
  exposure?: {
    client_visible?: string[]
    server_only?: string[]
  }
  copy_keys?: {
    force_update_prompt?: string
    install_assistant_entry?: string
    new_user_tutorial_entry?: string
    diagnostic_export_entry?: string
    announcement_entry?: string
  }
}

export interface CodexPlusFeatureFlags {
  advanced_provider_config: boolean
  install_assistant: boolean
  new_user_tutorial: boolean
  model_selector: boolean
  diagnostic_export: boolean
  announcements: boolean
  force_update_prompt: boolean
  strict_device_enforcement: boolean
}

export interface CodexPlusVersionEntry {
  config_version: string
  publish_scope: string
  updated_by: string
  updated_at: string
  change_reason: string
  rollback_from?: string | null
  config?: CodexPlusConfig
}

export interface CodexPlusOptions {
  groups: Array<{
    id: number
    name: string
    platform: string
    status: string
    subscription_type: string
    supported_model_scopes?: string[]
  }>
  payment_plans: Array<{
    id: number
    name: string
    group_id: number
    price: number
    validity_days: number
    validity_unit: string
    for_sale: boolean
    product_name: string
  }>
  models: Array<{
    model_id: string
    group_id?: number
    group_name?: string
    platform?: string
  }>
  feature_flags: string[]
  policy_presets: Array<{ id: string; name: string; description: string }>
}

export interface CodexPlusUserEntitlement {
  user?: {
    id: number
    email: string
    username: string
    status: string
    role: string
    balance: number
    concurrency: number
    rpm_limit: number
    allowed_group_ids: number[]
  }
  allowed_groups: CodexPlusOptions['groups']
  active_subscriptions: CodexPlusSubscriptionSummary[]
  subscriptions: CodexPlusSubscriptionSummary[]
  api_keys: Array<{
    id: number
    name: string
    group_id?: number
    status: string
    masked_key: string
    last_used_at?: string
  }>
  managed_provider_key: {
    exists: boolean
    masked_key?: string
    key_id?: number
  }
  usage_summary?: unknown
  devices: Array<{ device_id: string; status: string; last_seen?: string }>
  recent_events: Array<{ event_type: string; created_at: string; summary: string }>
  integration_status: Record<string, string>
}

export interface CodexPlusSubscriptionSummary {
  id: number
  user_id: number
  group_id: number
  group_name?: string
  group_platform?: string
  status: string
  starts_at: string
  expires_at: string
  daily_usage_usd: number
  weekly_usage_usd: number
  monthly_usage_usd: number
  notes?: string
}

export async function getCodexPlusConfig(): Promise<CodexPlusConfig> {
  const { data } = await apiClient.get<CodexPlusConfig>('/admin/codex-plus/config')
  return data
}

export async function validateCodexPlusConfig(config: CodexPlusConfig): Promise<{ valid: boolean }> {
  const { data } = await apiClient.post<{ valid: boolean }>('/admin/codex-plus/config/validate', config)
  return data
}

export async function publishCodexPlusConfig(
  config: CodexPlusConfig,
  changeReason: string
): Promise<CodexPlusConfig> {
  const { data } = await apiClient.post<CodexPlusConfig>('/admin/codex-plus/config/publish', {
    config,
    change_reason: changeReason
  })
  return data
}

export async function listCodexPlusConfigVersions(): Promise<CodexPlusVersionEntry[]> {
  const { data } = await apiClient.get<CodexPlusVersionEntry[]>('/admin/codex-plus/config/versions')
  return data
}

export async function rollbackCodexPlusConfig(
  configVersion: string,
  changeReason?: string
): Promise<CodexPlusConfig> {
  const { data } = await apiClient.post<CodexPlusConfig>('/admin/codex-plus/config/rollback', {
    config_version: configVersion,
    change_reason: changeReason
  })
  return data
}

export async function getCodexPlusOptions(): Promise<CodexPlusOptions> {
  const { data } = await apiClient.get<CodexPlusOptions>('/admin/codex-plus/options')
  return data
}

export async function getCodexPlusUserEntitlement(userId: number): Promise<CodexPlusUserEntitlement> {
  const { data } = await apiClient.get<CodexPlusUserEntitlement>(
    `/admin/codex-plus/users/${userId}/entitlement`
  )
  return data
}

export const codexPlusAdminAPI = {
  getConfig: getCodexPlusConfig,
  validateConfig: validateCodexPlusConfig,
  publishConfig: publishCodexPlusConfig,
  listVersions: listCodexPlusConfigVersions,
  rollbackConfig: rollbackCodexPlusConfig,
  getOptions: getCodexPlusOptions,
  getUserEntitlement: getCodexPlusUserEntitlement
}

export default codexPlusAdminAPI
