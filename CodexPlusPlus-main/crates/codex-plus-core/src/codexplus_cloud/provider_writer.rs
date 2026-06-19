use anyhow::Context;
use serde::Serialize;

use super::api::{BootstrapSnapshot, ManagedProvider};
use super::redaction::append_redacted_diagnostic;
use super::status::service_status_is_applyable;
use crate::settings::{BackendSettings, RelayMode, RelayProfile, RelayProtocol, SettingsStore};

pub const MANAGED_PROVIDER_ID: &str = "codex-plus-cloud";
pub const MANAGED_PROVIDER_NAME: &str = "Codex++ Cloud";

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ManagedProviderApplyResult {
    pub configured: bool,
    pub active: bool,
    pub backup_path: Option<String>,
    pub settings_path: String,
}

pub fn upsert_managed_provider(
    mut settings: BackendSettings,
    snapshot: &BootstrapSnapshot,
) -> anyhow::Result<BackendSettings> {
    validate_bootstrap_provider(snapshot)?;
    let profile = relay_profile_from_bootstrap(snapshot)?;
    let mut replaced = false;
    for existing in &mut settings.relay_profiles {
        if existing.id == MANAGED_PROVIDER_ID {
            *existing = profile.clone();
            replaced = true;
            break;
        }
    }
    if !replaced {
        settings.relay_profiles.push(profile);
    }
    settings.active_relay_id = MANAGED_PROVIDER_ID.to_string();
    settings.relay_profiles_enabled = true;
    Ok(settings)
}

pub fn apply_managed_provider_from_bootstrap(
    snapshot: &BootstrapSnapshot,
) -> anyhow::Result<ManagedProviderApplyResult> {
    validate_bootstrap_provider(snapshot)?;
    let store = SettingsStore::default();
    let current_settings = store.load().unwrap_or_default();
    let previous_active = current_settings.active_relay_id.clone();
    let next_settings = upsert_managed_provider(current_settings, snapshot)?;
    let home = crate::relay_config::default_codex_home_dir();
    append_redacted_diagnostic(
        "codexplus_cloud.provider_apply.start",
        serde_json::json!({
            "managed_provider_id": MANAGED_PROVIDER_ID,
            "service_status": snapshot.service.status,
            "config_version": snapshot.version_policy.config_version,
            "snapshot_version": snapshot.version_policy.snapshot_version,
            "gateway_base_url": snapshot.provider.gateway_base_url,
            "has_api_key": snapshot.provider.api_key.as_ref().is_some_and(|key| !key.trim().is_empty()),
        }),
    )
    .ok();

    let result = crate::relay_switch::switch_relay_profile_in_home(
        &store,
        &home,
        next_settings,
        &previous_active,
    )
    .with_context(|| "Codex++ Cloud provider write failed")?;
    append_redacted_diagnostic(
        "codexplus_cloud.provider_apply.ok",
        serde_json::json!({
            "managed_provider_id": MANAGED_PROVIDER_ID,
            "configured": result.configured,
            "backup_path": result.backup_path,
        }),
    )
    .ok();

    Ok(ManagedProviderApplyResult {
        configured: result.configured,
        active: result.settings.active_relay_id == MANAGED_PROVIDER_ID,
        backup_path: result.backup_path,
        settings_path: crate::paths::default_settings_path()
            .to_string_lossy()
            .to_string(),
    })
}

