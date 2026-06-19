use std::time::Duration;

use anyhow::Context;
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};

use super::redaction::redact_string;

const CLIENT_API_TIMEOUT: Duration = Duration::from_secs(20);

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Envelope<T> {
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub code: Option<i64>,
    pub message: String,
    #[serde(default)]
    pub error_code: Option<String>,
    pub data: Option<T>,
}

impl<T> Envelope<T> {
    pub fn is_success(&self) -> bool {
        matches!(self.status.as_str(), "success" | "ok") || self.code == Some(0)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub struct BootstrapSnapshot {
    pub service: ServiceState,
    pub provider: ManagedProvider,
    pub plan: PlanSummary,
    #[serde(default)]
    pub models: Vec<ModelSummary>,
    pub usage: UsageSummary,
    pub feature_flags: FeatureFlagSnapshot,
    #[serde(default)]
    pub announcements: Vec<Announcement>,
    pub version_policy: VersionPolicy,
    pub device: DeviceRegistration,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServiceState {
    pub status: String,
    pub message: String,
    #[serde(default)]
    pub message_key: Option<String>,
    pub action_hint: String,
    pub retryable: bool,
    #[serde(default)]
    pub support_url: Option<String>,
    #[serde(default)]
    pub error_code: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ManagedProvider {
    pub provider_id: String,
    pub display_name: String,
    pub gateway_base_url: String,
    pub auth_mode: String,
    #[serde(default)]
    pub api_key: Option<String>,
    pub key_summary: ProviderKeySummary,
    pub default_model: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProviderKeySummary {
    pub key_id: String,
    pub masked_key: String,
    pub created_at: String,
    #[serde(default)]
    pub last_used_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PlanSummary {
    pub plan_id: String,
    pub name: String,
    pub status: String,
    #[serde(default)]
    pub expires_at: Option<String>,
    #[serde(default)]
    pub renew_url: Option<String>,
    #[serde(default)]
    pub commerce_action: Option<RenewAction>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelSummary {
    pub model_id: String,
    #[serde(default)]
    pub route_model: Option<String>,
    pub label: String,
    pub is_default: bool,
    pub is_available: bool,
    #[serde(default)]
    pub disabled_reason: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UsageSummary {
    pub balance_display: String,
    pub low_balance: bool,
    pub period_usage_display: String,
    pub rate_limit_state: String,
    pub renew_action: RenewAction,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RenewAction {
    #[serde(default)]
    pub action_type: String,
    #[serde(default)]
    pub message_key: Option<String>,
    #[serde(default)]
    pub action_copy_key: Option<String>,
    pub label: String,
    #[serde(default)]
    pub url: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UsageSnapshot {
    pub service_status: String,
    #[serde(default)]
    pub balance_summary: Value,
    #[serde(default)]
    pub period_usage: Value,
    pub rate_limit_state: String,
    #[serde(default)]
    pub renew_action: Value,
    pub last_updated_at: String,
    pub snapshot_version: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeatureFlagSnapshot {
    pub advanced_provider_config: bool,
    pub install_assistant: bool,
    pub new_user_tutorial: bool,
    pub model_selector: bool,
    pub diagnostic_export: bool,
    #[serde(default)]
    pub announcements: bool,
    #[serde(default)]
    pub force_update_prompt: bool,
    #[serde(default)]
    pub strict_device_enforcement: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Announcement {
    pub id: String,
    pub severity: String,
    pub message: String,
    #[serde(default)]
    pub url: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct VersionPolicy {
    pub config_version: String,
    pub snapshot_version: String,
    pub refresh_ttl_seconds: u64,
    pub force_refresh: bool,
    pub minimum_client_version: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DeviceRegistration {
    pub device_id: String,
    pub status: String,
    pub message: String,
    pub snapshot_version: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RedeemResult {
    pub redeem_status: String,
    pub entitlement_delta_summary: String,
    pub service_status_after: String,
    pub snapshot_version: String,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoginData {
    #[serde(default)]
    pub status: Option<String>,
    #[serde(default, alias = "accessToken", alias = "token", alias = "jwt")]
    pub access_token: String,
    #[serde(default, alias = "userId")]
    pub user_id: String,
    #[serde(default, alias = "userLabel")]
    pub user_label: Option<String>,
    #[serde(default, alias = "expiresAt")]
    pub expires_at: Option<String>,
    #[serde(default)]
    pub user: Option<LoginUserData>,
    #[serde(default)]
    pub requires_2fa: bool,
    #[serde(default)]
    pub temp_token: Option<String>,
    #[serde(default)]
    pub user_email_masked: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DesktopLoginStartData {
    pub session_token: String,
    pub poll_token: String,
    pub authorize_url: String,
    #[serde(default)]
    pub verification_code: Option<String>,
    pub expires_at: String,
    #[serde(default)]
    pub poll_interval_seconds: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LoginUserData {
    #[serde(default)]
    pub id: i64,
    #[serde(default)]
    pub email: String,
    #[serde(default)]
    pub username: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DeviceRegisterRequest {
    pub device_id: String,
    pub platform: String,
    pub app_version: String,
    #[serde(default)]
    pub codex_version: Option<String>,
    pub last_seen_at: String,
}

pub fn normalize_base_url(base_url: &str) -> String {
    base_url.trim().trim_end_matches('/').to_string()
}

pub async fn login(
    base_url: &str,
    email: &str,
    password: &str,
) -> anyhow::Result<Envelope<LoginData>> {
    let base_url = normalize_base_url(base_url);
    if is_mock_base_url(&base_url) {
        return Ok(Envelope {
            status: "success".to_string(),
            code: Some(0),
            message: "Mock cloud session created.".to_string(),
            error_code: None,
            data: Some(LoginData {
                status: None,
                access_token: "mock-local-session-token".to_string(),
                user_id: email.trim().to_string(),
                user_label: Some(email.trim().to_string()),
                expires_at: None,
                user: None,
                requires_2fa: false,
                temp_token: None,
                user_email_masked: None,
            }),
        });
    }
    let client = crate::http_client::proxied_client(&format!(
        "CodexPlusPlus/{} CloudRuntime",
        crate::version::VERSION
    ))?;
    let endpoint = format!("{base_url}/api/v1/auth/login");
    let response = client
        .post(&endpoint)
        .timeout(CLIENT_API_TIMEOUT)
        .header(reqwest::header::ACCEPT, "application/json")
        .json(&json!({
            "email": email,
            "password": password
        }))
        .send()
        .await
        .with_context(|| format!("cloud login request failed: {}", redact_string(&endpoint)))?;
    parse_envelope_response(response).await
}

pub async fn login_2fa(
    base_url: &str,
    temp_token: &str,
    totp_code: &str,
) -> anyhow::Result<Envelope<LoginData>> {
    let base_url = normalize_base_url(base_url);
    if is_mock_base_url(&base_url) {
        return Ok(Envelope {
            status: "success".to_string(),
            code: Some(0),
            message: "Mock 2FA session completed.".to_string(),
            error_code: None,
            data: Some(LoginData {
                status: None,
                access_token: "mock-local-session-token".to_string(),
                user_id: "mock-2fa-user".to_string(),
                user_label: Some("mock-2fa-user".to_string()),
                expires_at: None,
                user: None,
                requires_2fa: false,
                temp_token: None,
                user_email_masked: None,
            }),
        });
    }
    let client = crate::http_client::proxied_client(&format!(
        "CodexPlusPlus/{} CloudRuntime",
        crate::version::VERSION
    ))?;
    let endpoint = format!("{base_url}/api/v1/auth/login/2fa");
    let response = client
        .post(&endpoint)
        .timeout(CLIENT_API_TIMEOUT)
        .header(reqwest::header::ACCEPT, "application/json")
        .json(&json!({
            "temp_token": temp_token,
            "totp_code": totp_code
        }))
        .send()
        .await
        .with_context(|| {
            format!(
                "cloud 2FA login request failed: {}",
                redact_string(&endpoint)
            )
        })?;
    parse_envelope_response(response).await
}

pub async fn start_browser_handoff(
    base_url: &str,
    device_id: &str,
    device_name: &str,
) -> anyhow::Result<Envelope<DesktopLoginStartData>> {
    let base_url = normalize_base_url(base_url);
    if is_mock_base_url(&base_url) {
        return Ok(Envelope {
            status: "success".to_string(),
            code: Some(0),
            message: "Mock browser handoff created.".to_string(),
            error_code: None,
            data: Some(DesktopLoginStartData {
                session_token: "mock-session-token".to_string(),
                poll_token: "mock-poll-token".to_string(),
                authorize_url: "https://codex-plus.example/auth/desktop/authorize?session_token=mock-session-token&verification_code=123456".to_string(),
                verification_code: Some("123456".to_string()),
                expires_at: "2026-06-16T00:15:00Z".to_string(),
                poll_interval_seconds: 2,
            }),
        });
    }
    let client = cloud_client()?;
    let endpoint = format!("{base_url}/api/v1/auth/desktop/start");
    let response = client
        .post(&endpoint)
        .timeout(CLIENT_API_TIMEOUT)
        .header(reqwest::header::ACCEPT, "application/json")
        .json(&json!({
            "device_id": device_id,
            "device_name": device_name
        }))
        .send()
        .await
        .with_context(|| {
            format!(
                "cloud browser handoff start failed: {}",
                redact_string(&endpoint)
            )
        })?;
    parse_envelope_response(response).await
}

pub async fn poll_browser_handoff(
    base_url: &str,
    session_token: &str,
    poll_token: &str,
) -> anyhow::Result<Envelope<LoginData>> {
    let base_url = normalize_base_url(base_url);
    if is_mock_base_url(&base_url) {
        return Ok(Envelope {
            status: "success".to_string(),
            code: Some(0),
            message: "Mock browser handoff completed.".to_string(),
            error_code: None,
            data: Some(LoginData {
                status: Some("completed".to_string()),
                access_token: "mock-local-session-token".to_string(),
                user_id: "mock-browser-user".to_string(),
                user_label: Some("mock-browser-user@example.com".to_string()),
                expires_at: None,
                user: Some(LoginUserData {
                    id: 1,
                    email: "mock-browser-user@example.com".to_string(),
                    username: "mock-browser-user".to_string(),
                }),
                requires_2fa: false,
                temp_token: None,
                user_email_masked: None,
            }),
        });
    }
    let client = cloud_client()?;
    let endpoint = format!("{base_url}/api/v1/auth/desktop/poll");
    let response = client
        .post(&endpoint)
        .timeout(CLIENT_API_TIMEOUT)
        .header(reqwest::header::ACCEPT, "application/json")
        .json(&json!({
            "session_token": session_token,
            "poll_token": poll_token
        }))
        .send()
        .await
        .with_context(|| {
            format!(
                "cloud browser handoff poll failed: {}",
                redact_string(&endpoint)
            )
        })?;
    parse_envelope_response(response).await
}

pub async fn get_bootstrap(
    base_url: &str,
    access_token: &str,
    device_id: &str,
) -> anyhow::Result<Envelope<BootstrapSnapshot>> {
    let base_url = normalize_base_url(base_url);
    if is_mock_base_url(&base_url) {
        return Ok(mock_bootstrap_envelope(&base_url, device_id));
    }
    let client = cloud_client()?;
    let endpoint = format!("{base_url}/api/v1/client/bootstrap");
    let response = client
        .get(&endpoint)
        .timeout(CLIENT_API_TIMEOUT)
        .bearer_auth(access_token)
        .header("X-CodexPlus-Device-Id", device_id)
        .header("X-CodexPlus-Client-Version", crate::version::VERSION)
        .header(reqwest::header::ACCEPT, "application/json")
        .send()
        .await
        .with_context(|| {
            format!(
                "cloud bootstrap request failed: {}",
                redact_string(&endpoint)
            )
        })?;
    parse_envelope_response(response).await
}

pub async fn get_usage(
    base_url: &str,
    access_token: &str,
    device_id: &str,
) -> anyhow::Result<Envelope<UsageSnapshot>> {
    let base_url = normalize_base_url(base_url);
    if is_mock_base_url(&base_url) {
        return Ok(mock_usage_envelope());
    }
    let client = cloud_client()?;
    let endpoint = format!("{base_url}/api/v1/client/usage");
    let response = client
        .get(&endpoint)
        .timeout(CLIENT_API_TIMEOUT)
        .bearer_auth(access_token)
        .header("X-CodexPlus-Device-Id", device_id)
        .header(reqwest::header::ACCEPT, "application/json")
        .send()
        .await
        .with_context(|| format!("cloud usage request failed: {}", redact_string(&endpoint)))?;
    parse_envelope_response(response).await
}

pub async fn register_device(
    base_url: &str,
    access_token: &str,
    request: &DeviceRegisterRequest,
) -> anyhow::Result<Envelope<DeviceRegistration>> {
    let base_url = normalize_base_url(base_url);
    if is_mock_base_url(&base_url) {
        return Ok(Envelope {
            status: "success".to_string(),
            code: Some(0),
            message: "Mock device registered.".to_string(),
            error_code: None,
            data: Some(DeviceRegistration {
                device_id: request.device_id.clone(),
                status: "active".to_string(),
                message: "Device active.".to_string(),
                snapshot_version: "mock-device".to_string(),
            }),
        });
    }
    let client = cloud_client()?;
    let endpoint = format!("{base_url}/api/v1/client/devices");
    let response = client
        .post(&endpoint)
        .timeout(CLIENT_API_TIMEOUT)
        .bearer_auth(access_token)
        .header("X-CodexPlus-Device-Id", &request.device_id)
        .header(reqwest::header::CONTENT_TYPE, "application/json")
        .json(request)
        .send()
        .await
        .with_context(|| {
            format!(
                "cloud device registration failed: {}",
                redact_string(&endpoint)
            )
        })?;
    parse_envelope_response(response).await
}

pub async fn redeem(
    base_url: &str,
    access_token: &str,
    device_id: &str,
    code: &str,
) -> anyhow::Result<Envelope<RedeemResult>> {
    let base_url = normalize_base_url(base_url);
    if is_mock_base_url(&base_url) {
        return Ok(Envelope {
            status: "success".to_string(),
            code: Some(0),
            message: "Mock code redeemed.".to_string(),
            error_code: None,
            data: Some(RedeemResult {
                redeem_status: "applied".to_string(),
                entitlement_delta_summary: "Mock entitlement refreshed.".to_string(),
                service_status_after: "available".to_string(),
                snapshot_version: "mock-redeem".to_string(),
                message: "Mock code redeemed.".to_string(),
            }),
        });
    }
    let client = cloud_client()?;
    let endpoint = format!("{base_url}/api/v1/client/redeem");
    let response = client
        .post(&endpoint)
        .timeout(CLIENT_API_TIMEOUT)
        .bearer_auth(access_token)
        .header("X-CodexPlus-Device-Id", device_id)
        .header(reqwest::header::CONTENT_TYPE, "application/json")
        .json(&json!({
            "code": code,
            "device_id": device_id
        }))
        .send()
        .await
        .with_context(|| format!("cloud redeem request failed: {}", redact_string(&endpoint)))?;
    parse_envelope_response(response).await
}

fn cloud_client() -> anyhow::Result<reqwest::Client> {
    crate::http_client::proxied_client(&format!(
        "CodexPlusPlus/{} CloudRuntime",
        crate::version::VERSION
    ))
}

async fn parse_envelope_response<T>(response: reqwest::Response) -> anyhow::Result<Envelope<T>>
where
    T: for<'de> Deserialize<'de>,
{
    let status = response.status();
    let text = response.text().await.unwrap_or_default();
    match serde_json::from_str::<Envelope<T>>(&text) {
        Ok(envelope) => Ok(envelope),
        Err(error) => {
            anyhow::bail!(
                "cloud response parse failed (HTTP {}): {}; body={}",
                status.as_u16(),
                error,
                redact_string(&text.chars().take(512).collect::<String>())
            )
        }
    }
}

fn is_mock_base_url(base_url: &str) -> bool {
    base_url.eq_ignore_ascii_case("mock://codex-plus-cloud")
        || base_url.eq_ignore_ascii_case("mock://available")
        || base_url.starts_with("mock://")
}

fn mock_bootstrap_envelope(base_url: &str, device_id: &str) -> Envelope<BootstrapSnapshot> {
    let status = base_url
        .strip_prefix("mock://")
        .filter(|value| !value.is_empty() && value != &"codex-plus-cloud")
        .unwrap_or("available");
    let service_status = match status {
        "not_purchased" => "not_purchased",
        "expired" => "expired",
        "low_balance" => "low_balance",
        "disabled" => "disabled",
        "device_revoked" => "device_revoked",
        "model_unavailable" => "model_unavailable",
        "rate_limited" => "rate_limited",
        "gateway_unhealthy" => "gateway_unhealthy",
        "local_codex_missing" => "local_codex_missing",
        "local_config_failed" => "local_config_failed",
        _ => "available",
    };
    let api_key = matches!(service_status, "available" | "low_balance" | "rate_limited")
        .then(|| "mock-local-api-key".to_string());
    Envelope {
        status: "success".to_string(),
        code: Some(0),
        message: "Mock bootstrap snapshot loaded.".to_string(),
        error_code: None,
        data: Some(BootstrapSnapshot {
            service: ServiceState {
                status: service_status.to_string(),
                message: format!("Mock service status: {service_status}."),
                message_key: Some(format!("service.{service_status}")),
                action_hint: "none".to_string(),
                retryable: true,
                support_url: None,
                error_code: None,
            },
            provider: ManagedProvider {
                provider_id: "codex-plus-cloud".to_string(),
                display_name: "Codex++ Cloud".to_string(),
                gateway_base_url: "https://api.codex-plus.example/v1".to_string(),
                auth_mode: "user_side_api_key".to_string(),
                api_key,
                key_summary: ProviderKeySummary {
                    key_id: "mock-key".to_string(),
                    masked_key: "sk-user-...mock".to_string(),
                    created_at: "2026-06-16T00:00:00Z".to_string(),
                    last_used_at: None,
                },
                default_model: "gpt-5-codex".to_string(),
            },
            plan: PlanSummary {
                plan_id: "mock".to_string(),
                name: "Mock Plan".to_string(),
                status: "active".to_string(),
                expires_at: None,
                renew_url: None,
                commerce_action: Some(RenewAction {
                    action_type: "manage_plan".to_string(),
                    message_key: Some("usage.manage_plan".to_string()),
                    action_copy_key: Some("action.manage_plan".to_string()),
                    label: "Manage plan".to_string(),
                    url: None,
                }),
            },
            models: vec![ModelSummary {
                model_id: "gpt-5-codex".to_string(),
                route_model: Some("openai/gpt-5-codex".to_string()),
                label: "GPT-5 Codex".to_string(),
                is_default: true,
                is_available: true,
                disabled_reason: None,
            }],
            usage: UsageSummary {
                balance_display: "Mock credits available".to_string(),
                low_balance: service_status == "low_balance",
                period_usage_display: "Mock usage".to_string(),
                rate_limit_state: "normal".to_string(),
                renew_action: RenewAction {
                    action_type: "manage_plan".to_string(),
                    message_key: Some("usage.manage_plan".to_string()),
                    action_copy_key: Some("action.manage_plan".to_string()),
                    label: "Manage plan".to_string(),
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
            announcements: Vec::new(),
            version_policy: VersionPolicy {
                config_version: "mock-config".to_string(),
                snapshot_version: format!("mock-{service_status}"),
                refresh_ttl_seconds: 300,
                force_refresh: false,
                minimum_client_version: "0.1.0".to_string(),
            },
            device: DeviceRegistration {
                device_id: device_id.to_string(),
                status: if service_status == "device_revoked" {
                    "revoked"
                } else {
                    "active"
                }
                .to_string(),
                message: "Mock device status.".to_string(),
                snapshot_version: format!("mock-{service_status}"),
            },
        }),
    }
}

fn mock_usage_envelope() -> Envelope<UsageSnapshot> {
    Envelope {
        status: "success".to_string(),
        code: Some(0),
        message: "Mock usage loaded.".to_string(),
        error_code: None,
        data: Some(UsageSnapshot {
            service_status: "available".to_string(),
            balance_summary: json!({"balance_display": "Mock credits available", "low_balance": false}),
            period_usage: json!({"period_id": "mock-day", "usage_display": "Mock usage", "reset_at": null}),
            rate_limit_state: "normal".to_string(),
            renew_action: json!({
                "action_type": "manage_plan",
                "message_key": "usage.manage_plan",
                "action_copy_key": "action.manage_plan",
                "label": "Manage plan",
                "url": null
            }),
            last_updated_at: "2026-06-16T00:00:00Z".to_string(),
            snapshot_version: "mock-usage".to_string(),
        }),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn bootstrap_fixture_shape_deserializes() {
        let envelope: Envelope<BootstrapSnapshot> = serde_json::from_str(
            r#"{
              "status": "success",
              "message": "ok",
              "error_code": null,
              "data": {
                "service": {"status":"available","message":"ok","action_hint":"none","retryable":true,"support_url":null,"error_code":null},
                "provider": {"provider_id":"codex-plus-cloud","display_name":"Codex++ Cloud","gateway_base_url":"https://api.example/v1","auth_mode":"user_side_api_key","api_key":"test-api-key","key_summary":{"key_id":"k","masked_key":"test-...key","created_at":"2026-06-16T00:00:00Z","last_used_at":null},"default_model":"gpt-5-codex"},
                "plan": {"plan_id":"starter","name":"Starter","status":"active","expires_at":null,"renew_url":null},
                "models": [],
                "usage": {"balance_display":"ok","low_balance":false,"period_usage_display":"ok","rate_limit_state":"normal","renew_action":{"label":"Manage","url":null}},
                "feature_flags": {"advanced_provider_config":false,"install_assistant":true,"new_user_tutorial":true,"model_selector":true,"diagnostic_export":true},
                "announcements": [],
                "version_policy": {"config_version":"cfg","snapshot_version":"snap","refresh_ttl_seconds":300,"force_refresh":false,"minimum_client_version":"0.1.0"},
                "device": {"device_id":"dev","status":"active","message":"ok","snapshot_version":"snap"}
              }
            }"#,
        )
        .unwrap();

        assert_eq!(
            envelope.data.unwrap().provider.provider_id,
            "codex-plus-cloud"
        );
    }

    #[test]
    fn bootstrap_client_action_fields_deserialize() {
        let envelope: Envelope<BootstrapSnapshot> = serde_json::from_str(
            r#"{
              "code": 0,
              "status": "success",
              "message": "ok",
              "data": {
                "service": {"status":"available","message":"ok","message_key":"service.available","action_hint":"none","retryable":true,"support_url":null,"error_code":null},
                "provider": {"provider_id":"codex-plus-cloud","display_name":"Codex++ Cloud","gateway_base_url":"https://api.example/v1","auth_mode":"user_side_api_key","api_key":"test-api-key","key_summary":{"key_id":"k","masked_key":"test-...key","created_at":"2026-06-16T00:00:00Z","last_used_at":null},"default_model":"gpt-5-codex"},
                "plan": {"plan_id":"starter","name":"Starter","status":"active","expires_at":null,"renew_url":null,"commerce_action":{"action_type":"manage_plan","message_key":"plan.starter.active","action_copy_key":"action.manage_plan","label":"Manage plan","url":"https://billing.example.test"}},
                "models": [],
                "usage": {"balance_display":"ok","low_balance":false,"period_usage_display":"ok","rate_limit_state":"normal","renew_action":{"action_type":"manage_plan","message_key":"usage.manage_plan","action_copy_key":"action.manage_plan","label":"Manage","url":null}},
                "feature_flags": {"advanced_provider_config":false,"install_assistant":true,"new_user_tutorial":true,"model_selector":true,"diagnostic_export":true,"announcements":true,"force_update_prompt":false,"strict_device_enforcement":true},
                "announcements": [],
                "version_policy": {"config_version":"cfg","snapshot_version":"snap","refresh_ttl_seconds":300,"force_refresh":false,"minimum_client_version":"0.1.0"},
                "device": {"device_id":"dev","status":"active","message":"ok","snapshot_version":"snap"}
              }
            }"#,
        )
        .unwrap();

        let snapshot = envelope.data.unwrap();
        assert_eq!(
            snapshot.service.message_key.as_deref(),
            Some("service.available")
        );
        assert_eq!(
            snapshot
                .plan
                .commerce_action
                .unwrap()
                .action_copy_key
                .as_deref(),
            Some("action.manage_plan")
        );
        assert!(snapshot.feature_flags.strict_device_enforcement);
    }

    #[test]
    fn existing_backend_code_envelope_counts_as_success() {
        let envelope: Envelope<Value> = serde_json::from_str(
            r#"{
              "code": 0,
              "message": "success",
              "data": {"ok": true}
            }"#,
        )
        .unwrap();

        assert!(envelope.is_success());
    }

    #[test]
    fn auth_login_envelope_shape_deserializes() {
        let envelope: Envelope<LoginData> = serde_json::from_str(
            r#"{
              "code": 0,
              "message": "success",
              "data": {
                "access_token": "jwt-token",
                "refresh_token": "refresh-token",
                "expires_in": 3600,
                "token_type": "Bearer",
                "user": {
                  "id": 42,
                  "email": "buyer@example.com",
                  "username": "buyer",
                  "role": "user",
                  "balance": 1,
                  "concurrency": 1,
                  "status": "active",
                  "allowed_groups": [],
                  "created_at": "2026-06-16T00:00:00Z",
                  "updated_at": "2026-06-16T00:00:00Z"
                }
              }
            }"#,
        )
        .unwrap();

        assert!(envelope.is_success());
        let data = envelope.data.unwrap();
        assert_eq!(data.access_token, "jwt-token");
        assert_eq!(data.user.unwrap().id, 42);
    }

    #[test]
    fn auth_login_2fa_envelope_shape_deserializes() {
        let envelope: Envelope<LoginData> = serde_json::from_str(
            r#"{
              "code": 0,
              "message": "success",
              "data": {
                "requires_2fa": true,
                "temp_token": "temp",
                "user_email_masked": "b***@example.com"
              }
            }"#,
        )
        .unwrap();

        let data = envelope.data.unwrap();
        assert!(data.requires_2fa);
        assert_eq!(data.temp_token.as_deref(), Some("temp"));
    }

    #[test]
    fn desktop_handoff_start_envelope_shape_deserializes() {
        let envelope: Envelope<DesktopLoginStartData> = serde_json::from_str(
            r#"{
              "code": 0,
              "message": "success",
              "data": {
                "session_token": "session",
                "poll_token": "poll",
                "authorize_url": "https://app.example.test/auth/desktop/authorize?session_token=session&verification_code=123456",
                "verification_code": "123456",
                "expires_at": "2026-06-16T00:15:00Z",
                "poll_interval_seconds": 2
              }
            }"#,
        )
        .unwrap();

        assert!(envelope.is_success());
        let data = envelope.data.unwrap();
        assert_eq!(data.session_token, "session");
        assert_eq!(data.poll_token, "poll");
        assert_eq!(data.verification_code.as_deref(), Some("123456"));
    }

    #[test]
    fn desktop_handoff_poll_envelope_shape_deserializes() {
        let pending: Envelope<LoginData> = serde_json::from_str(
            r#"{
              "code": 0,
              "message": "success",
              "data": { "status": "pending" }
            }"#,
        )
        .unwrap();
        assert_eq!(pending.data.unwrap().status.as_deref(), Some("pending"));

        let completed: Envelope<LoginData> = serde_json::from_str(
            r#"{
              "code": 0,
              "message": "success",
              "data": {
                "status": "completed",
                "access_token": "jwt-token",
                "refresh_token": "refresh-token",
                "expires_in": 3600,
                "token_type": "Bearer",
                "user": {
                  "id": 42,
                  "email": "buyer@example.com",
                  "username": "buyer"
                }
              }
            }"#,
        )
        .unwrap();

        let data = completed.data.unwrap();
        assert_eq!(data.status.as_deref(), Some("completed"));
        assert_eq!(data.access_token, "jwt-token");
        assert_eq!(data.user.unwrap().email, "buyer@example.com");
    }

    #[test]
    fn normalizes_base_url_trailing_slash() {
        assert_eq!(
            normalize_base_url(" https://api.example.test/// "),
            "https://api.example.test"
        );
    }
}