pub fn relay_profile_from_bootstrap(snapshot: &BootstrapSnapshot) -> anyhow::Result<RelayProfile> {
    validate_bootstrap_provider(snapshot)?;
    let provider = &snapshot.provider;
    let api_key = provider.api_key.as_deref().unwrap_or_default().trim();
    let base_url = provider.gateway_base_url.trim().trim_end_matches('/');
    let model = provider.default_model.trim();
    let config_contents = cloud_config_contents(base_url, model);
    let auth_contents = serde_json::to_string_pretty(&serde_json::json!({
        "OPENAI_API_KEY": api_key
    }))?;
    Ok(RelayProfile {
        id: MANAGED_PROVIDER_ID.to_string(),
        name: MANAGED_PROVIDER_NAME.to_string(),
        model: model.to_string(),
        base_url: base_url.to_string(),
        upstream_base_url: base_url.to_string(),
        api_key: api_key.to_string(),
        protocol: RelayProtocol::Responses,
        relay_mode: RelayMode::PureApi,
        official_mix_api_key: false,
        test_model: model.to_string(),
        config_contents,
        auth_contents,
        use_common_config: true,
        ..RelayProfile::default()
    })
}

fn validate_bootstrap_provider(snapshot: &BootstrapSnapshot) -> anyhow::Result<()> {
    if snapshot.provider.provider_id != MANAGED_PROVIDER_ID {
        anyhow::bail!(
            "unexpected managed provider id: {}",
            snapshot.provider.provider_id
        );
    }
    if !service_status_is_applyable(&snapshot.service.status) {
        anyhow::bail!(
            "Codex++ Cloud provider is not applyable while service status is {}",
            snapshot.service.status
        );
    }
    validate_provider_material(&snapshot.provider)
}

fn validate_provider_material(provider: &ManagedProvider) -> anyhow::Result<()> {
    if provider.display_name != MANAGED_PROVIDER_NAME {
        anyhow::bail!("unexpected managed provider display name");
    }
    if provider.gateway_base_url.trim().is_empty() {
        anyhow::bail!("Codex++ Cloud gateway base URL is missing");
    }
    if provider.default_model.trim().is_empty() {
        anyhow::bail!("Codex++ Cloud default model is missing");
    }
    if provider
        .api_key
        .as_deref()
        .map(str::trim)
        .filter(|key| !key.is_empty())
        .is_none()
    {
        anyhow::bail!("Codex++ Cloud gateway key is missing");
    }
    Ok(())
}

fn cloud_config_contents(_base_url: &str, model: &str) -> String {
    let codex_base_url = crate::protocol_proxy::local_responses_proxy_base_url(
        crate::protocol_proxy::DEFAULT_PROTOCOL_PROXY_PORT,
    );
    format!(
        "model = \"{}\"\nmodel_provider = \"custom\"\n\n[model_providers.custom]\nname = \"custom\"\nwire_api = \"responses\"\nrequires_openai_auth = true\nbase_url = \"{}\"\n",
        toml_escape(model),
        toml_escape(&codex_base_url)
    )
}

fn toml_escape(value: &str) -> String {
    value.replace('\\', "\\\\").replace('"', "\\\"")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::codexplus_cloud::api::{
        Announcement, DeviceRegistration, FeatureFlagSnapshot, ManagedProvider, PlanSummary,
        ProviderKeySummary, RenewAction, ServiceState, UsageSummary, VersionPolicy,
    };

    #[test]
    fn upsert_managed_provider_preserves_manual_profiles() {
        let settings = BackendSettings {
            relay_profiles: vec![RelayProfile {
                id: "manual-a".to_string(),
                name: "Manual A".to_string(),
                ..RelayProfile::default()
            }],
            active_relay_id: "manual-a".to_string(),
            ..BackendSettings::default()
        };

        let updated = upsert_managed_provider(settings, &snapshot("available")).unwrap();

        assert_eq!(updated.relay_profiles.len(), 2);
        assert!(
            updated
                .relay_profiles
                .iter()
                .any(|profile| profile.id == "manual-a")
        );
        assert_eq!(updated.active_relay_id, MANAGED_PROVIDER_ID);
    }

    #[test]
    fn repeated_upsert_reuses_managed_provider_id() {
        let settings =
            upsert_managed_provider(BackendSettings::default(), &snapshot("available")).unwrap();
        let updated = upsert_managed_provider(settings, &snapshot("available")).unwrap();

        assert_eq!(
            updated
                .relay_profiles
                .iter()
                .filter(|profile| profile.id == MANAGED_PROVIDER_ID)
                .count(),
            1
        );
    }

    #[test]
    fn gateway_unhealthy_without_key_is_not_applyable() {
        let result =
            upsert_managed_provider(BackendSettings::default(), &snapshot("gateway_unhealthy"));

        assert!(result.is_err());
    }

    #[test]
    fn managed_profile_does_not_serialize_api_key_field() {
        let profile = relay_profile_from_bootstrap(&snapshot("available")).unwrap();
        let value = serde_json::to_value(&profile).unwrap();

        assert!(value.get("apiKey").is_none());
        assert!(
            value["authContents"]
                .as_str()
                .unwrap()
                .contains("OPENAI_API_KEY")
        );
    }

    #[test]
    fn managed_profile_routes_codex_through_local_proxy() {
        let profile = relay_profile_from_bootstrap(&snapshot("available")).unwrap();

        assert_eq!(profile.base_url, "https://api.example/v1");
        assert_eq!(profile.upstream_base_url, "https://api.example/v1");
        assert!(
            profile
                .config_contents
                .contains(r#"base_url = "http://127.0.0.1:57321/v1""#)
        );
    }

    fn snapshot(status: &str) -> BootstrapSnapshot {
        BootstrapSnapshot {
            service: ServiceState {
                status: status.to_string(),
                message: "ok".to_string(),
                message_key: Some(format!("service.{status}")),
                action_hint: "none".to_string(),
                retryable: true,
                support_url: None,
                error_code: None,
            },
            provider: ManagedProvider {
                provider_id: MANAGED_PROVIDER_ID.to_string(),
                display_name: MANAGED_PROVIDER_NAME.to_string(),
                gateway_base_url: "https://api.example/v1".to_string(),
                auth_mode: "user_side_api_key".to_string(),
                api_key: (status != "gateway_unhealthy").then(|| "test-api-key".to_string()),
                key_summary: ProviderKeySummary {
                    key_id: "key".to_string(),
                    masked_key: "test-...key".to_string(),
                    created_at: "2026-06-16T00:00:00Z".to_string(),
                    last_used_at: None,
                },
                default_model: "gpt-5-codex".to_string(),
            },
            plan: PlanSummary {
                plan_id: "plan".to_string(),
                name: "Plan".to_string(),
                status: "active".to_string(),
                expires_at: None,
                renew_url: None,
                commerce_action: Some(RenewAction {
                    action_type: "manage_plan".to_string(),
                    message_key: Some("usage.manage_plan".to_string()),
                    action_copy_key: Some("action.manage_plan".to_string()),
                    label: "Manage".to_string(),
                    url: None,
                }),
            },
            models: Vec::new(),
            usage: UsageSummary {
                balance_display: "ok".to_string(),
                low_balance: false,
                period_usage_display: "ok".to_string(),
                rate_limit_state: "normal".to_string(),
                renew_action: RenewAction {
                    action_type: "manage_plan".to_string(),
                    message_key: Some("usage.manage_plan".to_string()),
                    action_copy_key: Some("action.manage_plan".to_string()),
                    label: "Manage".to_string(),
                    url: None,
                },
            },
            feature_flags: FeatureFlagSnapshot {
                advanced_provider_config: false,
                install_assistant: true,
                new_user_tutorial: true,
                model_selector: true,
                diagnostic_export: true,
                announcements: true,
                force_update_prompt: false,
                strict_device_enforcement: false,
            },
            announcements: Vec::<Announcement>::new(),
            version_policy: VersionPolicy {
                config_version: "cfg".to_string(),
                snapshot_version: "snap".to_string(),
                refresh_ttl_seconds: 300,
                force_refresh: false,
                minimum_client_version: "0.1.0".to_string(),
            },
            device: DeviceRegistration {
                device_id: "dev".to_string(),
                status: "active".to_string(),
                message: "ok".to_string(),
                snapshot_version: "snap".to_string(),
            },
        }
    }
}
